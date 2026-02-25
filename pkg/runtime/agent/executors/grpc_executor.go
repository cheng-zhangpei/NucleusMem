package tool_executors

// pkg/executor/grpc_executor.go

import (
	"context"
	"fmt"
)

// GRPCExecutor executes tools via gRPC calls
// TODO: implement after demo phase
type GRPCExecutor struct{}

func NewGRPCExecutor() *GRPCExecutor {
	return &GRPCExecutor{}
}

func (e *GRPCExecutor) Type() string {
	return "grpc"
}

func (e *GRPCExecutor) Execute(ctx context.Context, req *ToolRequest) (*ToolResponse, error) {
	return &ToolResponse{
		Success: false,
		Error:   "gRPC executor not implemented yet",
	}, fmt.Errorf("gRPC executor not implemented")
}
