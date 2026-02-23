package memspace_region

import (
	"NucleusMem/pkg/client"
	"NucleusMem/pkg/configs"
	"NucleusMem/pkg/storage"
	tinykv_client "NucleusMem/pkg/storage/tinykv-client"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"github.com/pingcap-incubator/tinykv/log"
	"net/http"
	"sync"
	"time"
)

const CommSeqKey = "comm_seq"
const CommRegistryKey = "comm_registry"

type CommMessage struct {
	Key       string `json:"key"` // points to Memory or Summary Region
	FromAgent uint64 `json:"from_agent"`
	ToAgent   uint64 `json:"to_agent"`
	RefType   string `json:"ref_type"` // "memory" or "summary"
	Timestamp int64  `json:"timestamp"`
	Content   string `json:"content"`
}

type AgentRegistryEntry struct {
	AgentID   uint64 `json:"agent_id"`
	Addr      string `json:"addr"` // e.g., "localhost:9001"
	Role      string `json:"role"`
	Timestamp int64  `json:"timestamp"`
}

type CommRegion struct {
	memSpaceID uint64
	seq        uint64
	mu         sync.RWMutex
	messages   []CommMessage
	registry   map[uint64]AgentRegistryEntry
	kvClient   *tinykv_client.MemClient
}

func NewCommRegion(kvClient *tinykv_client.MemClient, memSpaceID uint64) *CommRegion {
	cr := &CommRegion{
		messages:   make([]CommMessage, 0),
		registry:   make(map[uint64]AgentRegistryEntry),
		kvClient:   kvClient,
		memSpaceID: memSpaceID,
	}
	// Load sequence number
	if seq, err := cr.loadSeq(); err == nil {
		cr.seq = seq
	} else {
		cr.seq = 1
	}
	return cr
}

// RegisterAgent registers an agent in the address table
func (cr *CommRegion) RegisterAgent(agentID uint64, addr, role string) error {
	cr.mu.Lock()
	defer cr.mu.Unlock()

	// Update in-memory registry
	cr.registry[agentID] = AgentRegistryEntry{
		AgentID:   agentID,
		Addr:      addr,
		Role:      role,
		Timestamp: time.Now().Unix(),
	}

	// Serialize entire registry
	data, err := json.Marshal(cr.registry)
	if err != nil {
		return fmt.Errorf("failed to marshal registry: %w", err)
	}

	// Encode key
	rawKey := configs.EncodeKey(configs.ZoneComm, cr.memSpaceID, []byte(CommRegistryKey))

	// Persist registry
	err = cr.kvClient.Update(func(txn storage.Transaction) error {
		return txn.Put(rawKey, data)
	})
	if err != nil {
		return fmt.Errorf("failed to persist registry: %w", err)
	}

	// 添加通讯记录（解锁后调用，避免死锁）
	cr.mu.Unlock()
	err = cr.AddAgentCommRecord(agentID, addr, role)
	cr.mu.Lock()

	if err != nil {
		log.Warnf("Failed to add comm record for agent %d: %v", agentID, err)
		// Don't fail registration — comm record is optional
	}

	return nil
}
func (cr *CommRegion) loadSeq() (uint64, error) {
	rawKey := configs.EncodeKey(configs.ZoneComm, cr.memSpaceID, []byte(CommSeqKey))
	var seq uint64 = 1
	err := cr.kvClient.Update(func(txn storage.Transaction) error {
		data, err := txn.Get(rawKey)
		if len(data) == 0 {
			return nil
		}
		if len(data) < 8 {
			return nil
		}
		if err != nil {
			return nil
		}
		seq = binary.LittleEndian.Uint64(data)
		return nil
	})
	return seq, err
}

func (cr *CommRegion) saveSeq(seq uint64) error {
	rawKey := configs.EncodeKey(configs.ZoneComm, cr.memSpaceID, []byte(CommSeqKey))
	data := make([]byte, 8)
	binary.LittleEndian.PutUint64(data, seq)
	return cr.kvClient.Update(func(txn storage.Transaction) error {
		return txn.Put(rawKey, data)
	})
}

// SendMessage sends a message from fromAgent to toAgent
func (cr *CommRegion) SendMessage(fromAgent, toAgent uint64, key, content string) (string, error) {
	cr.mu.Lock()
	cr.seq++
	commKey := fmt.Sprintf("comm/%s/%d/%d", key, fromAgent, cr.seq)
	err := cr.saveSeq(cr.seq)
	cr.mu.Unlock()
	if err != nil {
		return "", fmt.Errorf("failed to save comm sequence: %w", err)
	}
	rawKey := configs.EncodeKey(configs.ZoneComm, cr.memSpaceID, []byte(commKey))
	msg := CommMessage{
		Key:       string(rawKey), // using raw key directly
		FromAgent: fromAgent,
		ToAgent:   toAgent,
		//RefType:   refType,
		Content:   content,
		Timestamp: time.Now().Unix(),
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return "", err
	}
	err = cr.kvClient.Update(func(txn storage.Transaction) error {
		return txn.Put(rawKey, data)
	})
	if err != nil {
		return "", fmt.Errorf("failed to persist comm message: %w", err)
	}
	// 4. Notify target agent via HTTP
	entry, ok := cr.getAgent(toAgent)
	if !ok {
		log.Warnf("Target agent %d not registered, skip notify", toAgent)
		return "", nil
	}
	// Create client and call /notify
	httpClient := &http.Client{Timeout: 60 * time.Second}
	agentClient := &client.AgentClient{
		BaseURL:    "http://" + entry.Addr,
		HttpClient: httpClient, // 确保 Client 结构体支持注入 httpClient
	} // Pass the original key (not commKey!) so agent can fetch content
	result, err := agentClient.Notify(string(rawKey), content)
	if err != nil {
		log.Warnf("Failed to notify agent %d: %v", toAgent, err)
		// Don't fail the whole operation — message is persisted
	}
	return result, nil
}

// getAgent is a thread-safe getter
func (cr *CommRegion) getAgent(agentID uint64) (AgentRegistryEntry, bool) {
	cr.mu.RLock()
	defer cr.mu.RUnlock()
	entry, ok := cr.registry[agentID]
	return entry, ok
}

// ListAgents returns a copy of the current agent registry
func (cr *CommRegion) ListAgents() []AgentRegistryEntry {
	cr.mu.RLock()
	defer cr.mu.RUnlock()

	agents := make([]AgentRegistryEntry, 0, len(cr.registry))
	for _, entry := range cr.registry {
		agents = append(agents, entry)
	}
	return agents
}

// recoverRegistry loads the entire registry from a single key
func (cr *CommRegion) recoverRegistry() {
	rawKey := configs.EncodeKey(configs.ZoneComm, cr.memSpaceID, []byte(CommRegistryKey))

	var registry map[uint64]AgentRegistryEntry
	err := cr.kvClient.Update(func(txn storage.Transaction) error {
		data, err := txn.Get(rawKey)
		if err != nil {
			return nil // key not found is OK
		}
		return json.Unmarshal(data, &registry)
	})
	if err != nil {
		log.Warnf("Failed to recover comm registry: %v", err)
		return
	}

	if registry != nil {
		cr.mu.Lock()
		cr.registry = registry
		cr.mu.Unlock()
		log.Infof("Recovered %d agents from comm registry", len(registry))
	}
}
func (cr *CommRegion) UnregisterAgent(agentID uint64) error {
	cr.mu.Lock()
	defer cr.mu.Unlock()

	delete(cr.registry, agentID)

	data, err := json.Marshal(cr.registry)
	if err != nil {
		return fmt.Errorf("failed to marshal registry: %w", err)
	}

	rawKey := configs.EncodeKey(configs.ZoneComm, cr.memSpaceID, []byte(CommRegistryKey))

	err = cr.kvClient.Update(func(txn storage.Transaction) error {
		return txn.Put(rawKey, data)
	})
	if err != nil {
		return fmt.Errorf("failed to persist registry: %w", err)
	}

	// ✅ 删除通讯记录（解锁后调用，避免死锁）
	cr.mu.Unlock()
	err = cr.DeleteAgentCommRecords(agentID)
	cr.mu.Lock()

	if err != nil {
		log.Warnf("Failed to delete comm records for agent %d: %v", agentID, err)
		// Don't fail unregistration — cleanup is best-effort
	}

	return nil
}

// AddAgentCommRecord adds a communication record when an agent registers
// This creates a system message to track agent lifecycle
func (cr *CommRegion) AddAgentCommRecord(agentID uint64, addr, role string) error {
	cr.mu.Lock()
	cr.seq++
	commKey := fmt.Sprintf("comm/agent/%d/register", agentID)
	err := cr.saveSeq(cr.seq)
	cr.mu.Unlock()

	if err != nil {
		return fmt.Errorf("failed to save comm sequence: %w", err)
	}

	rawKey := configs.EncodeKey(configs.ZoneComm, cr.memSpaceID, []byte(commKey))

	msg := CommMessage{
		Key:       string(rawKey),
		FromAgent: 0, // 0 means system
		ToAgent:   agentID,
		RefType:   "system",
		Timestamp: time.Now().Unix(),
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal comm record: %w", err)
	}

	err = cr.kvClient.Update(func(txn storage.Transaction) error {
		return txn.Put(rawKey, data)
	})
	if err != nil {
		return fmt.Errorf("failed to persist comm record: %w", err)
	}

	log.Infof("Added comm record for agent %d registration", agentID)
	return nil
}

// DeleteAgentCommRecords deletes all communication records for a specific agent
// This is called when an agent unregisters or is removed
func (cr *CommRegion) DeleteAgentCommRecords(agentID uint64) error {
	// Scan all comm keys for this agent
	prefix := configs.EncodeKey(configs.ZoneComm, cr.memSpaceID, []byte("comm/"))

	var keysToDelete [][]byte
	err := cr.kvClient.Update(func(txn storage.Transaction) error {
		kvPairs, err := txn.Scan(prefix)
		if err != nil {
			return err
		}

		for _, pair := range kvPairs {
			var msg CommMessage
			if err := json.Unmarshal(pair.Value, &msg); err != nil {
				continue
			}
			// Delete if this message involves the agent (either from or to)
			if msg.FromAgent == agentID || msg.ToAgent == agentID {
				keysToDelete = append(keysToDelete, pair.Key)
			}
		}
		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to scan comm records: %w", err)
	}

	// Batch delete
	if len(keysToDelete) > 0 {
		err = cr.kvClient.Update(func(txn storage.Transaction) error {
			for _, key := range keysToDelete {
				if err := txn.Delete(key); err != nil {
					log.Warnf("Failed to delete comm key %s: %v", string(key), err)
				}
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("failed to delete comm records: %w", err)
		}
		log.Infof("Deleted %d comm records for agent %d", len(keysToDelete), agentID)
	}
	return nil
}
