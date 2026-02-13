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
