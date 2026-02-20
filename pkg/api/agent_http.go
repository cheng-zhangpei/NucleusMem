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
	Key      string            `json:"key,omitempty"`     // e.g., "memory/101/5"
	Content  string            `json:"content,omitempty"` // direct content
	Metadata map[string]string `json:"metadata,omitempty"`
}

type NotifyResponse struct {
	Success      bool   `json:"success"`
	ErrorMessage string `json:"error_message,omitempty"`
}

// BindMemSpaceRequest is used to bind an agent to a MemSpace
type BindMemSpaceRequest struct {
	MemSpaceID string `json:"memspace_id"`
	Type       string `json:"type"` // "private" or "public"
	HttpAddr   string `json:"http_addr"`
}

// UnbindMemSpaceRequest is used to unbind an agent from a MemSpace
type UnbindMemSpaceRequest struct {
	MemSpaceID string `json:"memspace_id"`
}
