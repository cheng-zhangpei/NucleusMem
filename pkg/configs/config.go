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

// ==============================================rebuild the tool definition=====================================

// ToolType 定义工具的四大标准类型
type ToolType string

const (
	TypeHTTP     ToolType = "http"
	TypeShell    ToolType = "shell"
	TypeMCP      ToolType = "mcp"
	TypeDelegate ToolType = "delegate" // 对应 Claude Code / Sub-Agent
)

// ToolDefinition 是存入 MemSpace 的标准元数据
type StandardToolDefinition struct {
	// --- 1. 基础元数据 (所有类型通用) ---
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Tags        []string `json:"tags,omitempty"`

	// --- 2. 类型标识 (决定后续哪个 Config 生效) ---
	Type ToolType `json:"type"`

	// --- 3. 参数定义 (所有类型通用，但建议结构化) ---
	// 即使是 Shell，我们也希望 LLM 以 JSON 形式传参，然后我们在内部渲染
	Parameters []StandardToolParam `json:"parameters"`

	// --- 4. 执行配置 (根据 Type 二选一或多选一) ---
	HTTPConfig     *HTTPExecutorConfig     `json:"http_config,omitempty"`
	ShellConfig    *ShellExecutorConfig    `json:"shell_config,omitempty"`
	MCPConfig      *MCPExecutorConfig      `json:"mcp_config,omitempty"`
	DelegateConfig *DelegateExecutorConfig `json:"delegate_config,omitempty"`

	// --- 5. 运行时约束 (所有类型通用) ---
	TimeoutSeconds int `json:"timeout_seconds,omitempty"` // 默认 30s
	RetryCount     int `json:"retry_count,omitempty"`     // 默认 0
}

// ToolParam 定义单个参数，比纯 JSON Schema 更稳定
type StandardToolParam struct {
	Name        string      `json:"name"`
	Type        string      `json:"type"` // "string", "number", "boolean", "array"
	Description string      `json:"description"`
	Required    bool        `json:"required"`
	Default     interface{} `json:"default,omitempty"`
	Enum        []string    `json:"enum,omitempty"` // 可选值列表
}

// ================= 具体执行器配置 =================

// 1. HTTP 工具配置
type HTTPExecutorConfig struct {
	URL     string            `json:"url"`
	Method  string            `json:"method"` // GET, POST, PUT...
	Headers map[string]string `json:"headers,omitempty"`
	// 是否将参数作为 Query String 还是 Body JSON
	ParamLocation string `json:"param_location,omitempty"` // "query", "body", "path"
}

// 2. Shell 工具配置
type ShellExecutorConfig struct {
	// 命令模板，支持 Go template 语法，如: git commit -m "{{.message}}"
	CommandTemplate string `json:"command_template"`
	WorkDir         string `json:"work_dir,omitempty"` // 留空则使用 Agent 默认工作区
	// 是否允许交互式输入？通常设为 false 以保证自动化
	Interactive bool `json:"interactive,omitempty"`
}

// 3. MCP 工具配置
type MCPExecutorConfig struct {
	ServerName string `json:"server_name"` // 注册的 MCP Server ID
	ToolName   string `json:"tool_name"`   // MCP Server 内部的具体工具名
	// 如果需要传递额外的 Session ID 或 Context
	Metadata map[string]string `json:"metadata,omitempty"`
}

// 4. Delegate (Claude Code) 工具配置
type DelegateExecutorConfig struct {
	// 子 Agent 的类型
	AgentType string `json:"agent_type"` // "claude_code", "python_sandbox"
	// 静态指令前缀 (System Prompt 的一部分)
	InstructionPrefix string `json:"instruction_prefix,omitempty"`
	// 用户输入的哪个字段作为核心任务指令？
	TaskInputField string `json:"task_input_field"` // 例如 "goal" 或 "prompt"
	// 工作目录隔离策略
	SandboxMode string `json:"sandbox_mode,omitempty"` // "none", "docker", "tmp_dir"
}
