// pkg/viewspace/grow.go

package viewspace

import (
	"NucleusMem/pkg/api"
	"fmt"
	"time"

	"github.com/pingcap-incubator/tinykv/log"
)

type SpawnResult struct {
	Children []SpawnChild   `json:"children"`
	Dataflow []DataflowEdge `json:"dataflow"`
}

type SpawnChild struct {
	Name        string   `json:"name"`
	Type        string   `json:"type"`
	Tags        []string `json:"tags"`
	Description string   `json:"description"`
	Role        string   `json:"role"`
	Tools       []string `json:"tools,omitempty"`
}

// Grow makes a node's agent decompose its task, creates children,
// then recursively grows process children.
// Only global and process nodes can grow. Each node grows only once.
func (t *ViewSpaceTree) Grow(node *ViewSpaceNode) error {
	// Guard: atomic cannot grow
	if node.Type == "atomic" {
		log.Infof("[Grow] '%s' is atomic, skip", node.Name)
		return nil
	}

	// Guard: only grow once
	node.mu.Lock()
	if node.hasGrown {
		node.mu.Unlock()
		log.Warnf("[Grow] '%s' already grown, skip", node.Name)
		return nil
	}
	node.hasGrown = true
	node.Status = NodeStatusGrowing
	node.mu.Unlock()

	log.Infof("[Grow] '%s' (agent:%d) decomposing...", node.Name, node.AgentID)

	// Step 1: Submit decompose task to the agent via HTTP
	submitReq := &api.SubmitTaskRequest{
		Type:           "decompose",
		Content:        node.Description,
		AvailableTools: t.AvailableTools,
		MaxRetry:       3,
	}

	submitResp, err := node.AgentClient.SubmitTask(submitReq)
	if err != nil {
		node.Status = NodeStatusFailed
		return fmt.Errorf("submit to '%s': %w", node.Name, err)
	}

	log.Infof("[Grow] '%s' task=%s, waiting...", node.Name, submitResp.TaskID)

	// Step 2: Wait for agent to finish (LLM call happens inside agent)
	resultResp, err := node.AgentClient.GetTaskResult(submitResp.TaskID, 120*time.Second)
	if err != nil {
		node.Status = NodeStatusFailed
		return fmt.Errorf("wait for '%s': %w", node.Name, err)
	}

	if resultResp.ErrorMessage != "" {
		node.Status = NodeStatusFailed
		return fmt.Errorf("decompose '%s' failed: %s", node.Name, resultResp.ErrorMessage)
	}

	// Step 3: Parse agent's output into SpawnResult
	spawnResult, err := parseAgentDecomposeResult(resultResp.Result)
	if err != nil {
		node.Status = NodeStatusFailed
		return fmt.Errorf("parse '%s': %w", node.Name, err)
	}

	log.Infof("[Grow] '%s' → %d children, %d dataflow",
		node.Name, len(spawnResult.Children), len(spawnResult.Dataflow))

	// Step 4: Create child nodes
	for _, childDef := range spawnResult.Children {
		child := &ViewSpaceNode{
			Name:        childDef.Name,
			Type:        childDef.Type,
			Tags:        childDef.Tags,
			Description: childDef.Description,
			Role:        childDef.Role,
			Tools:       childDef.Tools,
			Parent:      node,
			Children:    make([]*ViewSpaceNode, 0),
			Status:      NodeStatusPending,
			tree:        t,
		}

		// Bring child to life (create memspace, agent, bind, client)
		if err := t.bringToLife(child); err != nil {
			node.Status = NodeStatusFailed
			return fmt.Errorf("create child '%s' under '%s': %w", child.Name, node.Name, err)
		}

		// Register in tree
		t.mu.Lock()
		t.Nodes[child.Name] = child
		t.mu.Unlock()

		node.mu.Lock()
		node.Children = append(node.Children, child)
		node.mu.Unlock()

		log.Infof("[Grow] '%s' spawned [%s] '%s' (agent:%d, ms:%d)",
			node.Name, child.Type, child.Name, child.AgentID, child.MemSpaceID)
	}

	// Step 5: Store dataflow edges between children
	node.mu.Lock()
	node.ChildDataflow = spawnResult.Dataflow
	node.Status = NodeStatusReady
	node.mu.Unlock()

	// Step 6: Recursively grow process children
	// The child's Description was already set by the parent's decomposition,
	// so when the child's agent decomposes, it sees the sub-task assigned by parent.
	for _, child := range node.Children {
		if child.Type == "process" {
			log.Infof("[Grow] Recursing into process '%s'...", child.Name)
			if err := t.Grow(child); err != nil {
				return fmt.Errorf("recursive grow '%s': %w", child.Name, err)
			}
		}
	}
	log.Infof("[Grow] '%s' complete: %d children", node.Name, len(node.Children))
	return nil
}
