package agent_manager

import "sync"

// AgentInfo 缓存的 Agent 元信息
type AgentInfo struct {
	AgentID    string
	Status     string // Running, Failed, Stopped
	NodeID     uint64 // 所属 Monitor 节点 ID
	NodeAddr   string // Monitor 节点标识（如 hostname）
	MemSpaceID uint64 // 可选：挂载的 MemSpace ID（未来扩展）
}

type AgentCache struct {
	mu     sync.RWMutex
	agents map[string]*AgentInfo // key: agent_id (string)
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

// GetAllAgents 返回所有缓存的 agents（用于 debug 或全局视图）
func (ac *AgentCache) GetAllAgents() []*AgentInfo {
	ac.mu.RLock()
	defer ac.mu.RUnlock()
	agents := make([]*AgentInfo, 0, len(ac.agents))
	for _, a := range ac.agents {
		agents = append(agents, a)
	}
	return agents
}
