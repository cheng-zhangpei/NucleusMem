package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"NucleusMem/pkg/api"
)

const defaultTimeout = 10 * time.Second

type AgentManagerClient struct {
	baseURL    string
	httpClient *http.Client
}

func NewAgentManagerClient(baseURL string) *AgentManagerClient {
	if !isHTTPURL(baseURL) {
		baseURL = "http://" + baseURL
	}

	return &AgentManagerClient{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: defaultTimeout},
	}
}

// ListAgents - GET /api/v1/agent_manager/listAgent
func (c *AgentManagerClient) ListAgents() ([]*api.AgentInfo, error) {
	return c.ListAgentsWithContext(context.Background())
}

func (c *AgentManagerClient) ListAgentsWithContext(ctx context.Context) ([]*api.AgentInfo, error) {
	reqURL := c.baseURL + "/api/v1/agent_manager/listAgent"
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("list agents: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("list agents failed with status %d: %s", resp.StatusCode, string(body))
	}

	var agents []*api.AgentInfo
	if err := json.NewDecoder(resp.Body).Decode(&agents); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return agents, nil
}

// LaunchAgent - POST /api/v1/agent_manager/launch
func (c *AgentManagerClient) LaunchAgent(req *api.LaunchAgentRequestHTTP) (*api.LaunchAgentResponseHTTP, error) {
	return c.LaunchAgentWithContext(context.Background(), req)
}

func (c *AgentManagerClient) LaunchAgentWithContext(ctx context.Context, req *api.LaunchAgentRequestHTTP) (*api.LaunchAgentResponseHTTP, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	reqURL := c.baseURL + "/api/v1/agent_manager/launch"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", reqURL, bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("launch agent: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var httpResponse api.LaunchAgentResponseHTTP
	if err := json.Unmarshal(respBody, &httpResponse); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := httpResponse.ErrorMessage
		if msg == "" {
			msg = string(respBody)
		}
		return nil, fmt.Errorf("launch failed: %s (status=%d)", msg, resp.StatusCode)
	}

	return &httpResponse, nil
}

// StopAgent - DELETE /api/v1/agent_manager/destroy?agent_id=123
func (c *AgentManagerClient) StopAgent(agentID uint64) error {
	return c.StopAgentWithContext(context.Background(), agentID)
}

func (c *AgentManagerClient) StopAgentWithContext(ctx context.Context, agentID uint64) error {
	// Build URL with query parameter
	baseURL := c.baseURL + "/api/v1/agent_manager/destroy"
	params := url.Values{}
	params.Add("agent_id", fmt.Sprintf("%d", agentID))
	reqURL := baseURL + "?" + params.Encode()

	req, err := http.NewRequestWithContext(ctx, "DELETE", reqURL, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("stop agent: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("stop failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// SendStatusUpdate - POST /api/v1/monitors/status
func (c *AgentManagerClient) SendStatusUpdate(update api.MonitorStatusUpdate) error {
	return c.SendStatusUpdateWithContext(context.Background(), update)
}

func (c *AgentManagerClient) SendStatusUpdateWithContext(ctx context.Context, update api.MonitorStatusUpdate) error {
	body, err := json.Marshal(update)
	if err != nil {
		return fmt.Errorf("marshal status update: %w", err)
	}
	reqURL := c.baseURL + "/api/v1/agent_manager/update"
	req, err := http.NewRequestWithContext(ctx, "POST", reqURL, bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send status update: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("status update failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

func isHTTPURL(url string) bool {
	return len(url) > 7 && (url[:7] == "http://" || url[:8] == "https://")
}
