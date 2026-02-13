// pkg/controller/agent_manager/agent_cache.go
package agent_manager

import "sync"

// AgentInfo 缓存的 Agent 元信息（使用 uint64 ID）
type AgentInfo struct {
	AgentID    uint64 // ← 统一为 uint64
	Status     string // Running, Connected, Stopped, Failed
	NodeID     uint64 // 所属 Monitor 节点 ID
	NodeAddr   string // Monitor 地址（如 "localhost:8081"）
	MemSpaceID uint64 // 可选：主 MemSpace ID
	HTTPAddr   string // Agent 的 HTTP 地址（用于直接调用）
}

// AgentCache 线程安全的 Agent 元数据缓存
type AgentCache struct {
	mu     sync.RWMutex
	agents map[uint64]*AgentInfo // ← key 改为 uint64
}

func NewAgentCache() *AgentCache {
	return &AgentCache{
		agents: make(map[uint64]*AgentInfo),
	}
}

// UpdateAgent 更新或添加 Agent 信息
func (ac *AgentCache) UpdateAgent(info *AgentInfo) {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	ac.agents[info.AgentID] = info
}

// RemoveAgent 删除指定 Agent
func (ac *AgentCache) RemoveAgent(agentID uint64) {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	delete(ac.agents, agentID)
}

// GetAgent 获取指定 Agent 信息
func (ac *AgentCache) GetAgent(agentID uint64) (*AgentInfo, bool) {
	ac.mu.RLock()
	defer ac.mu.RUnlock()
	info, ok := ac.agents[agentID]
	return info, ok
}

// GetAllAgents 返回所有缓存的 Agents（用于调试/展示）
func (ac *AgentCache) GetAllAgents() []*AgentInfo {
	ac.mu.RLock()
	defer ac.mu.RUnlock()

	agents := make([]*AgentInfo, 0, len(ac.agents))
	for _, a := range ac.agents {
		// 创建副本避免外部修改
		agentCopy := *a
		agents = append(agents, &agentCopy)
	}
	return agents
}

// Size 返回缓存中 Agent 数量
func (ac *AgentCache) Size() int {
	ac.mu.RLock()
	defer ac.mu.RUnlock()
	return len(ac.agents)
}

// Display 打印当前缓存状态（用于调试）
func (ac *AgentCache) Display() {
	agents := ac.GetAllAgents()
	if len(agents) == 0 {
		println("No agents in cache")
		return
	}

	println("Current Agent Cache:")
	println("AgentID\tStatus\t\tNodeID\tNodeAddr\t\tHTTPAddr")
	for _, a := range agents {
		println(
			a.AgentID, "\t",
			a.Status, "\t",
			a.NodeID, "\t",
			a.NodeAddr, "\t",
			a.HTTPAddr,
		)
	}
}
