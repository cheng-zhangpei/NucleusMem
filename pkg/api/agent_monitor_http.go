package api

// 接收由manager通往monitor的数据
// LaunchAgentRequestHTTP 是 HTTP 接收的请求体
type LaunchAgentRequestHTTP struct {
	AgentID            uint64            `json:"agent_id"`
	Role               string            `json:"role"`
	Image              string            `json:"image,omitempty"`      // Docker 镜像
	BinPath            string            `json:"bin_path,omitempty"`   // 二进制路径
	MountMemSpaceNames []string          `json:"mount_memspace_names"` // 要挂载的 MemSpace 名称列表
	Env                map[string]string `json:"env,omitempty"`        // 环境变量注入
	HttpAddr           string            `json:"http_addr,omitempty"`  // 环境变量注入
}

// LaunchAgentResponseHTTP 是 HTTP 返回体
type LaunchAgentResponseHTTP struct {
	Success      bool   `json:"success"`
	ErrorMessage string `json:"error_message,omitempty"`

	// 新增字段：返回新 Agent 的关键信息
	AgentID  uint64 `json:"agent_id,omitempty"`
	HttpAddr string `json:"http_addr,omitempty"` // Agent 的 HTTP 地址
	NodeID   uint64 `json:"node_id,omitempty"`   // 所属 Monitor ID
}

// StopAgentRequestHTTP
type StopAgentRequestHTTP struct {
	AgentID uint64 `json:"agent_id"`
}
type StopAgentResponseHTTP struct {
	Success      bool   `json:"success"`
	ErrorMessage string `json:"error_message,omitempty"`
	AgentID      uint64 `json:"agent_id"` // 被停止的 Agent ID
}

type MonitorHeartbeatHTTP struct {
	NodeID       string               `json:"node_id"`
	CPUUsage     float64              `json:"cpu_usage"`
	MemUsage     float64              `json:"mem_usage"`
	ActiveAgents int32                `json:"active_agents"`
	Agents       []AgentRuntimeStatus `json:"agents"`
	Timestamp    int64                `json:"timestamp"`
}

type AgentRuntimeStatus struct {
	AgentID      string `json:"agent_id"` // 注意：用 string（和你原 proto 一致）
	Phase        string `json:"phase"`
	StartTime    int64  `json:"start_time"`
	RestartCount int32  `json:"restart_count"`
	Addr         string `json:"addr,omitempty"`
}
