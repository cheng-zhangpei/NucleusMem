package agent_monitor

type LaunchAgentInternalRequest struct {
	AgentID            uint64
	Role               string
	Image              string
	BinPath            string
	MountMemSpaceNames []string
	HttpAddress        string
	Env                map[string]string
	ConfigFilePath     string
}

// 内部状态结构（仅在 monitor 包内使用）
type MonitorNodeStatusInternal struct {
	NodeID       string
	CPUUsage     float64
	MemUsage     float64
	ActiveAgents int32
	Agents       []AgentRuntimeStatusInternal
	Timestamp    int64
}

type AgentRuntimeStatusInternal struct {
	AgentID      string
	Phase        string
	StartTime    int64
	RestartCount int32
}
