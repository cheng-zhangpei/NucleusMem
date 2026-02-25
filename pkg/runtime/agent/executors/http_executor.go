package tool_executors

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const defaultHTTPTimeout = 30 * time.Second

type HTTPExecutor struct {
	client *http.Client
}

func NewHTTPExecutor() *HTTPExecutor {
	return &HTTPExecutor{
		client: &http.Client{
			Timeout: defaultHTTPTimeout,
		},
	}
}

// NewHTTPExecutorWithClient allows injecting a custom http.Client (useful for testing)
func NewHTTPExecutorWithClient(client *http.Client) *HTTPExecutor {
	return &HTTPExecutor{client: client}
}

func (e *HTTPExecutor) Type() string {
	return "http"
}

func (e *HTTPExecutor) Execute(ctx context.Context, req *ToolRequest) (*ToolResponse, error) {
	if req.Endpoint == "" {
		return &ToolResponse{
			Success: false,
			Error:   "endpoint is empty",
		}, fmt.Errorf("HTTP executor: endpoint is empty for tool '%s'", req.ToolName)
	}

	// Default method is POST
	method := req.Method
	if method == "" {
		method = "POST"
	}

	// Override timeout if specified
	if req.TimeoutSec > 0 {
		e.client.Timeout = time.Duration(req.TimeoutSec) * time.Second
	}

	// Marshal parameters as JSON body
	var bodyReader io.Reader
	if req.Parameters != nil && method != "GET" {
		bodyBytes, err := json.Marshal(req.Parameters)
		if err != nil {
			return &ToolResponse{
				Success: false,
				Error:   fmt.Sprintf("failed to marshal parameters: %v", err),
			}, err
		}
		bodyReader = bytes.NewBuffer(bodyBytes)
	}

	// Build HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, method, req.Endpoint, bodyReader)
	if err != nil {
		return &ToolResponse{
			Success: false,
			Error:   fmt.Sprintf("failed to create request: %v", err),
		}, err
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	for k, v := range req.Headers {
		httpReq.Header.Set(k, v)
	}

	// Execute
	resp, err := e.client.Do(httpReq)
	if err != nil {
		return &ToolResponse{
			Success: false,
			Error:   fmt.Sprintf("HTTP request failed: %v", err),
		}, err
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return &ToolResponse{
			Success:    false,
			StatusCode: resp.StatusCode,
			Error:      fmt.Sprintf("failed to read response body: %v", err),
		}, err
	}

	// Parse response body as JSON
	var data map[string]interface{}
	if len(respBody) > 0 {
		if err := json.Unmarshal(respBody, &data); err != nil {
			// Not JSON, put raw body in data
			data = map[string]interface{}{
				"raw": string(respBody),
			}
		}
	}

	success := resp.StatusCode >= 200 && resp.StatusCode < 300
	result := &ToolResponse{
		Success:    success,
		Data:       data,
		StatusCode: resp.StatusCode,
		RawBody:    string(respBody),
	}

	if !success {
		result.Error = fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	return result, nil
}
