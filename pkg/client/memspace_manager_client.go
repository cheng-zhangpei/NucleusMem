package client

import (
	"NucleusMem/pkg/api"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// MemSpaceManagerClient is the HTTP client for MemSpaceManager operations
type MemSpaceManagerClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewMemSpaceManagerClient creates a new MemSpaceManager client
func NewMemSpaceManagerClient(baseURL string) *MemSpaceManagerClient {
	if !strings.HasPrefix(baseURL, "http://") && !strings.HasPrefix(baseURL, "https://") {
		baseURL = "http://" + baseURL
	}
	return &MemSpaceManagerClient{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// post is a helper to make POST requests
func (c *MemSpaceManagerClient) post(endpoint string, req interface{}, resp interface{}) error {
	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	url := c.baseURL + endpoint
	httpResp, err := c.httpClient.Post(url, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("HTTP request failed: %w", err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		return fmt.Errorf("API returned status: %d", httpResp.StatusCode)
	}

	if err := json.NewDecoder(httpResp.Body).Decode(resp); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	return nil
}

// ListMemSpaces returns all memspace info from the manager
func (c *MemSpaceManagerClient) ListMemSpaces() ([]api.MemSpaceInfo, error) {
	req := map[string]interface{}{}
	var resp struct {
		Success   bool               `json:"success"`
		MemSpaces []api.MemSpaceInfo `json:"memspaces"`
		Error     string             `json:"error,omitempty"`
	}

	err := c.post("/api/v1/manager/list_memspaces", req, &resp)
	if err != nil {
		return nil, fmt.Errorf("list memspaces failed: %w", err)
	}

	if !resp.Success {
		return nil, fmt.Errorf("manager returned error: %s", resp.Error)
	}

	return resp.MemSpaces, nil
}

// BindMemSpace binds an agent to a memspace
func (c *MemSpaceManagerClient) BindMemSpace(agentID, memspaceID uint64) error {
	req := api.BindMemSpaceRequest{
		AgentID:    agentID,
		MemSpaceID: fmt.Sprintf("%d", memspaceID),
	}
	var resp api.BindMemSpaceResponse

	err := c.post("/api/v1/manager/bind_memspace", req, &resp)
	if err != nil {
		return fmt.Errorf("bind memspace failed: %w", err)
	}

	if !resp.Success {
		return fmt.Errorf("bind memspace failed: %s", resp.ErrorMessage)
	}

	return nil
}

// UnbindMemSpace unbinds an agent from a memspace
func (c *MemSpaceManagerClient) UnbindMemSpace(agentID, memspaceID uint64) error {
	req := api.UnbindMemSpaceRequest{
		AgentID:    agentID,
		MemSpaceID: fmt.Sprintf("%d", memspaceID),
	}
	var resp api.UnbindMemSpaceResponse

	err := c.post("/api/v1/manager/unbind_memspace", req, &resp)
	if err != nil {
		return fmt.Errorf("unbind memspace failed: %w", err)
	}

	if !resp.Success {
		return fmt.Errorf("unbind memspace failed: %s", resp.ErrorMessage)
	}

	return nil
}

// LaunchMemSpace launches a new memspace via manager
func (c *MemSpaceManagerClient) LaunchMemSpace(req *api.LaunchMemSpaceRequestManager) error {
	var resp api.LaunchMemSpaceResponse

	err := c.post("/api/v1/manager/launch_memspace", req, &resp)
	if err != nil {
		return fmt.Errorf("launch memspace failed: %w", err)
	}

	if !resp.Success {
		return fmt.Errorf("launch memspace failed: %s", resp.ErrorMessage)
	}

	return nil
}

// ShutdownMemSpace shuts down a memspace via manager
func (c *MemSpaceManagerClient) ShutdownMemSpace(memspaceID uint64) error {
	req := api.ShutdownMemSpaceRequest{
		MemSpaceID: memspaceID,
	}
	var resp api.ShutdownMemSpaceResponse

	err := c.post("/api/v1/manager/shutdown_memspace", req, &resp)
	if err != nil {
		return fmt.Errorf("shutdown memspace failed: %w", err)
	}

	if !resp.Success {
		return fmt.Errorf("shutdown memspace failed: %s", resp.ErrorMessage)
	}

	return nil
}

// NotifyMemSpaceUpdate notifies manager about memspace status changes (used by monitors)
func (c *MemSpaceManagerClient) NotifyMemSpaceUpdate(memspaces []api.MemSpaceInfo) error {
	req := api.NotifyMemSpacesRequest{
		MemSpaces: memspaces,
	}
	var resp api.NotifyMemSpacesResponse

	err := c.post("/api/v1/manager/notify_memspaces", req, &resp)
	if err != nil {
		return fmt.Errorf("notify memspace update failed: %w", err)
	}
	return nil
}
