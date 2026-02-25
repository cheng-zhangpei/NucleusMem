// pkg/runtime/memspace/region/task_region_test.go

package memspace_region

import (
	tinykv_client "NucleusMem/pkg/storage/tinykv-client"
	"testing"
)

const testPDAddr = "127.0.0.1:2379"

// Each test uses a different memSpaceID to avoid key conflicts between tests
func newTestTaskRegion(memSpaceID uint64) *TaskRegion {
	client, err := tinykv_client.NewMemClient(testPDAddr)
	if err != nil {
		panic("failed to connect to TinyKV: " + err.Error())
	}
	return NewTaskRegion(client, memSpaceID)
}

func TestTaskRegion_SaveAndLoadDAG(t *testing.T) {
	tr := newTestTaskRegion(10001)
	tr.Cleanup()
	dag := &TaskDAG{
		Nodes: []TaskDAGNode{
			{Name: "static-analysis", Description: "run lint"},
			{Name: "security-scan", Description: "scan vulnerabilities"},
			{Name: "synthesis", Description: "generate report"},
		},
		Edges: []TaskDAGEdge{
			{From: "static-analysis", To: "synthesis", Fields: []string{"metrics", "violations"}},
			{From: "security-scan", To: "synthesis", Fields: []string{"vulnerabilities"}},
		},
	}

	if err := tr.SaveDAG(dag); err != nil {
		t.Fatalf("SaveDAG failed: %v", err)
	}

	loaded, err := tr.LoadDAG()
	if err != nil {
		t.Fatalf("LoadDAG failed: %v", err)
	}

	if len(loaded.Nodes) != 3 {
		t.Errorf("expected 3 nodes, got %d", len(loaded.Nodes))
	}
	if len(loaded.Edges) != 2 {
		t.Errorf("expected 2 edges, got %d", len(loaded.Edges))
	}
	if loaded.Nodes[0].Name != "static-analysis" {
		t.Errorf("expected first node 'static-analysis', got '%s'", loaded.Nodes[0].Name)
	}
	if loaded.Edges[0].From != "static-analysis" || loaded.Edges[0].To != "synthesis" {
		t.Errorf("unexpected edge: from=%s to=%s", loaded.Edges[0].From, loaded.Edges[0].To)
	}
}

func TestTaskRegion_UpdateAndGetNodeStatus(t *testing.T) {
	tr := newTestTaskRegion(10002)
	tr.Cleanup()

	// No state initially
	_, err := tr.GetNodeState("nodeA")
	if err == nil {
		t.Error("expected error for non-existent node, got nil")
	}

	// Set to running
	if err := tr.UpdateNodeStatus("nodeA", TaskNodeRunning); err != nil {
		t.Fatalf("UpdateNodeStatus to running failed: %v", err)
	}

	state, err := tr.GetNodeState("nodeA")
	if err != nil {
		t.Fatalf("GetNodeState failed: %v", err)
	}
	if state.Status != TaskNodeRunning {
		t.Errorf("expected running, got %s", state.Status)
	}
	if state.StartedAt == 0 {
		t.Error("expected StartedAt to be set")
	}

	// Set to completed
	if err := tr.UpdateNodeStatus("nodeA", TaskNodeCompleted); err != nil {
		t.Fatalf("UpdateNodeStatus to completed failed: %v", err)
	}

	state, err = tr.GetNodeState("nodeA")
	if err != nil {
		t.Fatalf("GetNodeState failed: %v", err)
	}
	if state.Status != TaskNodeCompleted {
		t.Errorf("expected completed, got %s", state.Status)
	}
	if state.DoneAt == 0 {
		t.Error("expected DoneAt to be set")
	}
}

func TestTaskRegion_UpdateNodeResult_Success(t *testing.T) {
	tr := newTestTaskRegion(10003)
	tr.Cleanup()

	result := map[string]interface{}{
		"metrics":    "all_good",
		"violations": []interface{}{"v1", "v2"},
	}

	if err := tr.UpdateNodeResult("nodeA", result, ""); err != nil {
		t.Fatalf("UpdateNodeResult failed: %v", err)
	}

	// Status should be completed
	state, err := tr.GetNodeState("nodeA")
	if err != nil {
		t.Fatalf("GetNodeState failed: %v", err)
	}
	if state.Status != TaskNodeCompleted {
		t.Errorf("expected completed, got %s", state.Status)
	}

	// Result content
	got, err := tr.GetNodeResult("nodeA")
	if err != nil {
		t.Fatalf("GetNodeResult failed: %v", err)
	}
	if got["metrics"] != "all_good" {
		t.Errorf("expected metrics 'all_good', got %v", got["metrics"])
	}
}

func TestTaskRegion_UpdateNodeResult_Failure(t *testing.T) {
	tr := newTestTaskRegion(10004)
	tr.Cleanup()

	if err := tr.UpdateNodeResult("nodeB", nil, "something broke"); err != nil {
		t.Fatalf("UpdateNodeResult failed: %v", err)
	}

	state, err := tr.GetNodeState("nodeB")
	if err != nil {
		t.Fatalf("GetNodeState failed: %v", err)
	}
	if state.Status != TaskNodeFailed {
		t.Errorf("expected failed, got %s", state.Status)
	}
	if state.Error != "something broke" {
		t.Errorf("expected error 'something broke', got '%s'", state.Error)
	}
}

func TestTaskRegion_GetAllNodeStates(t *testing.T) {
	tr := newTestTaskRegion(10005)
	tr.Cleanup()

	tr.UpdateNodeStatus("A", TaskNodeCompleted)
	tr.UpdateNodeStatus("B", TaskNodeRunning)
	tr.UpdateNodeStatus("C", TaskNodePending)

	states, err := tr.GetAllNodeStates()
	if err != nil {
		t.Fatalf("GetAllNodeStates failed: %v", err)
	}

	// At least 3 states (might have more if memSpaceID collides, but we use unique IDs)
	if len(states) < 3 {
		t.Errorf("expected at least 3 states, got %d", len(states))
	}
	if states["A"] == nil || states["A"].Status != TaskNodeCompleted {
		t.Errorf("expected A completed, got %v", states["A"])
	}
	if states["B"] == nil || states["B"].Status != TaskNodeRunning {
		t.Errorf("expected B running, got %v", states["B"])
	}
	if states["C"] == nil || states["C"].Status != TaskNodePending {
		t.Errorf("expected C pending, got %v", states["C"])
	}
}

func TestTaskRegion_GetReadyNodes(t *testing.T) {
	tr := newTestTaskRegion(10006)
	tr.Cleanup()

	// DAG: A → C, B → C
	dag := &TaskDAG{
		Nodes: []TaskDAGNode{
			{Name: "A"},
			{Name: "B"},
			{Name: "C"},
		},
		Edges: []TaskDAGEdge{
			{From: "A", To: "C"},
			{From: "B", To: "C"},
		},
	}
	if err := tr.SaveDAG(dag); err != nil {
		t.Fatalf("SaveDAG failed: %v", err)
	}

	// Initially A and B should be ready (no deps)
	ready, err := tr.GetReadyNodes()
	if err != nil {
		t.Fatalf("GetReadyNodes failed: %v", err)
	}
	readySet := toSet(ready)
	if !readySet["A"] || !readySet["B"] {
		t.Errorf("expected A and B ready, got %v", ready)
	}
	if readySet["C"] {
		t.Errorf("C should not be ready yet, got %v", ready)
	}

	// Complete A → B still ready, C blocked
	tr.UpdateNodeResult("A", map[string]interface{}{"done": true}, "")
	ready, err = tr.GetReadyNodes()
	if err != nil {
		t.Fatalf("GetReadyNodes failed: %v", err)
	}
	readySet = toSet(ready)
	if !readySet["B"] {
		t.Errorf("expected B ready, got %v", ready)
	}
	if readySet["C"] {
		t.Errorf("C should still be blocked, got %v", ready)
	}

	// Complete B → C ready
	tr.UpdateNodeResult("B", map[string]interface{}{"done": true}, "")
	ready, err = tr.GetReadyNodes()
	if err != nil {
		t.Fatalf("GetReadyNodes failed: %v", err)
	}
	readySet = toSet(ready)
	if !readySet["C"] {
		t.Errorf("expected C ready, got %v", ready)
	}

	// Complete C → nothing ready
	tr.UpdateNodeResult("C", map[string]interface{}{"done": true}, "")
	ready, err = tr.GetReadyNodes()
	if err != nil {
		t.Fatalf("GetReadyNodes failed: %v", err)
	}
	if len(ready) != 0 {
		t.Errorf("expected 0 ready nodes, got %v", ready)
	}
}

func TestTaskRegion_GetReadyNodes_FailedDep(t *testing.T) {
	tr := newTestTaskRegion(10007)
	tr.Cleanup()

	// DAG: A → B
	dag := &TaskDAG{
		Nodes: []TaskDAGNode{{Name: "A"}, {Name: "B"}},
		Edges: []TaskDAGEdge{{From: "A", To: "B"}},
	}
	tr.SaveDAG(dag)

	// Fail A → B should NOT become ready
	tr.UpdateNodeResult("A", nil, "crashed")
	ready, err := tr.GetReadyNodes()
	if err != nil {
		t.Fatalf("GetReadyNodes failed: %v", err)
	}
	readySet := toSet(ready)
	if readySet["B"] {
		t.Error("B should not be ready when dependency A failed")
	}
}

func TestTaskRegion_IsAllCompleted(t *testing.T) {
	tr := newTestTaskRegion(10008)
	tr.Cleanup()

	dag := &TaskDAG{
		Nodes: []TaskDAGNode{{Name: "X"}, {Name: "Y"}},
		Edges: []TaskDAGEdge{},
	}
	tr.SaveDAG(dag)

	done, err := tr.IsAllCompleted()
	if err != nil {
		t.Fatalf("IsAllCompleted failed: %v", err)
	}
	if done {
		t.Error("should not be all completed initially")
	}

	// Complete X only
	tr.UpdateNodeResult("X", map[string]interface{}{}, "")
	done, _ = tr.IsAllCompleted()
	if done {
		t.Error("should not be all completed with only X done")
	}

	// Complete Y
	tr.UpdateNodeResult("Y", map[string]interface{}{}, "")
	done, _ = tr.IsAllCompleted()
	if !done {
		t.Error("should be all completed now")
	}
}

func TestTaskRegion_AuditLog(t *testing.T) {
	tr := newTestTaskRegion(10009)
	tr.Cleanup()

	if err := tr.WriteAuditEntry(1, "task_start", "nodeA", "starting analysis"); err != nil {
		t.Fatalf("WriteAuditEntry failed: %v", err)
	}
	if err := tr.WriteAuditEntry(1, "tool_call", "nodeA", "calling lint"); err != nil {
		t.Fatalf("WriteAuditEntry failed: %v", err)
	}
	if err := tr.WriteAuditEntry(2, "task_complete", "nodeB", "scan finished"); err != nil {
		t.Fatalf("WriteAuditEntry failed: %v", err)
	}

	// Get all
	entries, err := tr.GetAuditEntries(0)
	if err != nil {
		t.Fatalf("GetAuditEntries failed: %v", err)
	}
	if len(entries) != 3 {
		t.Errorf("expected 3 audit entries, got %d", len(entries))
	}

	// Verify content of first entry
	found := false
	for _, e := range entries {
		if e.Action == "task_start" && e.NodeName == "nodeA" {
			found = true
			if e.AgentID != 1 {
				t.Errorf("expected AgentID 1, got %d", e.AgentID)
			}
		}
	}
	if !found {
		t.Error("task_start entry for nodeA not found")
	}

	// Filter with future timestamp → 0 results
	entries, err = tr.GetAuditEntries(9999999999)
	if err != nil {
		t.Fatalf("GetAuditEntries failed: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries with future filter, got %d", len(entries))
	}
}

func TestTaskRegion_DAGOverwrite(t *testing.T) {
	tr := newTestTaskRegion(10010)
	tr.Cleanup()

	// Save first DAG
	dag1 := &TaskDAG{
		Nodes: []TaskDAGNode{{Name: "A"}, {Name: "B"}},
		Edges: []TaskDAGEdge{{From: "A", To: "B"}},
	}
	tr.SaveDAG(dag1)

	// Overwrite with a different DAG
	dag2 := &TaskDAG{
		Nodes: []TaskDAGNode{{Name: "X"}, {Name: "Y"}, {Name: "Z"}},
		Edges: []TaskDAGEdge{
			{From: "X", To: "Z"},
			{From: "Y", To: "Z"},
		},
	}
	tr.SaveDAG(dag2)

	// Should load the second DAG
	loaded, err := tr.LoadDAG()
	if err != nil {
		t.Fatalf("LoadDAG failed: %v", err)
	}
	if len(loaded.Nodes) != 3 {
		t.Errorf("expected 3 nodes from overwritten DAG, got %d", len(loaded.Nodes))
	}
	if loaded.Nodes[0].Name != "X" {
		t.Errorf("expected first node 'X', got '%s'", loaded.Nodes[0].Name)
	}
}

// helper
func toSet(strs []string) map[string]bool {
	m := make(map[string]bool, len(strs))
	for _, s := range strs {
		m[s] = true
	}
	return m
}
