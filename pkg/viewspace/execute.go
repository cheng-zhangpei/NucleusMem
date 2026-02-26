// pkg/viewspace/execute.go

package viewspace

import (
	"NucleusMem/pkg/api"
	"fmt"
	"sync"
	"time"

	"github.com/pingcap-incubator/tinykv/log"
)

// NodeResult is the output of a completed ViewSpaceNode
type NodeResult struct {
	NodeName string                 `json:"node_name"`
	Status   string                 `json:"status"` // "completed" | "failed"
	Output   map[string]interface{} `json:"output,omitempty"`
	Error    string                 `json:"error,omitempty"`
}

// ExecutionContext tracks the execution state of a node's children
// each ViewSpaceNode will bind a ExecutionContext to record the execution status of each child nodes
type ExecutionContext struct {
	mu             sync.Mutex
	totalChildren  int
	completedCount int
	childResults   map[string]*NodeResult // childName -> result
	allDone        chan struct{}          // closed when all children complete
}

func newExecutionContext(childCount int) *ExecutionContext {
	return &ExecutionContext{
		totalChildren:  childCount,
		completedCount: 0,
		childResults:   make(map[string]*NodeResult),
		allDone:        make(chan struct{}),
	}
}

// ReportChildDone is called when a child node finishes
// Returns true if all children are now done
// child ----report----> parent
// the parent node call this function to calc if all tasks were done
func (ec *ExecutionContext) ReportChildDone(result *NodeResult) bool {
	ec.mu.Lock()
	defer ec.mu.Unlock()

	ec.childResults[result.NodeName] = result
	ec.completedCount++

	log.Infof("[Exec] Child '%s' reported done (%d/%d)",
		result.NodeName, ec.completedCount, ec.totalChildren)

	if ec.completedCount >= ec.totalChildren {
		close(ec.allDone) // send the sig to the Done Channel
		return true
	}
	return false
}

// WaitForAllChildren blocks until all children have reported completion
func (ec *ExecutionContext) WaitForAllChildren(timeout time.Duration) error {
	select {
	case <-ec.allDone:
		return nil
	case <-time.After(timeout):
		ec.mu.Lock()
		defer ec.mu.Unlock()
		return fmt.Errorf("timeout: %d/%d children completed", ec.completedCount, ec.totalChildren)
	}
}

// GetAllResults returns all child results
func (ec *ExecutionContext) GetAllResults() map[string]*NodeResult {
	ec.mu.Lock()
	defer ec.mu.Unlock()
	cp := make(map[string]*NodeResult, len(ec.childResults))
	for k, v := range ec.childResults {
		cp[k] = v
	}
	return cp
}

// Execute starts the execution of the entire tree from root
// This is called after Grow is complete
func (t *ViewSpaceTree) Execute() (*NodeResult, error) {
	if t.Root == nil {
		return nil, fmt.Errorf("tree has no root")
	}

	log.Infof("=== ViewSpaceTree Execute ===")
	result, err := t.executeNode(t.Root)
	if err != nil {
		return nil, fmt.Errorf("execution failed: %w", err)
	}

	log.Infof("=== Execution Complete ===")
	log.Infof("Final result: %+v", result)
	return result, nil
}

// executeNode runs a single node and returns its result
// This is recursive: non-leaf nodes wait for their children first
func (t *ViewSpaceTree) executeNode(node *ViewSpaceNode) (*NodeResult, error) {
	node.mu.Lock()
	node.Status = NodeStatusRunning
	node.mu.Unlock()

	log.Infof("[Exec] Starting '%s' [%s]", node.Name, node.Type)

	switch node.Type {
	case "atomic":
		return t.executeAtomic(node)
	case "process", "global":
		return t.executeComposite(node)
	default:
		return nil, fmt.Errorf("unknown node type: %s", node.Type)
	}
}

// executeAtomic runs the actual tools on an atomic node
func (t *ViewSpaceTree) executeAtomic(node *ViewSpaceNode) (*NodeResult, error) {
	log.Infof("[Exec] Atomic '%s': executing tools %v", node.Name, node.Tools)

	// Submit tool tasks to the agent sequentially
	// (later: use tool DAG for parallel execution)
	allOutput := make(map[string]interface{})

	for _, toolName := range node.Tools {
		log.Infof("[Exec] '%s': running tool '%s'", node.Name, toolName)
		// Submit tool task
		submitReq := &api.SubmitTaskRequest{
			Type:     "tool",
			ToolName: toolName,
			Content:  node.Description,
		}

		submitResp, err := node.AgentClient.SubmitTask(submitReq)
		if err != nil {
			node.Status = NodeStatusFailed
			return &NodeResult{
				NodeName: node.Name,
				Status:   "failed",
				Error:    fmt.Sprintf("submit tool '%s': %v", toolName, err),
			}, nil
		}

		// Wait for tool result
		resultResp, err := node.AgentClient.GetTaskResult(submitResp.TaskID, 60*time.Second)
		if err != nil {
			node.Status = NodeStatusFailed
			return &NodeResult{
				NodeName: node.Name,
				Status:   "failed",
				Error:    fmt.Sprintf("tool '%s' timeout: %v", toolName, err),
			}, nil
		}

		if resultResp.ErrorMessage != "" {
			log.Warnf("[Exec] '%s' tool '%s' failed: %s", node.Name, toolName, resultResp.ErrorMessage)
			node.Status = NodeStatusFailed
			return &NodeResult{
				NodeName: node.Name,
				Status:   "failed",
				Error:    fmt.Sprintf("tool '%s': %s", toolName, resultResp.ErrorMessage),
			}, nil
		}

		// Store tool output
		allOutput[toolName] = resultResp.Result
		log.Infof("[Exec] '%s' tool '%s' completed", node.Name, toolName)
	}

	node.mu.Lock()
	node.Status = NodeStatusDone
	node.mu.Unlock()

	result := &NodeResult{
		NodeName: node.Name,
		Status:   "completed",
		Output:   allOutput,
	}

	log.Infof("[Exec] Atomic '%s' done: %d tool outputs", node.Name, len(allOutput))
	return result, nil
}

// executeComposite runs a process or global node:
// launches all children concurrently, waits for them, aggregates results
func (t *ViewSpaceTree) executeComposite(node *ViewSpaceNode) (*NodeResult, error) {
	node.mu.RLock()
	children := make([]*ViewSpaceNode, len(node.Children))
	copy(children, node.Children)
	dataflow := make([]DataflowEdge, len(node.ChildDataflow))
	copy(dataflow, node.ChildDataflow)
	node.mu.RUnlock()

	if len(children) == 0 {
		node.Status = NodeStatusDone
		return &NodeResult{
			NodeName: node.Name,
			Status:   "completed",
			Output:   map[string]interface{}{"note": "no children"},
		}, nil
	}

	// Build dependency map from dataflow
	// A child can start only when all its dataflow dependencies are done
	deps := buildDependencyMap(children, dataflow)
	// Execution context tracks completion
	execCtx := newExecutionContext(len(children))
	// Results channel for collecting from goroutines
	type childResult struct {
		name   string
		result *NodeResult
		err    error
	}
	resultCh := make(chan childResult, len(children))

	// Track which children have been started
	started := make(map[string]bool)
	var startMu sync.Mutex

	// tryStartReady checks which children can start and launches them
	tryStartReady := func() {
		startMu.Lock()
		defer startMu.Unlock()

		for _, child := range children {
			if started[child.Name] {
				continue
			}

			// Check if all dependencies of the child are satisfied
			allDepsDone := true
			for _, depName := range deps[child.Name] {
				execCtx.mu.Lock()
				_, done := execCtx.childResults[depName]
				execCtx.mu.Unlock()
				if !done {
					allDepsDone = false
					break
				}
			}

			if allDepsDone {
				// all the dependencies of the child have been finished, so this is time to execute the child?
				started[child.Name] = true
				log.Infof("[Exec] '%s': starting child '%s'", node.Name, child.Name)
				go func(c *ViewSpaceNode) {
					result, err := t.executeNode(c)
					resultCh <- childResult{name: c.Name, result: result, err: err}
				}(child)
			}
		}
	}

	// Start initial ready children (those with no dependencies)
	tryStartReady()

	// Collect results and start newly-ready children
	for i := 0; i < len(children); i++ {
		cr := <-resultCh // block here until at least one child finish its all task

		if cr.err != nil {
			log.Errorf("[Exec] '%s' child '%s' error: %v", node.Name, cr.name, cr.err)
			cr.result = &NodeResult{
				NodeName: cr.name,
				Status:   "failed",
				Error:    cr.err.Error(),
			}
		}

		execCtx.ReportChildDone(cr.result)

		// After a child completes, check if new children can start
		tryStartReady()
	}

	// Aggregate results
	allResults := execCtx.GetAllResults()
	aggregatedOutput := make(map[string]interface{})
	allSuccess := true

	for name, result := range allResults {
		aggregatedOutput[name] = result.Output
		if result.Status != "completed" {
			allSuccess = false
		}
	}

	status := "completed"
	if !allSuccess {
		status = "partial"
	}

	node.mu.Lock()
	if allSuccess {
		node.Status = NodeStatusDone
	} else {
		node.Status = NodeStatusFailed
	}
	node.mu.Unlock()

	finalResult := &NodeResult{
		NodeName: node.Name,
		Status:   status,
		Output:   aggregatedOutput,
	}

	log.Infof("[Exec] Composite '%s' done: %d/%d children succeeded",
		node.Name, countCompleted(allResults), len(children))

	return finalResult, nil
}

// buildDependencyMap returns: childName -> [names of children it depends on]
func buildDependencyMap(children []*ViewSpaceNode, dataflow []DataflowEdge) map[string][]string {
	deps := make(map[string][]string)

	// Initialize all children with empty deps
	for _, c := range children {
		deps[c.Name] = []string{}
	}

	// Build from dataflow edges
	// If there's an edge from A -> B, B depends on A
	childNames := make(map[string]bool)
	for _, c := range children {
		childNames[c.Name] = true
	}

	for _, edge := range dataflow {
		// Only consider edges where both endpoints are direct children
		if childNames[edge.From] && childNames[edge.To] {
			deps[edge.To] = append(deps[edge.To], edge.From)
		}
	}

	return deps
}

func countCompleted(results map[string]*NodeResult) int {
	count := 0
	for _, r := range results {
		if r.Status == "completed" {
			count++
		}
	}
	return count
}
