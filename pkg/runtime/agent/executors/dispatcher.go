package tool_executors

import (
	"context"
	"fmt"
	"sync"
)

// Dispatcher routes tool calls to the appropriate executor based on tool type
type Dispatcher struct {
	mu        sync.RWMutex
	executors map[string]Executor
}

func NewDispatcher() *Dispatcher {
	return &Dispatcher{
		executors: make(map[string]Executor),
	}
}

// Register adds an executor for a given type
// e.g., Register("http", httpExecutor)
func (d *Dispatcher) Register(execType string, exec Executor) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.executors[execType] = exec
}

// Dispatch selects the right executor and runs the tool
func (d *Dispatcher) Dispatch(ctx context.Context, execType string, req *ToolRequest) (*ToolResponse, error) {
	d.mu.RLock()
	exec, ok := d.executors[execType]
	d.mu.RUnlock()

	if !ok {
		return &ToolResponse{
			Success: false,
			Error:   fmt.Sprintf("no executor registered for type '%s'", execType),
		}, fmt.Errorf("unknown executor type: %s", execType)
	}

	return exec.Execute(ctx, req)
}

// SupportedTypes returns all registered executor types
func (d *Dispatcher) SupportedTypes() []string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	types := make([]string, 0, len(d.executors))
	for t := range d.executors {
		types = append(types, t)
	}
	return types
}
