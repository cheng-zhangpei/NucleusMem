package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"NucleusMem/pkg/api"
)

// MemSpaceManagerClient is the HTTP client for MemSpaceManager operations
type MemSpaceManagerClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewMemSpaceManagerClient creates a new MemSpaceManager client
func NewMemSpaceManagerClient(baseURL string) *MemSpaceManagerClient {
	return &MemSpaceManagerClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Helper to make POST requests
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

// NotifyMemSpaceUpdate sends memspace updates to the manager
func (c *MemSpaceManagerClient) NotifyMemSpaceUpdate(memspaces []api.MemSpaceInfo) error {
	req := api.NotifyMemSpacesRequest{MemSpaces: memspaces}
	var resp api.NotifyMemSpacesResponse
	err := c.post("/api/v1/manager/notify_memspaces", req, &resp)
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("notify failed")
	}
	return nil
}
