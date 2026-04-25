package standard_executor

import (
	"NucleusMem/pkg/configs"
	"context"
	"fmt"
)

type MCPStandardExecutor struct{}

func NewMCPStandardExecutor() *MCPStandardExecutor {
	return &MCPStandardExecutor{}
}
func (e *MCPStandardExecutor) Execute(ctx context.Context, tool *configs.StandardToolDefinition, params map[string]interface{}) (map[string]interface{}, error) {
	// TODO: 实现 MCP 协议调用
	return nil, fmt.Errorf("MCP executor not yet implemented")
}
