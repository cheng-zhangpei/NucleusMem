// Package client 提供与 AgentMonitor 交互的 HTTP 客户端
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"NucleusMem/pkg/api"
)

// AgentMonitorClient 是 AgentMonitor 的 HTTP 客户端
type AgentMonitorClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewAgentMonitorClient 创建新客户端
func NewAgentMonitorClient(baseURL string, opts ...ClientOption) *AgentMonitorClient {
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	c := &AgentMonitorClient{
		baseURL:    baseURL, // e.g., "http://192.168.1.10:8080"
		httpClient: client,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// ClientOption 允许自定义客户端行为
type ClientOption func(*AgentMonitorClient)

// WithHTTPClient 允许传入自定义 http.Client（用于测试或高级配置）
func WithHTTPClient(cli *http.Client) ClientOption {
	return func(c *AgentMonitorClient) {
		c.httpClient = cli
	}
}

// WithTimeout 设置超时
func WithTimeout(timeout time.Duration) ClientOption {
	return func(c *AgentMonitorClient) {
		c.httpClient.Timeout = timeout
	}
}

// LaunchAgent 启动一个 Agent
func (c *AgentMonitorClient) LaunchAgent(ctx context.Context, req *api.LaunchAgentRequestHTTP) (*api.LaunchAgentResponseHTTP, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	url := c.baseURL + "/api/v1/monitor/launch"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	var httpResponse api.LaunchAgentResponseHTTP
	if err := json.Unmarshal(respBody, &httpResponse); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	// 如果 HTTP 状态码不是 2xx，但响应体有 success=false，我们也视为业务错误
	if resp.StatusCode >= 400 {
		msg := httpResponse.ErrorMessage
		if msg == "" {
			msg = string(respBody)
		}
		return nil, fmt.Errorf("launch agent failed: %s (status=%d)", msg, resp.StatusCode)
	}

	return &httpResponse, nil
}

// StopAgent 停止一个 Agent
func (c *AgentMonitorClient) StopAgent(ctx context.Context, agentID uint64) (*api.StopAgentResponseHTTP, error) {
	url := fmt.Sprintf("%s/api/v1/monitor/destroy", c.baseURL)
	// 发送 JSON body 而不是 query 参数
	reqBody := map[string]uint64{"agent_id": agentID}
	jsonData, _ := json.Marshal(reqBody)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	var httpResponse api.StopAgentResponseHTTP
	if err := json.Unmarshal(respBody, &httpResponse); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	if resp.StatusCode >= 400 {
		msg := httpResponse.ErrorMessage
		if msg == "" {
			msg = string(respBody)
		}
		return nil, fmt.Errorf("stop agent failed: %s (status=%d)", msg, resp.StatusCode)
	}

	return &httpResponse, nil
}

// GetStatus 获取 Monitor 节点状态
func (c *AgentMonitorClient) GetStatus(ctx context.Context) (*api.MonitorHeartbeatHTTP, error) {
	url := c.baseURL + "/api/v1/monitor/status"
	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("get status failed: %s (status=%d)", string(body), resp.StatusCode)
	}

	var status api.MonitorHeartbeatHTTP
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return nil, fmt.Errorf("decode status response: %w", err)
	}

	return &status, nil
}

// ConnectAgent connects to an external agent via Monitor's HTTP API
func (c *AgentMonitorClient) ConnectAgent(ctx context.Context, agentID uint64, addr string) error {
	req := api.ConnectAgentRequest{
		AgentID: agentID,
		Addr:    addr,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal connect request: %w", err)
	}

	url := c.baseURL + "/api/v1/agents/connect"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("connect agent: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	var httpResponse api.ConnectAgentResponse
	if err := json.Unmarshal(respBody, &httpResponse); err != nil {
		return fmt.Errorf("unmarshal response: %w", err)
	}

	if !httpResponse.Success {
		msg := httpResponse.ErrorMessage
		if msg == "" {
			msg = string(respBody)
		}
		return fmt.Errorf("connect agent failed: %s (status=%d)", msg, resp.StatusCode)
	}

	return nil
}
