// pkg/agent/standard_executors.go

package agent

import (
	"NucleusMem/pkg/configs"
	"NucleusMem/pkg/runtime/agent/standard_executor"
	"context"
	//"fmt"
)

// StandardToolExecutor 是标准工具执行器的统一接口
type StandardToolExecutor interface {
	Execute(ctx context.Context, tool *configs.StandardToolDefinition, params map[string]interface{}) (map[string]interface{}, error)
}

// StandardExecutorRegistry 持有所有类型的执行器实例
type StandardExecutorRegistry struct {
	HTTP     StandardToolExecutor
	Shell    StandardToolExecutor
	Delegate StandardToolExecutor
	MCP      StandardToolExecutor
}

// NewStandardExecutorRegistry initializes the registry with concrete implementations
func NewStandardExecutorRegistry() *StandardExecutorRegistry {
	return &StandardExecutorRegistry{
		HTTP:     standard_executor.NewHTTPStandardExecutor(),
		Shell:    standard_executor.NewShellStandardExecutor(),
		Delegate: standard_executor.NewDelegateStandardExecutor(),
		MCP:      standard_executor.NewMCPStandardExecutor(),
	}
}
