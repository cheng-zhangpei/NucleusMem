// Package client provides HTTP clients for NucleusMem services
package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"NucleusMem/pkg/api"
)

type AgentClient struct {
	BaseURL    string
	HttpClient *http.Client
}

func NewAgentClient(baseURL string) *AgentClient {
	if !strings.HasPrefix(baseURL, "http://") && !strings.HasPrefix(baseURL, "https://") {
		baseURL = "http://" + baseURL
	}
	return &AgentClient{
		BaseURL:    baseURL,
		HttpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// Helper to make POST requests
func (c *AgentClient) post(endpoint string, req interface{}, resp interface{}) error {
	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	url, err := url.JoinPath(c.BaseURL, endpoint)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	httpResp, err := c.HttpClient.Post(url, "application/json", bytes.NewBuffer(body))
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

// TempChat sends a temporary chat message to the agent (in-memory context only)
func (c *AgentClient) TempChat(message string) (*api.TempChatResponse, error) {
	req := api.TempChatRequest{Message: message}
	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := c.HttpClient.Post(c.BaseURL+"/api/v1/agent/chat/temp", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("temp chat failed with status %d: %s", resp.StatusCode, string(body))
	}

	var result api.TempChatResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &result, nil
}

// Chat sends a persistent chat message to the agent (with MemSpace integration - future)
func (c *AgentClient) Chat(message string) (*api.ChatResponse, error) {
	req := api.ChatRequest{Message: message}
	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := c.HttpClient.Post(c.BaseURL+"/api/v1/agent/chat", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("chat failed with status %d: %s", resp.StatusCode, string(body))
	}

	var result api.ChatResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &result, nil
}

// HealthCheckWithMonitor performs health check and binds to specified monitor
func (c *AgentClient) HealthCheckWithMonitor(monitorID uint64) (*api.AgentHealthResponse, error) {
	req := api.AgentHealthRequest{MonitorID: monitorID}

	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal health request: %w", err)
	}
	resp, err := c.HttpClient.Post(c.BaseURL+"/api/v1/agent/health", "application/json", bytes.NewBuffer(jsonData))

	if err != nil {
		return nil, fmt.Errorf("health check request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read health response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("health check failed with status %d: %s", resp.StatusCode, string(body))
	}

	var result api.AgentHealthResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse health response: %w", err)
	}

	return &result, nil
}

// HealthCheck Keep original HealthCheck for backward compatibility
func (c *AgentClient) HealthCheck() (*api.AgentHealthResponse, error) {
	return c.HealthCheckWithMonitor(0)
}

// pkg/client/agent_client.go
func (c *AgentClient) Shutdown() error {
	resp, err := c.HttpClient.Post(c.BaseURL+"/api/v1/agent/shutdown", "application/json", nil)
	if err != nil {
		return fmt.Errorf("shutdown request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("shutdown failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

func (c *AgentClient) Notify(key, content string) (string, error) {
	req := &api.NotifyRequest{
		Key:     key,
		Content: content,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("failed to marshal notify request: %w", err)
	}

	resp, err := c.HttpClient.Post(c.BaseURL+"/api/v1/agent/notify", "application/json", bytes.NewBuffer(body))
	if err != nil {
		return "", fmt.Errorf("failed to send notify request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("notify request failed with status: %d", resp.StatusCode)
	}

	var notifyResp api.NotifyResponse
	if err := json.NewDecoder(resp.Body).Decode(&notifyResp); err != nil {
		return "", fmt.Errorf("failed to decode notify response: %w", err)
	}

	if !notifyResp.Success {
		msg := notifyResp.ErrorMessage
		if msg == "" {
			msg = "unknown error"
		}
		return "", fmt.Errorf("notify failed: %s", msg)
	}

	return notifyResp.Result, nil
}

// BindMemSpace binds the agent to a MemSpace (local only)
func (c *AgentClient) BindMemSpace(req *api.BindMemSpaceRequest) error {
	var resp map[string]interface{}
	err := c.post("/api/v1/agent/bind_memspace", req, &resp)
	if err != nil {
		//log.Errorf("err occurred posting bind_memspace request: %v", err)
		return fmt.Errorf("bind memspace failed: %v", err.Error())
	}
	if success, ok := resp["success"].(bool); !ok || !success {
		msg := "unknown error"
		if e, ok := resp["error"].(string); ok {
			msg = e
		}
		return fmt.Errorf("bind memspace failed: %s", msg)
	}
	return nil
}

// UnbindMemSpace unbinds the agent from a MemSpace
func (c *AgentClient) UnbindMemSpace(memspaceID uint64) error {
	req := &api.UnbindMemSpaceRequest{
		MemSpaceID: fmt.Sprintf("%d", memspaceID),
	}
	var resp map[string]interface{}
	err := c.post("/api/v1/agent/unbind_memspace", req, &resp)
	if err != nil {
		return fmt.Errorf("unbind memspace failed: %w", err)
	}
	if success, ok := resp["success"].(bool); !ok || !success {
		msg := "unknown error"
		if e, ok := resp["error"].(string); ok {
			msg = e
		}
		return fmt.Errorf("unbind memspace failed: %s", msg)
	}
	return nil
}

// Communicate sends a message to another agent via the Agent HTTP API
func (c *AgentClient) Communicate(targetAgentID uint64, key, content string) (string, error) {
	req := api.CommunicateRequest{
		TargetAgentID: fmt.Sprintf("%d", targetAgentID),
		Key:           key,
		Content:       content,
	}
	var resp api.CommunicateResponse

	err := c.post("/api/v1/agent/communicate", req, &resp)
	if err != nil {
		return "", fmt.Errorf("communicate failed: %w", err)
	}

	if !resp.Success {
		return "", fmt.Errorf("communicate failed: %s", resp.ErrorMessage)
	}

	return resp.Result, nil
}

// SubmitTask submits a task to the agent's task queue and returns a taskID
func (c *AgentClient) SubmitTask(req *api.SubmitTaskRequest) (*api.SubmitTaskResponse, error) {
	var resp api.SubmitTaskResponse
	err := c.post("/api/v1/agent/submit_task", req, &resp)
	if err != nil {
		return nil, fmt.Errorf("submit task failed: %w", err)
	}
	if !resp.Success {
		return nil, fmt.Errorf("submit task failed: %s", resp.ErrorMessage)
	}
	return &resp, nil
}

// GetTaskResult polls for a task result by taskID
func (c *AgentClient) GetTaskResult(taskID string, timeout time.Duration) (*api.GetTaskResultResponse, error) {
	// Set a longer timeout for this call since we're waiting for LLM
	originalTimeout := c.HttpClient.Timeout
	c.HttpClient.Timeout = timeout + 5*time.Second
	defer func() { c.HttpClient.Timeout = originalTimeout }()

	req := &api.GetTaskResultRequest{
		TaskID:    taskID,
		TimeoutMs: int64(timeout / time.Millisecond),
	}
	var resp api.GetTaskResultResponse
	err := c.post("/api/v1/agent/task_result", req, &resp)
	if err != nil {
		return nil, fmt.Errorf("get task result failed: %w", err)
	}
	if !resp.Success {
		return nil, fmt.Errorf("get task result failed: %s", resp.ErrorMessage)
	}
	return &resp, nil
}
