package api

// pkg/api/memspace.go
type MemSpaceHealthResponse struct {
	Status      string `json:"status"`
	MemSpaceID  string `json:"memspace_id"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	OwnerID     uint64 `json:"owner_id"`
	Description string `json:"description"`
	Timestamp   int64  `json:"timestamp"`
}

// WriteMemory
type WriteMemoryRequest struct {
	Content string `json:"content"`
	AgentID string `json:"agent_id"` // string to avoid JSON number issues
}
type WriteMemoryResponse struct {
	Success      bool   `json:"success"`
	ErrorMessage string `json:"error_message,omitempty"`
}

// GetMemoryContext
type GetMemoryContextRequest struct {
	SummaryBefore int64  `json:"summary_before"`
	Query         string `json:"query"`
	N             int    `json:"n"`
}
type GetMemoryContextResponse struct {
	Success      bool     `json:"success"`
	Summary      string   `json:"summary"`
	Memories     []string `json:"memories"`
	ErrorMessage string   `json:"error_message,omitempty"`
}

// RegisterAgent
type RegisterAgentRequest struct {
	AgentID string `json:"agent_id"`
	Addr    string `json:"addr"`
	Role    string `json:"role"`
}
type RegisterAgentResponse struct {
	Success      bool   `json:"success"`
	ErrorMessage string `json:"error_message,omitempty"`
}

// UnregisterAgent
type UnregisterAgentRequest struct {
	AgentID string `json:"agent_id"`
}
type UnregisterAgentResponse struct {
	Success      bool   `json:"success"`
	ErrorMessage string `json:"error_message,omitempty"`
}

// SendMessage
type SendMessageRequest struct {
	FromAgent string `json:"from_agent"`
	ToAgent   string `json:"to_agent"`
	Key       string `json:"key"`
	RefType   string `json:"ref_type"`
}
type SendMessageResponse struct {
	Success      bool   `json:"success"`
	ErrorMessage string `json:"error_message,omitempty"`
}

// ListAgents
type AgentRegistryEntry struct {
	AgentID   string `json:"agent_id"`
	Addr      string `json:"addr"`
	Role      string `json:"role"`
	Timestamp int64  `json:"timestamp"`
}
type ListAgentsResponse struct {
	Success bool                 `json:"success"`
	Agents  []AgentRegistryEntry `json:"agents"`
}

// BindAgent
type BindAgentRequest struct {
	AgentID string `json:"agent_id"`
}
type BindAgentResponse struct {
	Success      bool   `json:"success"`
	ErrorMessage string `json:"error_message,omitempty"`
}

// UnbindAgent
type UnbindAgentRequest struct {
	AgentID string `json:"agent_id"`
}
type UnbindAgentResponse struct {
	Success      bool   `json:"success"`
	ErrorMessage string `json:"error_message,omitempty"`
}
type GetByKeyRequest struct {
	RawKey string `json:"raw_key"` // plain string, e.g., "memory/1001/5"
}

type GetByKeyResponse struct {
	Success bool   `json:"success"`
	Value   string `json:"value,omitempty"` // raw JSON string
	Error   string `json:"error,omitempty"`
}
