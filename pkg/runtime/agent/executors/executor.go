// pkg/executor/executor.go

package tool_executors

import (
	"context"
)

// ToolRequest is the unified input for all executors
type ToolRequest struct {
	ToolName   string
	Endpoint   string
	Method     string // for HTTP: "GET", "POST", etc. Default "POST"
	Parameters map[string]interface{}
	Headers    map[string]string // optional, for HTTP
	TimeoutSec int               // 0 means use default
}

// ToolResponse is the unified output from all executors
type ToolResponse struct {
	Success bool
	Data    map[string]interface{}
	Error   string
	// Raw response for debugging
	StatusCode int    // HTTP status code, or exit code for shell
	RawBody    string // raw response body
}

// Executor is the common interface for all tool execution backends
type Executor interface {
	// Type returns the executor type identifier
	Type() string
	// Execute runs the tool and returns the result
	Execute(ctx context.Context, req *ToolRequest) (*ToolResponse, error)
}
