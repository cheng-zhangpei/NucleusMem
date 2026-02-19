package api

// LaunchMemSpace
type LaunchMemSpaceRequest struct {
	BinPath        string            `json:"bin_path"`
	ConfigFilePath string            `json:"config_file_path"`
	Env            map[string]string `json:"env,omitempty"`
}
type LaunchMemSpaceResponse struct {
	Success      bool   `json:"success"`
	MemSpaceID   uint64 `json:"memspace_id,omitempty"`
	Addr         string `json:"addr,omitempty"`
	ErrorMessage string `json:"error_message,omitempty"`
}

// StopMemSpace
type StopMemSpaceRequest struct {
	MemSpaceID string `json:"memspace_id"`
}
type StopMemSpaceResponse struct {
	Success      bool   `json:"success"`
	ErrorMessage string `json:"error_message,omitempty"`
}

// RegisterMemSpace
type RegisterMemSpaceRequest struct {
	MemSpaceID string `json:"memspace_id"`
	Name       string `json:"name"`
	OwnerID    string `json:"owner_id"`
	Type       string `json:"type"`
	Addr       string `json:"addr"`
}
type RegisterMemSpaceResponse struct {
	Success      bool   `json:"success"`
	ErrorMessage string `json:"error_message,omitempty"`
}

// ConnectMemSpace
type ConnectMemSpaceRequest struct {
	MemSpaceID string `json:"memspace_id"`
	Addr       string `json:"addr"`
}
type ConnectMemSpaceResponse struct {
	Success      bool   `json:"success"`
	ErrorMessage string `json:"error_message,omitempty"`
}

// UnregisterMemSpace
type UnregisterMemSpaceRequest struct {
	MemSpaceID string `json:"memspace_id"`
}
type UnregisterMemSpaceResponse struct {
	Success      bool   `json:"success"`
	ErrorMessage string `json:"error_message,omitempty"`
}

// ListMemSpaces
type MemSpaceInfo struct {
	MemSpaceID  string `json:"memspace_id"`
	Name        string `json:"name"`
	OwnerID     string `json:"owner_id"`
	Type        string `json:"type"`
	Status      string `json:"status"`
	HttpAddr    string `json:"http_addr"`
	Description string `json:"description"`
	LastSeen    int64  `json:"last_seen"`
}
type ListMemSpacesResponse struct {
	Success   bool           `json:"success"`
	MemSpaces []MemSpaceInfo `json:"memspaces"`
}
