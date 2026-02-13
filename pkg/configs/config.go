package configs

// pkg/configs/agent_config.go
type AgentConfig struct {
	AgentId             uint64          `yaml:"agent_id"`
	AgentManagerAddr    string          `yaml:"agent_manager_addr"`
	MemSpaceManagerAddr string          `yaml:"memspace_manager_addr"`
	PrivateMemSpaceInfo *MemSpaceInfo   `yaml:"private_memspace_info,omitempty"`
	PublicMemSpaceInfo  []*MemSpaceInfo `yaml:"public_memspace_info,omitempty"`
	ChatServerAddr      string          `yaml:"chat_server_addr"`
	VectorServerAddr    string          `yaml:"vector_server_addr"`
	Role                string          `yaml:"role"`
	Image               string          `yaml:"image"`
	Path                string          `yaml:"path"`
	IsJob               bool            `yaml:"is_job"`
	HttpAddr            string          `yaml:"http_addr"`
}

type MemSpaceInfo struct {
	MemSpaceId   uint64 `yaml:"memspace_id"`
	MemSpaceAddr string `yaml:"memspace_addr"`
}

// AgentManagerConfig AgentManager 的启动配置
type AgentManagerConfig struct {
	MonitorURLs map[uint64]string `yaml:"monitor_urls"` // nodeID -> http://host:port
}

type MonitorConfig struct {
	NodeID     uint64 `yaml:"node_id"`
	MonitorUrl string `yaml:"monitor_url"`
}
