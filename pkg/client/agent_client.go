// Package client provides HTTP clients for NucleusMem services
package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"NucleusMem/pkg/api"
)

type AgentClient struct {
	baseURL    string
	httpClient *http.Client
}

func NewAgentClient(baseURL string) *AgentClient {
	return &AgentClient{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// TempChat sends a temporary chat message to the agent (in-memory context only)
func (c *AgentClient) TempChat(message string) (*api.TempChatResponse, error) {
	req := api.TempChatRequest{Message: message}
	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := c.httpClient.Post(c.baseURL+"/api/v1/agent/chat/temp", "application/json", bytes.NewBuffer(jsonData))
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

	resp, err := c.httpClient.Post(c.baseURL+"/api/v1/agent/chat", "application/json", bytes.NewBuffer(jsonData))
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
	resp, err := c.httpClient.Post(c.baseURL+"/api/v1/agent/health", "application/json", bytes.NewBuffer(jsonData))

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
	resp, err := c.httpClient.Post(c.baseURL+"/api/v1/agent/shutdown", "application/json", nil)
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
func (c *AgentClient) BaseURL() string {
	return c.baseURL
}
