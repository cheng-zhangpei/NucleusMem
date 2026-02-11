package api

// LaunchAgentRequestHTTP 是 HTTP 接收的请求体
type LaunchAgentRequestHTTP struct {
	AgentID            uint64            `json:"agent_id"`
	Role               string            `json:"role"`
	Image              string            `json:"image,omitempty"`      // Docker 镜像
	BinPath            string            `json:"bin_path,omitempty"`   // 二进制路径
	MountMemSpaceNames []string          `json:"mount_memspace_names"` // 要挂载的 MemSpace 名称列表
	Env                map[string]string `json:"env,omitempty"`        // 环境变量注入
}

// LaunchAgentResponseHTTP 是 HTTP 返回体
type LaunchAgentResponseHTTP struct {
	Success      bool   `json:"success"`
	ErrorMessage string `json:"error_message,omitempty"`
}

// StopAgentRequestHTTP
type StopAgentRequestHTTP struct {
	AgentID uint64 `json:"agent_id"`
}

// StopAgentResponseHTTP
type StopAgentResponseHTTP struct {
	Success      bool   `json:"success"`
	ErrorMessage string `json:"error_message,omitempty"`
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
}
