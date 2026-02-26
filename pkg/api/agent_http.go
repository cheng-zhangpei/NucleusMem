package api

// TempChatRequest represents a temporary chat request (in-memory only)
type TempChatRequest struct {
	Message string `json:"message"`
}

// TempChatResponse represents the response for temporary chat
type TempChatResponse struct {
	Success      bool   `json:"success"`
	Response     string `json:"response,omitempty"`
	ErrorMessage string `json:"error_message,omitempty"`
}

// ChatRequest represents a persistent chat request (with MemSpace)
type ChatRequest struct {
	Message string `json:"message"`
}

// ChatResponse represents the response for persistent chat
type ChatResponse struct {
	Success      bool   `json:"success"`
	Response     string `json:"response,omitempty"`
	ErrorMessage string `json:"error_message,omitempty"`
}

// AgentHealthRequest represents a health check request with monitor binding info
type AgentHealthRequest struct {
	MonitorID uint64 `json:"monitor_id"`
}

// AgentHealthResponse represents agent health status with binding info
type AgentHealthResponse struct {
	Status    string `json:"status"` // "healthy" or "unhealthy"
	IsJob     bool   `json:"is_job"`
	MonitorID uint64 `json:"monitor_id,omitempty"` // 当前绑定的 Monitor ID
}
type ShutdownResponse struct {
	Success bool `json:"success"`
}
type NotifyRequest struct {
	Key     string `json:"key"`
	Content string `json:"content"`
}

// NotifyResponse is the response for notify operation
type NotifyResponse struct {
	Success      bool   `json:"success"`
	Result       string `json:"result,omitempty"` // ✅ 任务处理结果
	ErrorMessage string `json:"error_message,omitempty"`
}

// BindMemSpaceRequest is used to bind an agent to a MemSpace
type BindMemSpaceRequest struct {
	MemSpaceID string `json:"memspace_id"`
	AgentID    uint64 `json:"agent_id"`
	Type       string `json:"type"` // "private" or "public"
	HttpAddr   string `json:"http_addr"`
}

// UnbindMemSpaceRequest is used to unbind an agent from a MemSpace
type UnbindMemSpaceRequest struct {
	AgentID    uint64 `json:"agent_id"`
	MemSpaceID string `json:"memspace_id"`
}
type CommunicateRequest struct {
	TargetAgentID string `json:"target_agent_id"` // 目标 Agent ID
	Key           string `json:"key"`             // key
	Content       string `json:"content"`         // 通讯内容
}

// CommunicateResponse is the response for communicate operation
type CommunicateResponse struct {
	Result       string `json:"result"`
	Success      bool   `json:"success"`
	ErrorMessage string `json:"error_message,omitempty"`
}

type SubmitTaskRequest struct {
	Type             string                 `json:"type"` // "decompose", "chat", "tool", etc.
	Content          string                 `json:"content"`
	AvailableTools   []string               `json:"available_tools,omitempty"`
	AvailableMemTags []string               `json:"available_mem_tags,omitempty"`
	MaxRetry         int                    `json:"max_retry,omitempty"`
	ToolName         string                 `json:"tool_name,omitempty"`
	Params           map[string]interface{} `json:"params,omitempty"`
}

type SubmitTaskResponse struct {
	Success      bool   `json:"success"`
	TaskID       string `json:"task_id"`
	ErrorMessage string `json:"error,omitempty"`
}

type GetTaskResultRequest struct {
	TaskID    string `json:"task_id"`
	TimeoutMs int64  `json:"timeout_ms"` // how long to wait
}

type GetTaskResultResponse struct {
	Success      bool   `json:"success"`
	TaskID       string `json:"task_id"`
	Result       string `json:"result"`
	ErrorMessage string `json:"error,omitempty"`
	Done         bool   `json:"done"`
}
