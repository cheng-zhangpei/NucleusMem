package agent_manager

import "sync"

// AgentInfo 定义你希望缓存的 Agent 信息
type AgentInfo struct {
	AgentID    string
	Status     string // Running, Pending, Failed
	NodeAddr   string
	MemSpaceID uint64
}

type AgentCache struct {
	mu     sync.RWMutex
	agents map[string]*AgentInfo
}

func NewAgentCache() *AgentCache {
	return &AgentCache{
		agents: make(map[string]*AgentInfo),
	}
}

func (ac *AgentCache) UpdateAgent(info *AgentInfo) {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	ac.agents[info.AgentID] = info
}

func (ac *AgentCache) GetAgent(agentID string) (*AgentInfo, bool) {
	ac.mu.RLock()
	defer ac.mu.RUnlock()
	info, ok := ac.agents[agentID]
	return info, ok
}
