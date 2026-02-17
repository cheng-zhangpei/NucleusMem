package memspace_region

import (
	tinykv_client "NucleusMem/pkg/storage/tinykv-client"
	"sync"
	"time"
)

type CommMessage struct {
	Key       string `json:"key"` // points to Memory or Summary Region
	FromAgent uint64 `json:"from_agent"`
	ToAgent   uint64 `json:"to_agent"`
	RefType   string `json:"ref_type"` // "memory" or "summary"
	Timestamp int64  `json:"timestamp"`
}

type AgentRegistryEntry struct {
	AgentID   uint64 `json:"agent_id"`
	Addr      string `json:"addr"` // e.g., "localhost:9001"
	Role      string `json:"role"`
	Timestamp int64  `json:"timestamp"`
}

type CommRegion struct {
	mu       sync.RWMutex
	messages []CommMessage
	registry map[uint64]AgentRegistryEntry
	kvClient *tinykv_client.MemClient
}

func NewCommRegion(kvClient *tinykv_client.MemClient) *CommRegion {
	return &CommRegion{
		messages: make([]CommMessage, 0),
		registry: make(map[uint64]AgentRegistryEntry),
		kvClient: kvClient,
	}
}

// RegisterAgent registers an agent in the address table
func (cr *CommRegion) RegisterAgent(agentID uint64, addr, role string) {
	cr.mu.Lock()
	defer cr.mu.Unlock()
	cr.registry[agentID] = AgentRegistryEntry{
		AgentID:   agentID,
		Addr:      addr,
		Role:      role,
		Timestamp: time.Now().Unix(),
	}
}

// GetAgent returns agent registry entry
func (cr *CommRegion) GetAgent(agentID uint64) (AgentRegistryEntry, bool) {
	cr.mu.RLock()
	defer cr.mu.RUnlock()
	entry, ok := cr.registry[agentID]
	return entry, ok
}

// PushMessage writes a communication signal
func (cr *CommRegion) PushMessage(msg *CommMessage) {
	if msg.Timestamp == 0 {
		msg.Timestamp = time.Now().Unix()
	}
	cr.mu.Lock()
	defer cr.mu.Unlock()
	cr.messages = append(cr.messages, *msg)
}

// PopMessagesForAgent retrieves and removes all messages for an agent
func (cr *CommRegion) PopMessagesForAgent(agentID uint64) []CommMessage {
	cr.mu.Lock()
	defer cr.mu.Unlock()

	var result []CommMessage
	remaining := make([]CommMessage, 0)

	for _, msg := range cr.messages {
		if msg.ToAgent == agentID {
			result = append(result, msg)
		} else {
			remaining = append(remaining, msg)
		}
	}

	cr.messages = remaining
	return result
}
