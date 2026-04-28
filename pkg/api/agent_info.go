package api

type AgentInfo struct {
	AgentID    uint64 // ← 统一为 uint64
	Status     string // Running, Connected, Stopped, Failed
	NodeID     uint64 // 所属 Monitor 节点 ID
	NodeAddr   string // Monitor 地址（如 "localhost:8081"）
	MemSpaceID uint64 // 可选：主 MemSpace ID
	HTTPAddr   string // Agent 的 HTTP 地址（用于直接调用）
}
