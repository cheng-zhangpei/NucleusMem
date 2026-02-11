package configs

type AgentConfig struct {
	AgentId             uint64
	AgentManagerAddr    string
	MemSpaceManagerAddr string
	PrivateMemSpaceInfo *MemSpaceInfo
	PublicMemSpaceInfo  []*MemSpaceInfo
	ChatServerAddr      string
	VectorServerAddr    string
	Role                string
	Image               string
	Path                string
	IsJob               bool
}

type MemSpaceInfo struct {
	MemSpaceId   uint64
	MemSpaceAddr string
}

// AgentManagerConfig AgentManager 的启动配置
type AgentManagerConfig struct {
	MonitorURLs map[uint64]string `yaml:"monitor_urls"` // nodeID -> http://host:port
}

type MonitorConfig struct {
	MonitorUrl string
}
