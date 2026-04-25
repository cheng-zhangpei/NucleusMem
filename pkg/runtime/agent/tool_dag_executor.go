package agent

import (
	"NucleusMem/pkg/configs"
	"context"
	"fmt"
	"sync"

	"github.com/pingcap-incubator/tinykv/log"
)

// ToolDAGExecutor handles concurrent execution of tools based on dependency graph
// It implements a topological sort scheduler similar to ViewSpaceTree.executeComposite
// only the represented Agent in atomic ViewSpace will do execute function

type ToolDAGExecutor struct {
	agent      *Agent
	dag        *configs.ToolDAG
	memSpaceID uint64
	taskID     string
}

// ToolExecResult holds the result of a single tool execution

// NewToolDAGExecutor creates a new executor instance
func NewToolDAGExecutor(agent *Agent, dag *configs.ToolDAG, memSpaceID uint64, taskID string) *ToolDAGExecutor {
	return &ToolDAGExecutor{
		agent:      agent,
		dag:        dag,
		memSpaceID: memSpaceID,
		taskID:     taskID,
	}
}

// Execute runs the entire tool DAG and returns results for all nodes
func (e *ToolDAGExecutor) Execute(ctx context.Context) (map[string]*configs.ToolExecResult, error) {
	log.Infof("[ToolDAG] Starting execution for task %s, %d nodes, %d edges",
		e.taskID, len(e.dag.Nodes), len(e.dag.Edges))
	// Build dependency map: toolName -> [dependencies]
	deps := e.buildDependencyMap()
	// Execution context tracks completion status
	execCtx := newToolExecContext(len(e.dag.Nodes))
	// Channel to collect results from goroutines
	type toolResult struct {
		name   string
		result *configs.ToolExecResult
	}
	resultCh := make(chan toolResult, len(e.dag.Nodes))
	// Track which tools have been started to avoid duplicate execution
	started := make(map[string]bool)
	var startMu sync.Mutex
	// tryStartReady checks which tools are ready and launches them
	var tryStartReady func()
	tryStartReady = func() {
		startMu.Lock()
		defer startMu.Unlock()

		for _, node := range e.dag.Nodes {
			if started[node.ToolName] {
				continue
			}

			// Check if all dependencies are satisfied
			allDepsDone := true
			for _, depName := range deps[node.ToolName] {
				execCtx.mu.Lock()
				_, done := execCtx.results[depName]
				execCtx.mu.Unlock()
				if !done {
					allDepsDone = false
					break
				}
			}

			if allDepsDone {
				started[node.ToolName] = true
				log.Infof("[ToolDAG] Starting tool '%s'", node.ToolName)

				// Execute tool in a separate goroutine
				go func(n configs.ToolDAGNode) {
					output, err := e.executeSingleTool(n)
					resultCh <- toolResult{
						name: n.ToolName,
						result: &configs.ToolExecResult{
							ToolName: n.ToolName,
							Output:   output,
							Error:    errMsg(err),
							Status:   statusStr(err),
						},
					}
				}(node)
			}
		}
	}

	// Start initial ready nodes (those with no dependencies)
	tryStartReady()

	// Collect results and trigger newly ready nodes
	for i := 0; i < len(e.dag.Nodes); i++ {
		select {
		case tr := <-resultCh:
			execCtx.ReportDone(tr.result)
			log.Infof("[ToolDAG] Tool '%s' completed: %s", tr.name, tr.result.Status)
			// Try to start next batch of tools
			tryStartReady()
		case <-ctx.Done():
			return execCtx.results, ctx.Err()
		}
	}

	log.Infof("[ToolDAG] Execution completed for task %s", e.taskID)
	return execCtx.results, nil
}

// executeSingleTool executes a single tool via the agent's dispatcher
func (e *ToolDAGExecutor) executeSingleTool(node configs.ToolDAGNode) (map[string]interface{}, error) {
	// Get tool definition from MemSpace
	toolDef, err := e.agent.getToolDefinition(node.ToolName, e.memSpaceID)
	if err != nil {
		return nil, fmt.Errorf("tool '%s' not found: %w", node.ToolName, err)
	}

	// Merge parameters (node params override default)
	params := node.Params
	if params == nil {
		params = make(map[string]interface{})
	}
	// Invoke tool using existing agent logic
	return e.agent.invokeTool(toolDef, params)
}

// buildDependencyMap constructs the adjacency list for the DAG
func (e *ToolDAGExecutor) buildDependencyMap() map[string][]string {
	deps := make(map[string][]string)

	// Initialize all nodes with empty dependencies
	for _, node := range e.dag.Nodes {
		deps[node.ToolName] = []string{}
	}

	// Build map for quick lookup
	nodeNames := make(map[string]bool)
	for _, node := range e.dag.Nodes {
		nodeNames[node.ToolName] = true
	}

	// Process edges
	for _, edge := range e.dag.Edges {
		// Only consider edges where both endpoints exist in current DAG
		if nodeNames[edge.From] && nodeNames[edge.To] {
			deps[edge.To] = append(deps[edge.To], edge.From)
		}
	}

	return deps
}

// toolExecContext tracks the state of tool execution within a DAG
type toolExecContext struct {
	mu      sync.Mutex
	total   int
	done    int
	results map[string]*configs.ToolExecResult
	allDone chan struct{}
}

func newToolExecContext(total int) *toolExecContext {
	return &toolExecContext{
		total:   total,
		done:    0,
		results: make(map[string]*configs.ToolExecResult),
		allDone: make(chan struct{}),
	}
}

// ReportDone marks a tool as completed and checks if all are done
func (ec *toolExecContext) ReportDone(result *configs.ToolExecResult) {
	ec.mu.Lock()
	defer ec.mu.Unlock()
	ec.results[result.ToolName] = result
	ec.done++
	if ec.done >= ec.total {
		close(ec.allDone)
	}
}

// Helper functions for error handling
func errMsg(err error) string {
	if err != nil {
		return err.Error()
	}
	return ""
}

func statusStr(err error) string {
	if err != nil {
		return "failed"
	}
	return "completed"
}
