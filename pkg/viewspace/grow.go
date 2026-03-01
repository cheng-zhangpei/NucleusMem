// pkg/viewspace/grow.go

package viewspace

import (
	"NucleusMem/pkg/api"
	"NucleusMem/pkg/configs"
	"fmt"
	"strings"
	"time"

	"github.com/pingcap-incubator/tinykv/log"
)

type SpawnResult struct {
	Children []SpawnChild   `json:"children"`
	Dataflow []DataflowEdge `json:"dataflow"`
}

// pkg/viewspace/grow.go

type SpawnChild struct {
	Name        string   `json:"name"`
	Type        string   `json:"type"`
	Tags        []string `json:"tags"`
	Description string   `json:"description"`
	Role        string   `json:"role"` // RepresentedAgent 角色
	Tools       []string `json:"tools,omitempty"`

	// 新增：ViewSpace 内部协作声明
	Workers []WorkerSpec `json:"workers,omitempty"`

	// 新增：额外 MemSpace 需求
	MountMemSpaces []MemSpaceMount `json:"mount_memspaces,omitempty"`
}

// WorkerSpec declares a worker agent needed within a ViewSpace
type WorkerSpec struct {
	Role        string `json:"role"`
	Description string `json:"description"`
}

// MemSpaceMount declares an additional MemSpace to mount
type MemSpaceMount struct {
	Source   string `json:"source"`    // "parent" | "new" | "tag:<tag_name>"
	Purpose  string `json:"purpose"`   // 说明用途
	ReadOnly bool   `json:"read_only"` // 是否只读
}

// Grow makes a node's agent decompose its task, creates children,
// then recursively grows process children.
// Only global and process nodes can grow. Each node grows only once.
// pkg/viewspace/grow.go

func (t *ViewSpaceTree) Grow(node *ViewSpaceNode) error {
	if node.Type == "atomic" {
		t.Growth.Record("grow", node.Name, "Atomic node, no growth needed", t)
		return nil
	}

	node.mu.Lock()
	if node.hasGrown {
		node.mu.Unlock()
		return nil
	}
	node.hasGrown = true
	node.Status = NodeStatusGrowing
	node.mu.Unlock()

	// Step 1: Decompose
	t.Growth.Record("decompose", node.Name,
		fmt.Sprintf("Asking agent:%d to decompose task...", node.AgentID), t)

	submitReq := &api.SubmitTaskRequest{
		Type:           "decompose",
		Content:        node.Description,
		AvailableTools: t.AvailableTools,
		MaxRetry:       3,
	}

	submitResp, err := node.AgentClient.SubmitTask(submitReq)
	if err != nil {
		node.Status = NodeStatusFailed
		t.Growth.Record("failed", node.Name,
			fmt.Sprintf("Submit failed: %v", err), t)
		return fmt.Errorf("submit to '%s': %w", node.Name, err)
	}

	resultResp, err := node.AgentClient.GetTaskResult(submitResp.TaskID, 120*time.Second)
	if err != nil {
		node.Status = NodeStatusFailed
		t.Growth.Record("failed", node.Name,
			fmt.Sprintf("Wait timeout: %v", err), t)
		return fmt.Errorf("wait for '%s': %w", node.Name, err)
	}

	if resultResp.ErrorMessage != "" {
		node.Status = NodeStatusFailed
		t.Growth.Record("failed", node.Name,
			fmt.Sprintf("Decompose error: %s", resultResp.ErrorMessage), t)
		return fmt.Errorf("decompose '%s' failed: %s", node.Name, resultResp.ErrorMessage)
	}

	// Step 2: Parse
	spawnResult, err := parseAgentDecomposeResult(resultResp.Result)
	if err != nil {
		node.Status = NodeStatusFailed
		t.Growth.Record("failed", node.Name,
			fmt.Sprintf("Parse failed: %v", err), t)
		return fmt.Errorf("parse '%s': %w", node.Name, err)
	}

	t.Growth.Record("decompose", node.Name,
		fmt.Sprintf("Decomposed into %d children, %d dataflows",
			len(spawnResult.Children), len(spawnResult.Dataflow)), t)

	// Step 3: Spawn children
	for _, childDef := range spawnResult.Children {
		child := &ViewSpaceNode{
			Name:           childDef.Name,
			Type:           childDef.Type,
			Tags:           childDef.Tags,
			Description:    childDef.Description,
			Role:           childDef.Role,
			Tools:          childDef.Tools,
			Parent:         node,
			Children:       make([]*ViewSpaceNode, 0),
			Status:         NodeStatusPending,
			tree:           t,
			workersSpecs:   childDef.Workers,
			memSpaceMounts: childDef.MountMemSpaces,
		}

		t.Growth.Record("spawn", child.Name,
			fmt.Sprintf("Spawning [%s] under '%s' (role: %s)",
				child.Type, node.Name, child.Role), t)

		// Inject default parent mount
		t.injectDefaultMounts(child)

		// Bring to life
		if err := t.bringToLife(child); err != nil {
			node.Status = NodeStatusFailed
			t.Growth.Record("failed", child.Name,
				fmt.Sprintf("bringToLife failed: %v", err), t)
			return fmt.Errorf("create child '%s' under '%s': %w", child.Name, node.Name, err)
		}

		// Register in tree
		t.mu.Lock()
		t.Nodes[child.Name] = child
		t.mu.Unlock()

		node.mu.Lock()
		node.Children = append(node.Children, child)
		node.mu.Unlock()

		// Inject ToolDAG for atomic nodes
		if child.Type == "atomic" && len(childDef.Tools) > 0 {
			dag := &configs.ToolDAG{
				Nodes: make([]configs.ToolDAGNode, len(childDef.Tools)),
				Edges: []configs.ToolDAGEdge{},
			}
			for i, toolName := range childDef.Tools {
				dag.Nodes[i] = configs.ToolDAGNode{ToolName: toolName}
			}
			if child.MemSpaceClient != nil {
				if err := child.MemSpaceClient.SaveToolDAG(dag); err != nil {
					log.Warnf("[Grow] Failed to save ToolDAG for %s: %v", child.Name, err)
				}
			}
		}

		t.Growth.Record("ready", child.Name,
			fmt.Sprintf("[%s] alive → agent:%d, ms:%d, workers:%d, mounts:%d",
				child.Type, child.AgentID, child.MemSpaceID,
				len(child.Workers), len(child.MountedMemSpaces)), t)
	}

	// Step 4: Store dataflow
	node.mu.Lock()
	node.ChildDataflow = spawnResult.Dataflow
	node.Status = NodeStatusReady
	node.mu.Unlock()

	if len(spawnResult.Dataflow) > 0 {
		dfDesc := make([]string, 0, len(spawnResult.Dataflow))
		for _, df := range spawnResult.Dataflow {
			dfDesc = append(dfDesc, fmt.Sprintf("%s→%s", df.From, df.To))
		}
		t.Growth.Record("grow", node.Name,
			fmt.Sprintf("Dataflow: %s", strings.Join(dfDesc, ", ")), t)
	}

	// Step 5: Recurse into process children
	for _, child := range node.Children {
		if child.Type == "process" {
			t.Growth.Record("grow", child.Name,
				"Recursing into process node...", t)
			if err := t.Grow(child); err != nil {
				return fmt.Errorf("recursive grow '%s': %w", child.Name, err)
			}
		}
	}

	t.Growth.Record("ready", node.Name,
		fmt.Sprintf("Growth complete: %d children", len(node.Children)), t)
	// when the recursive process finished, print the struct of the tree
	if node.Parent == nil {
		t.PrintSnapshot()
	}
	return nil
}
