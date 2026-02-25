package tool_executors

// pkg/executor/mcp_executor.go

import (
	"context"
	"fmt"
)

// MCPExecutor executes tools via Model Context Protocol
// TODO: implement after demo phase
type MCPExecutor struct{}

func NewMCPExecutor() *MCPExecutor {
	return &MCPExecutor{}
}

func (e *MCPExecutor) Type() string {
	return "mcp"
}

func (e *MCPExecutor) Execute(ctx context.Context, req *ToolRequest) (*ToolResponse, error) {
	return &ToolResponse{
		Success: false,
		Error:   "MCP executor not implemented yet",
	}, fmt.Errorf("MCP executor not implemented")
}
