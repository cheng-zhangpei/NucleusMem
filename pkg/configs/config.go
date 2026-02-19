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
	HttpAddr    string            `yaml:"http_addr"`
	MonitorURLs map[uint64]string `yaml:"monitor_urls"` // nodeID -> http://host:port
}

type MonitorConfig struct {
	NodeID          uint64 `yaml:"node_id"`
	MonitorUrl      string `yaml:"monitor_url"`
	AgentManagerUrl string `yaml:"agent_manager_url"`
}

// MemSpaceManagerConfig holds configuration for MemSpaceManager
type MemSpaceManagerConfig struct {
	// MonitorURLs maps NodeID to MemSpaceMonitor HTTP addresses
	// Example: {1: "localhost:9081", 2: "localhost:9082"}
	MonitorURLs map[uint64]string `yaml:"monitor_urls"`
	// ListenAddr is the address where MemSpaceManager HTTP server listens
	ListenAddr string `yaml:"listen_addr"`
}

// MemSpaceMonitorConfig holds configuration for MemSpaceMonitor
type MemSpaceMonitorConfig struct {
	NodeID             uint64            `yaml:"node_id"`
	MonitorUrl         string            `yaml:"monitor_url"`
	MemSpaceManagerURL string            `yaml:"memspace_manager_url"`
	MemSpaceUrls       map[uint64]string `yaml:"memspace_urls"`
}
type MemSpaceConfig struct {
	MemSpaceID          uint64 `yaml:"memspace_id"`
	Name                string `yaml:"name"`
	Type                string `yaml:"type"` // "private" or "public"
	OwnerID             uint64 `yaml:"owner_id"`
	Description         string `yaml:"description"`
	HttpAddr            string `yaml:"http_addr"`
	PdAddr              string `yaml:"pd_addr"`
	EmbeddingClientAddr string `yaml:"embedding_client_addr"`
	LightModelAddr      string `yaml:"light_model_addr"`
	SummaryCnt          uint64 `yaml:"summary_cnt"`
	SummaryThreshold    uint64 `yaml:"summary_threshold"`
	BinPath             string `yaml:"bin_path"`
	ConfigFilePath      string `yaml:"config_file_path"`
}

// MemSpaceStatus represents the lifecycle state of a MemSpace
type MemSpaceStatus string

const (
	MemSpaceStatusInactive MemSpaceStatus = "inactive" // No agents bound
	MemSpaceStatusActive   MemSpaceStatus = "active"   // At least one agent bound
	MemSpaceStatusStopping MemSpaceStatus = "stopping" // Being shut down
	MemSpaceStatusStopped  MemSpaceStatus = "stopped"  // Fully stopped
)
