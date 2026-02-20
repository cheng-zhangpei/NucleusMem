package api

// NotifyMemSpaces
type NotifyMemSpacesRequest struct {
	MemSpaces []MemSpaceInfo `json:"memspaces"`
}

type NotifyMemSpacesResponse struct {
	Success bool `json:"success"`
}

// ListMemSpaces
type ListMemSpacesResponseManager struct {
	Success   bool           `json:"success"`
	MemSpaces []MemSpaceInfo `json:"memspaces"`
}

// BindMemSpace
type BindMemSpaceRequestMemManager struct {
	AgentID    string `json:"agent_id"`
	MemSpaceID string `json:"memspace_id"`
}
type BindMemSpaceResponse struct {
	Success      bool   `json:"success"`
	ErrorMessage string `json:"error_message,omitempty"`
}

// UnbindMemSpace
type UnbindMemSpaceRequestMemManager struct {
	AgentID    string `json:"agent_id"`
	MemSpaceID string `json:"memspace_id"`
}
type UnbindMemSpaceResponse struct {
	Success      bool   `json:"success"`
	ErrorMessage string `json:"error_message,omitempty"`
}

// LaunchMemSpace
type LaunchMemSpaceRequestManager struct {
	MemSpaceID          uint64 `json:"memspace_id"`
	Name                string `json:"name"`
	Type                string `json:"type"`
	OwnerID             uint64 `json:"owner_id"`
	Description         string `json:"description"`
	HttpAddr            string `json:"http_addr"`
	PdAddr              string `json:"pd_addr"`
	EmbeddingClientAddr string `json:"embedding_client_addr"`
	LightModelAddr      string `json:"light_model_addr"`
	SummaryCnt          uint64 `json:"summary_cnt"`
	SummaryThreshold    uint64 `json:"summary_threshold"`
	BinPath             string `json:"bin_path"`
	ConfigFilePath      string `json:"config_file_path"`
}
type LaunchMemSpaceResponseManager struct {
	Success      bool   `json:"success"`
	ErrorMessage string `json:"error_message,omitempty"`
}

// ShutdownMemSpace
type ShutdownMemSpaceRequest struct {
	MemSpaceID uint64 `json:"memspace_id"`
}
type ShutdownMemSpaceResponse struct {
	Success      bool   `json:"success"`
	ErrorMessage string `json:"error_message,omitempty"`
}

// NotifyMemSpaces (already defined)
