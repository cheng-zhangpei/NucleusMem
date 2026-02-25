package tool_executors

import (
	"context"
	"fmt"
)

// pkg/executor/shell_executor.go

// ShellExecutor executes tools as local shell commands
// TODO(cheng): implement after demo phase
type ShellExecutor struct{}

func NewShellExecutor() *ShellExecutor {
	return &ShellExecutor{}
}

func (e *ShellExecutor) Type() string {
	return "shell"
}

func (e *ShellExecutor) Execute(ctx context.Context, req *ToolRequest) (*ToolResponse, error) {
	return &ToolResponse{
		Success: false,
		Error:   "shell executor not implemented yet",
	}, fmt.Errorf("shell executor not implemented")
}
