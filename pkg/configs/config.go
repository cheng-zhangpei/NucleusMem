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
	role                string          `yaml:"role"`
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
	NodeID          uint64            `yaml:"node_id"`
	MonitorUrl      string            `yaml:"monitor_url"`
	AgentManagerUrl string            `yaml:"agent_manager_url"`
	agentsURLs      map[uint64]string `yaml:"agent_urls"`
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

type ToolDefinition struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Tags        []string          `json:"tags,omitempty"` // the tag of the tools
	Parameters  []ToolParam       `json:"parameters,omitempty"`
	ReturnType  string            `json:"return_type,omitempty"` // "string", "object", "list"
	Endpoint    string            `json:"endpoint,omitempty"`    // HTTP endpoint or function name
	Metadata    map[string]string `json:"metadata,omitempty"`
	CreatedAt   int64             `json:"created_at"`
	ExecType    string            `json:"exec_type"` // "http", "shell", "mcp", "grpc"

}

type ToolParam struct {
	Name     string `json:"name"`
	Type     string `json:"type"` // "string", "int", "bool", "object"
	Required bool   `json:"required"`
	Default  string `json:"default,omitempty"`
}
type ToolDAG struct {
	Nodes []ToolDAGNode `json:"nodes"`
	Edges []ToolDAGEdge `json:"edges"`
}

// ToolDAGNode represents a single tool execution unit
type ToolDAGNode struct {
	ToolName string                 `json:"tool_name"`
	Params   map[string]interface{} `json:"params,omitempty"`
}

// ToolDAGEdge defines dependency between tools
// If there is an edge from A -> B, B depends on A
type ToolDAGEdge struct {
	From   string   `json:"from"`
	To     string   `json:"to"`
	Fields []string `json:"fields,omitempty"` // Output fields from 'From' passed to 'To'
}
type ToolExecResult struct {
	ToolName string                 `json:"tool_name"`
	Output   map[string]interface{} `json:"output,omitempty"`
	Error    string                 `json:"error,omitempty"`
	Status   string                 `json:"status"`            // "completed" | "failed"
	Seq      uint64                 `json:"seq,omitempty"`     // Optional: execution sequence number
	DoneAt   int64                  `json:"done_at,omitempty"` // Optional: completion timestamp
}

// ToolExecBatchResult groups multiple tool execution results
// Used for recording an entire DAG's results atomically
type ToolExecBatchResult struct {
	ViewSpaceID string                     `json:"viewspace_id,omitempty"`
	Results     map[string]*ToolExecResult `json:"results"` // toolName -> result
	Timestamp   int64                      `json:"timestamp"`
}
type ToolExecRecord struct {
	Seq       uint64                 `json:"seq"`
	ToolName  string                 `json:"tool_name"`
	AgentID   uint64                 `json:"agent_id"`
	Input     map[string]interface{} `json:"input,omitempty"`
	Output    map[string]interface{} `json:"output,omitempty"`
	Status    ToolExecStatus         `json:"status"`
	Error     string                 `json:"error,omitempty"`
	StartedAt int64                  `json:"started_at,omitempty"`
	DoneAt    int64                  `json:"done_at,omitempty"`
}
type ToolExecStatus string

const (
	ToolExecPending   ToolExecStatus = "pending"
	ToolExecRunning   ToolExecStatus = "running"
	ToolExecCompleted ToolExecStatus = "completed"
	ToolExecFailed    ToolExecStatus = "failed"
)
