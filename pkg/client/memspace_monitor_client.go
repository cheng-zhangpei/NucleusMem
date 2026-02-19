// Package client provides HTTP clients for NucleusMem services
package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"NucleusMem/pkg/api"
)

// MemSpaceMonitorClient is the HTTP client for MemSpaceMonitor operations
type MemSpaceMonitorClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewMemSpaceMonitorClient creates a new MemSpaceMonitor client
func NewMemSpaceMonitorClient(baseURL string) *MemSpaceMonitorClient {
	return &MemSpaceMonitorClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Helper to make POST requests
func (c *MemSpaceMonitorClient) post(endpoint string, req interface{}, resp interface{}) error {
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

// LaunchMemSpace launches a new MemSpace process
func (c *MemSpaceMonitorClient) LaunchMemSpace(binPath, configFilePath string, env map[string]string) (uint64, string, error) {
	req := api.LaunchMemSpaceRequest{
		BinPath:        binPath,
		ConfigFilePath: configFilePath,
		Env:            env,
	}
	var resp api.LaunchMemSpaceResponse
	err := c.post("/api/v1/monitor/launch_memspace", req, &resp)
	if err != nil {
		return 0, "", err
	}
	if !resp.Success {
		return 0, "", fmt.Errorf("launch memspace failed: %s", resp.ErrorMessage)
	}
	return resp.MemSpaceID, resp.Addr, nil
}

// StopMemSpace stops a running MemSpace
func (c *MemSpaceMonitorClient) StopMemSpace(memspaceID uint64) error {
	req := api.StopMemSpaceRequest{
		MemSpaceID: fmt.Sprintf("%d", memspaceID),
	}
	var resp api.StopMemSpaceResponse
	err := c.post("/api/v1/monitor/stop_memspace", req, &resp)
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("stop memspace failed: %s", resp.ErrorMessage)
	}
	return nil
}

// RegisterMemSpace registers an existing MemSpace
func (c *MemSpaceMonitorClient) RegisterMemSpace(memspaceID, name, ownerID, memspaceType, addr string) error {
	req := api.RegisterMemSpaceRequest{
		MemSpaceID: memspaceID,
		Name:       name,
		OwnerID:    ownerID,
		Type:       memspaceType,
		Addr:       addr,
	}
	var resp api.RegisterMemSpaceResponse
	err := c.post("/api/v1/monitor/register_memspace", req, &resp)
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("register memspace failed: %s", resp.ErrorMessage)
	}
	return nil
}

// ConnectMemSpace connects to a remote MemSpace
func (c *MemSpaceMonitorClient) ConnectMemSpace(memspaceID, addr string) error {
	req := api.ConnectMemSpaceRequest{
		MemSpaceID: memspaceID,
		Addr:       addr,
	}
	var resp api.ConnectMemSpaceResponse
	err := c.post("/api/v1/monitor/connect_memspace", req, &resp)
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("connect memspace failed: %s", resp.ErrorMessage)
	}
	return nil
}

// UnregisterMemSpace removes a MemSpace from the monitor
func (c *MemSpaceMonitorClient) UnregisterMemSpace(memspaceID uint64) error {
	req := api.UnregisterMemSpaceRequest{
		MemSpaceID: fmt.Sprintf("%d", memspaceID),
	}
	var resp api.UnregisterMemSpaceResponse
	err := c.post("/api/v1/monitor/unregister_memspace", req, &resp)
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("unregister memspace failed: %s", resp.ErrorMessage)
	}
	return nil
}

// ListMemSpaces returns all registered MemSpaces
func (c *MemSpaceMonitorClient) ListMemSpaces() ([]api.MemSpaceInfo, error) {
	req := map[string]interface{}{}
	var resp api.ListMemSpacesResponse
	err := c.post("/api/v1/monitor/list_memspaces", req, &resp)
	if err != nil {
		return nil, err
	}
	if !resp.Success {
		return nil, fmt.Errorf("list memspaces failed")
	}
	return resp.MemSpaces, nil
}

// HealthCheck performs a health check
func (c *MemSpaceMonitorClient) HealthCheck() error {
	req := map[string]interface{}{}
	var resp map[string]string
	err := c.post("/health", req, &resp)
	if err != nil {
		return err
	}
	if status, ok := resp["status"]; !ok || status != "healthy" {
		return fmt.Errorf("memspace monitor unhealthy: %v", resp)
	}
	return nil
}
