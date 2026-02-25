// Package client provides HTTP clients for NucleusMem services
package client

import (
	"NucleusMem/pkg/api"
	"NucleusMem/pkg/configs"
	_ "NucleusMem/pkg/configs"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// MemSpaceClient is the HTTP client for MemSpace operations
type MemSpaceClient struct {
	BaseURL    string
	httpClient *http.Client
}

// NewMemSpaceClient creates a new MemSpace client
func NewMemSpaceClient(baseURL string) *MemSpaceClient {
	if !strings.HasPrefix(baseURL, "http://") && !strings.HasPrefix(baseURL, "https://") {
		baseURL = "http://" + baseURL
	}
	return &MemSpaceClient{
		BaseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Helper to make POST requests
func (c *MemSpaceClient) post(endpoint string, req interface{}, resp interface{}) error {
	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	url := c.BaseURL + endpoint
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

// WriteMemory writes a memory entry to MemSpace
func (c *MemSpaceClient) WriteMemory(content string, agentID uint64) error {
	req := api.WriteMemoryRequest{
		Content: content,
		AgentID: fmt.Sprintf("%d", agentID),
	}
	var resp api.WriteMemoryResponse
	err := c.post("/api/v1/memspace/write_memory", req, &resp)
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("write memory failed: %s", resp.ErrorMessage)
	}
	return nil
}

// GetMemoryContext retrieves combined context from Summary and Memory regions
func (c *MemSpaceClient) GetMemoryContext(summaryBefore int64, query string, n int) (string, []string, error) {
	req := api.GetMemoryContextRequest{
		SummaryBefore: summaryBefore,
		Query:         query,
		N:             n,
	}
	var resp api.GetMemoryContextResponse
	err := c.post("/api/v1/memspace/get_memory_context", req, &resp)
	if err != nil {
		return "", nil, err
	}
	if !resp.Success {
		return "", nil, fmt.Errorf("get memory context failed: %s", resp.ErrorMessage)
	}
	return resp.Summary, resp.Memories, nil
}

// RegisterAgent registers an agent in the Comm Region
func (c *MemSpaceClient) RegisterAgent(agentID uint64, addr, role string) error {
	req := api.RegisterAgentRequest{
		AgentID: fmt.Sprintf("%d", agentID),
		Addr:    addr,
		Role:    role,
	}
	var resp api.RegisterAgentResponse
	err := c.post("/api/v1/memspace/register_agent", req, &resp)
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("register agent failed: %s", resp.ErrorMessage)
	}
	return nil
}

// UnregisterAgent removes an agent from the registry
func (c *MemSpaceClient) UnregisterAgent(agentID uint64) error {
	req := api.UnregisterAgentRequest{
		AgentID: fmt.Sprintf("%d", agentID),
	}
	var resp api.UnregisterAgentResponse
	err := c.post("/api/v1/memspace/unregister_agent", req, &resp)
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("unregister agent failed: %s", resp.ErrorMessage)
	}
	return nil
}

// SendMessage sends a collaboration message
func (c *MemSpaceClient) SendMessage(fromAgent, toAgent uint64, key, content string) (string, error) {
	req := api.SendMessageRequest{
		FromAgent: fmt.Sprintf("%d", fromAgent),
		ToAgent:   fmt.Sprintf("%d", toAgent),
		Key:       key,
		Content:   content,
	}
	var resp api.SendMessageResponse
	err := c.post("/api/v1/memspace/send_message", req, &resp)
	if err != nil {
		return "", err
	}
	if !resp.Success {
		return "", fmt.Errorf("send message failed: %s", resp.ErrorMessage)
	}
	return resp.Response, nil
}

// ListAgents returns all registered agents
func (c *MemSpaceClient) ListAgents() ([]api.AgentRegistryEntry, error) {
	// Empty request body
	req := map[string]interface{}{}
	var resp api.ListAgentsResponse
	err := c.post("/api/v1/memspace/list_agents", req, &resp)
	if err != nil {
		return nil, err
	}
	if !resp.Success {
		return nil, fmt.Errorf("list agents failed")
	}
	return resp.Agents, nil
}

// HealthCheck performs a health check
func (c *MemSpaceClient) HealthCheckWithInfo() (*api.MemSpaceHealthResponse, error) {
	req := map[string]interface{}{}
	var resp api.MemSpaceHealthResponse
	err := c.post("/api/v1/memspace/health", req, &resp)
	if err != nil {
		return nil, err
	}
	if resp.Status != "healthy" {
		return nil, fmt.Errorf("memspace unhealthy")
	}
	return &resp, nil
}
func (c *MemSpaceClient) Shutdown() error {
	req := map[string]interface{}{}
	var resp map[string]bool
	err := c.post("/api/v1/memspace/shutdown", req, &resp)
	if err != nil {
		return fmt.Errorf("shutdown request failed: %w", err)
	}
	if !resp["success"] {
		return fmt.Errorf("shutdown failed")
	}
	return nil
}

// BindAgent binds an agent to this MemSpace
func (c *MemSpaceClient) BindAgent(agentID uint64, addr, role string) error {
	req := api.BindAgentRequest{
		AgentID: fmt.Sprintf("%d", agentID),
		Addr:    addr,
		Role:    role,
	}
	var resp api.BindAgentResponse

	err := c.post("/api/v1/memspace/bind_agent", req, &resp)
	if err != nil {
		return fmt.Errorf("bind agent failed: %w", err)
	}

	if !resp.Success {
		return fmt.Errorf("bind agent failed: %s", resp.ErrorMessage)
	}
	return nil
}

// UnbindAgent unbinds an agent from this MemSpace
func (c *MemSpaceClient) UnbindAgent(agentID uint64) error {
	req := api.UnbindAgentRequest{
		AgentID: fmt.Sprintf("%d", agentID),
	}
	var resp api.UnbindAgentResponse
	err := c.post("/api/v1/memspace/unbind_agent", req, &resp)
	if err != nil {
		return fmt.Errorf("unbind agent failed: %w", err)
	}
	if !resp.Success {
		return fmt.Errorf("unbind agent failed: %s", resp.ErrorMessage)
	}
	return nil
}

// GetByKey fetches raw value by raw key (returns raw bytes)
func (c *MemSpaceClient) GetMemoryByKey(rawKey []byte) (string, error) {
	req := map[string]string{
		"raw_key": string(rawKey),
	}
	var resp struct {
		Success bool   `json:"success"`
		Value   string `json:"value"`
		Error   string `json:"error"`
	}
	err := c.post("/api/v1/memspace/get_by_key", req, &resp)
	if err != nil {
		return "", err
	}
	if !resp.Success {
		return "", fmt.Errorf("get by key failed: %s", resp.Error)
	}
	return resp.Value, nil

}

func (c *MemSpaceClient) GetTool(name string) (*configs.ToolDefinition, error) {
	req := struct {
		Name string `json:"name"`
	}{
		Name: name,
	}
	var resp struct {
		Success bool                    `json:"success"`
		Tool    *configs.ToolDefinition `json:"tool,omitempty"`
		Error   string                  `json:"error,omitempty"`
	}

	err := c.post("/api/v1/memspace/tool/get", req, &resp)
	if err != nil {
		return nil, err
	}
	if !resp.Success {
		return nil, fmt.Errorf("get tool failed: %s", resp.Error)
	}
	return resp.Tool, nil
}

// ListTools returns all tool definitions in this MemSpace
func (c *MemSpaceClient) ListTools() ([]*configs.ToolDefinition, error) {
	req := struct{}{} // Empty request body

	var resp struct {
		Success bool                      `json:"success"`
		Tools   []*configs.ToolDefinition `json:"tools,omitempty"`
		Error   string                    `json:"error,omitempty"`
	}

	err := c.post("/api/v1/memspace/tools/list", req, &resp)
	if err != nil {
		return nil, err
	}
	if !resp.Success {
		return nil, fmt.Errorf("list tools failed: %s", resp.Error)
	}
	return resp.Tools, nil
}

// RegisterTool registers a new tool in this MemSpace
func (c *MemSpaceClient) RegisterTool(tool *configs.ToolDefinition) error {
	var resp struct {
		Success bool   `json:"success"`
		Error   string `json:"error,omitempty"`
	}

	err := c.post("/api/v1/memspace/tool/register", tool, &resp)
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("register tool failed: %s", resp.Error)
	}
	return nil
}

// DeleteTool removes a tool from this MemSpace
func (c *MemSpaceClient) DeleteTool(name string) error {
	req := struct {
		Name string `json:"name"`
	}{
		Name: name,
	}

	var resp struct {
		Success bool   `json:"success"`
		Error   string `json:"error,omitempty"`
	}

	err := c.post("/api/v1/memspace/tool/delete", req, &resp)
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("delete tool failed: %s", resp.Error)
	}
	return nil
}

// FindToolsByTags finds tools matching the given tags
func (c *MemSpaceClient) FindToolsByTags(tags []string) ([]*configs.ToolDefinition, error) {
	req := struct {
		Tags []string `json:"tags"`
	}{
		Tags: tags,
	}

	var resp struct {
		Success bool                      `json:"success"`
		Tools   []*configs.ToolDefinition `json:"tools,omitempty"`
		Error   string                    `json:"error,omitempty"`
	}

	err := c.post("/api/v1/memspace/tool/find_by_tags", req, &resp)
	if err != nil {
		return nil, err
	}
	if !resp.Success {
		return nil, fmt.Errorf("find tools failed: %s", resp.Error)
	}
	return resp.Tools, nil
}
