// pkg/memspace/memspace.go
package memspace

import (
	"NucleusMem/pkg/client"
	"NucleusMem/pkg/configs"
	memspace_region "NucleusMem/pkg/runtime/memspace/region"
	"NucleusMem/pkg/storage"
	"NucleusMem/pkg/storage/tinykv-client"
	"context"
	"fmt"
	"github.com/pingcap-incubator/tinykv/log"
	"github.com/pkg/errors"
	"sync"
	"time"
)

type AgentBinding struct {
	AgentID uint64
	Addr    string
	Role    string
	BoundAt int64
}
type MemSpaceType string

const (
	MemSpaceTypePrivate MemSpaceType = "private"
	MemSpaceTypePublic  MemSpaceType = "public"
)

type MemSpace struct {
	ID          string
	Type        MemSpaceType
	Description string
	OwnerID     uint64 // only meaningful for private
	summaryCnt  uint64

	// New fields for background worker
	summaryThreshold  uint64 // trigger threshold (e.g., 10 memories)
	workerCtx         context.Context
	workerCancel      context.CancelFunc
	workerWG          sync.WaitGroup
	lastSummarizedSeq uint64
	chatServer        *client.ChatServerClient
	MemoryRegion      *memspace_region.MemoryRegion
	CommRegion        *memspace_region.CommRegion
	SummaryRegion     *memspace_region.SummaryRegion
	mu                sync.RWMutex
	kvClient          *tinykv_client.MemClient
	status            configs.MemSpaceStatus
	boundAgents       map[uint64]*AgentBinding
	httpAddr          string
}

// NewMemSpace creates a new MemSpace instance from config
func NewMemSpace(config *configs.MemSpaceConfig) (*MemSpace, error) {
	// Validate required fields
	if config.MemSpaceID == 0 {
		return nil, errors.New("memspace_id is required")
	}
	if config.Type != "private" && config.Type != "public" {
		return nil, errors.New("type must be 'private' or 'public'")
	}
	// Set defaults
	summaryCnt := config.SummaryCnt
	if summaryCnt == 0 {
		summaryCnt = 5 // default batch size
	}

	summaryThreshold := config.SummaryThreshold
	if summaryThreshold == 0 {
		summaryThreshold = 10 // default threshold
	}

	// Create clients
	kvClient, err := tinykv_client.NewMemClient(config.PdAddr)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create TinyKV client")
	}

	serverClient := client.NewEmbeddingServerClient(config.EmbeddingClientAddr)
	chatServerClient := client.NewChatServerClient(config.LightModelAddr)
	ctx, cancel := context.WithCancel(context.Background())

	// Convert type string to enum
	var memspaceType MemSpaceType
	if config.Type == "private" {
		memspaceType = MemSpaceTypePrivate
	} else {
		memspaceType = MemSpaceTypePublic
	}

	ms := &MemSpace{
		ID:               fmt.Sprintf("%d", config.MemSpaceID),
		Type:             memspaceType,
		Description:      config.Description,
		OwnerID:          config.OwnerID,
		summaryCnt:       summaryCnt,
		summaryThreshold: summaryThreshold,
		workerCtx:        ctx,
		workerCancel:     cancel,
		chatServer:       chatServerClient,
		kvClient:         kvClient,
		MemoryRegion:     memspace_region.NewMemoryRegion(kvClient, config.MemSpaceID, serverClient),
		CommRegion:       memspace_region.NewCommRegion(kvClient, config.MemSpaceID),
		SummaryRegion:    memspace_region.NewSummaryRegion(kvClient, chatServerClient, config.MemSpaceID),
		status:           configs.MemSpaceStatusInactive,
		boundAgents:      make(map[uint64]*AgentBinding),
		httpAddr:         config.HttpAddr,
	}

	// Start background summary worker
	// ms.startSummaryWorker()

	return ms, nil
}
func (m *MemSpace) WriteMemory(memory string, agentId uint64) error {
	// todo (cheng) check the authority
	if memory == "" {
		return errors.New("memory content cannot be empty")
	}
	return m.MemoryRegion.Write(agentId, memory)
}

// GetMemoryContext retrieves combined context from Summary and Memory regions
// - summaryBefore: get latest summary before this timestamp
// - query: semantic search query
// - n: number of similar memories to retrieve
func (m *MemSpace) GetMemoryContext(summaryBefore int64, query string, n int) (summary string, memories []string, err error) {
	// 1. Get latest summary before timestamp
	summaryRecords, err := m.SummaryRegion.GetBefore(summaryBefore)
	if err != nil {
		return "", nil, fmt.Errorf("failed to get summaries: %w", err)
	}
	var latestSummary string
	if len(summaryRecords) > 0 {
		// Assume records are sorted by timestamp (or find max)
		latest := summaryRecords[0]
		for _, s := range summaryRecords {
			if s.Timestamp > latest.Timestamp {
				latest = s
			}
		}
		latestSummary = latest.Content
	}
	// 2. Search top-n similar memories
	memories, err = m.MemoryRegion.Search(query, n)
	if err != nil {
		return "", nil, fmt.Errorf("failed to search memories: %w", err)
	}
	return latestSummary, memories, nil
}

// todo this will move the monitor (worker poll arch)
// startSummaryWorker launches a background goroutine for auto-summarization
func (m *MemSpace) startSummaryWorker() {
	m.workerWG.Add(1)
	go func() {
		defer m.workerWG.Done()
		ticker := time.NewTicker(5 * time.Second) // check every 5s
		defer ticker.Stop()
		for {
			select {
			case <-m.workerCtx.Done():
				log.Infof("Summary worker stopped for MemSpace %s", m.ID)
				return
			case <-ticker.C:
				m.trySummarize()
			}
		}
	}()
}
func (m *MemSpace) trySummarize() {
	count, err := m.MemoryRegion.Count()
	if err != nil || count < m.summaryThreshold {
		return
	}

	// 获取 (key, record) 对
	batch, err := m.MemoryRegion.GetAllWithKeys()
	if err != nil {
		log.Warnf("Failed to get memory batch: %v", err)
		return
	}

	var contents []string
	var sourceIDs []string
	var deleteKeys []string
	var maxSeq uint64

	for _, item := range batch {
		contents = append(contents, item.Record.Content)
		sourceIDs = append(sourceIDs, item.Record.ID)
		deleteKeys = append(deleteKeys, item.Key)

		seq := configs.ParseMemSeqFromKey(item.Key)
		if seq > maxSeq {
			maxSeq = seq
		}
	}

	// gen the summary
	summaryRec, err := m.SummaryRegion.GenerateSummary(contents, sourceIDs)
	if err != nil {
		log.Errorf("Summary generation failed: %v", err)
		return
	}
	if err := m.SummaryRegion.Add(summaryRec); err != nil {
		log.Errorf("Failed to save summary: %v", err)
		return
	}

	err = m.MemoryRegion.DeleteBatchByKeys(deleteKeys)
	if err != nil {
		log.Warnf("Failed to delete compressed memories: %v", err)
	}

	m.lastSummarizedSeq = maxSeq
	log.Infof("Summary added and %d memories deleted. LastIndex: %d", len(deleteKeys), m.lastSummarizedSeq)
}
func (m *MemSpace) RegisterAgent(agentID uint64, addr, role string) error {
	if m.Type == MemSpaceTypePrivate {
		return fmt.Errorf("the memspace type is private")
	}
	return m.CommRegion.RegisterAgent(agentID, addr, role)
}
func (m *MemSpace) UnRegisterAgent(agentID uint64) error {
	if m.Type == MemSpaceTypePrivate {
		return fmt.Errorf("the memspace type is private")
	}
	return m.CommRegion.UnregisterAgent(agentID)
}
func (m *MemSpace) SendMessage(fromAgent, toAgent uint64, key, content string) (string, error) {
	if m.Type == MemSpaceTypePrivate {
		return "", fmt.Errorf("the memspace type is private")
	}

	return m.CommRegion.SendMessage(fromAgent, toAgent, key, content)
}
func (m *MemSpace) ListAgents() []memspace_region.AgentRegistryEntry {
	if m.Type == MemSpaceTypePrivate {
		return nil
	}
	return m.CommRegion.ListAgents()
}

// BindAgent binds an agent to this MemSpace and registers in CommRegion
func (m *MemSpace) BindAgent(agentID uint64, addr, role string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 1. 检查是否已绑定
	if _, exists := m.boundAgents[agentID]; exists {
		return fmt.Errorf("agent %d already bound to memspace %s", agentID, m.ID)
	}
	// 2. 权限控制：根据 MemSpace 类型判断是否允许绑定
	if m.Type == MemSpaceTypePrivate {
		// Private MemSpace 只能绑定 OwnerID 对应的 Agent
		if agentID != m.OwnerID {
			return fmt.Errorf("private memspace %s can only be bound by owner (agent %d), got agent %d",
				m.ID, m.OwnerID, agentID)
		}
	}
	// Public MemSpace 允许任意 Agent 绑定（无额外限制）
	// 3. 添加到绑定列表
	m.boundAgents[agentID] = &AgentBinding{
		AgentID: agentID,
		Addr:    addr,
		Role:    role,
		BoundAt: time.Now().Unix(),
	}

	// 4. 更新状态
	if m.status == configs.MemSpaceStatusInactive {
		m.status = configs.MemSpaceStatusActive
		log.Infof("MemSpace %s activated (agent %d bound)", m.ID, agentID)
	}

	// 5. 注册到通讯区（仅 Public MemSpace）
	if m.Type == MemSpaceTypePublic {
		m.mu.Unlock()
		if err := m.CommRegion.RegisterAgent(agentID, addr, role); err != nil {
			log.Warnf("Failed to register agent %d in comm region: %v", agentID, err)
		}
		m.mu.Lock()
	}

	log.Infof("Agent %d bound to MemSpace %s (addr: %s, role: %s, type: %s)",
		agentID, m.ID, addr, role, m.Type)
	return nil
}
func (ms *MemSpace) GetByKey(rawKey []byte) ([]byte, error) {
	var value []byte
	err := ms.kvClient.Update(func(txn storage.Transaction) error {
		data, err := txn.Get(rawKey)
		if err != nil {
			return err // e.g., not found
		}
		value = data
		return nil
	})
	if err != nil {
		return nil, err
	}
	return value, nil
}

func (m *MemSpace) UnBindAgent(agentID uint64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 1. 检查是否存在
	if _, exists := m.boundAgents[agentID]; !exists {
		return fmt.Errorf("agent %d not bound to memspace %s", agentID, m.ID)
	}

	// 2. 权限控制：Private MemSpace 只能由 Owner 解绑
	if m.Type == MemSpaceTypePrivate && agentID != m.OwnerID {
		return fmt.Errorf("private memspace %s can only be unbound by owner (agent %d), got agent %d",
			m.ID, m.OwnerID, agentID)
	}

	// 3. 从绑定列表删除
	delete(m.boundAgents, agentID)

	// 4. 更新状态
	if len(m.boundAgents) == 0 && m.status == configs.MemSpaceStatusActive {
		m.status = configs.MemSpaceStatusInactive
		log.Infof("MemSpace %s deactivated (no agents bound)", m.ID)
	}

	// 5. 从通讯区注销（仅 Public MemSpace）
	if m.Type == MemSpaceTypePublic {
		m.mu.Unlock()
		if err := m.CommRegion.UnregisterAgent(agentID); err != nil {
			log.Warnf("Failed to unregister agent %d from comm region: %v", agentID, err)
		}
		m.mu.Lock()
	}

	log.Infof("Agent %d unbound from MemSpace %s", agentID, m.ID)
	return nil
}

// GetStatus returns the current status of the MemSpace
func (m *MemSpace) GetStatus() configs.MemSpaceStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.status
}

// IsBound checks if an agent is bound to this MemSpace
func (m *MemSpace) IsBound(agentID uint64) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, exists := m.boundAgents[agentID]
	return exists
}
func (m *MemSpace) Stop() {
	m.workerCancel()
	m.workerWG.Wait()
}
