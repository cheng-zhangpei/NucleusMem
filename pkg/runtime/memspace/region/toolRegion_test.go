// pkg/runtime/memspace/region/tool_region_test.go

package memspace_region

import (
	tinykv_client "NucleusMem/pkg/storage/tinykv-client"
	"testing"
)

func newTestToolRegion(memSpaceID uint64) *ToolRegion {
	client, err := tinykv_client.NewMemClient(testPDAddr)
	if err != nil {
		panic("failed to connect to TinyKV: " + err.Error())
	}
	return NewToolRegion(client, memSpaceID)
}

func TestToolRegion_RegisterAndGetTool(t *testing.T) {
	tr := newTestToolRegion(20001)
	tr.Cleanup()
	tool := &ToolDefinition{
		Name:        "lint",
		Description: "Run linter on source code",
		Tags:        []string{"static-analysis", "code-tools"},
		Parameters: []ToolParam{
			{Name: "path", Type: "string", Required: true},
			{Name: "config", Type: "string", Required: false, Default: ".eslintrc"},
		},
		ReturnType: "object",
		Endpoint:   "http://tools.internal/lint",
	}

	if err := tr.RegisterTool(tool); err != nil {
		t.Fatalf("RegisterTool failed: %v", err)
	}

	got, err := tr.GetTool("lint")
	if err != nil {
		t.Fatalf("GetTool failed: %v", err)
	}
	if got.Name != "lint" {
		t.Errorf("expected name 'lint', got '%s'", got.Name)
	}
	if got.Description != "Run linter on source code" {
		t.Errorf("unexpected description: %s", got.Description)
	}
	if len(got.Tags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(got.Tags))
	}
	if len(got.Parameters) != 2 {
		t.Errorf("expected 2 params, got %d", len(got.Parameters))
	}
	if got.Parameters[0].Required != true {
		t.Error("expected first param required=true")
	}
	if got.Parameters[1].Default != ".eslintrc" {
		t.Errorf("expected default '.eslintrc', got '%s'", got.Parameters[1].Default)
	}
	if got.CreatedAt == 0 {
		t.Error("expected CreatedAt to be auto-set")
	}
}

func TestToolRegion_GetTool_NotFound(t *testing.T) {
	tr := newTestToolRegion(20002)
	tr.Cleanup()

	_, err := tr.GetTool("nonexistent")
	if err == nil {
		t.Error("expected error for non-existent tool")
	}
}

func TestToolRegion_RegisterEmptyName(t *testing.T) {
	tr := newTestToolRegion(20003)
	tr.Cleanup()

	err := tr.RegisterTool(&ToolDefinition{Name: ""})
	if err == nil {
		t.Error("expected error for empty tool name")
	}
}

func TestToolRegion_GetTools_Batch(t *testing.T) {
	tr := newTestToolRegion(20004)
	tr.Cleanup()

	tr.RegisterTool(&ToolDefinition{Name: "lint", Tags: []string{"analysis"}})
	tr.RegisterTool(&ToolDefinition{Name: "scanner", Tags: []string{"security"}})
	tr.RegisterTool(&ToolDefinition{Name: "formatter", Tags: []string{"style"}})

	// Get subset
	got, err := tr.GetTools([]string{"lint", "formatter"})
	if err != nil {
		t.Fatalf("GetTools failed: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("expected 2 tools, got %d", len(got))
	}

	// Verify names
	names := map[string]bool{}
	for _, tool := range got {
		names[tool.Name] = true
	}
	if !names["lint"] || !names["formatter"] {
		t.Errorf("expected lint and formatter, got %v", names)
	}
}

func TestToolRegion_GetTools_WithNonExistent(t *testing.T) {
	tr := newTestToolRegion(20005)
	tr.Cleanup()

	tr.RegisterTool(&ToolDefinition{Name: "lint"})

	_, err := tr.GetTools([]string{"lint", "nonexistent"})
	if err == nil {
		t.Error("expected error when requesting non-existent tool")
	}
}

func TestToolRegion_ListTools(t *testing.T) {
	tr := newTestToolRegion(20006)
	tr.Cleanup()

	// Empty initially
	tools, err := tr.ListTools()
	if err != nil {
		t.Fatalf("ListTools failed: %v", err)
	}
	if len(tools) != 0 {
		t.Errorf("expected 0 tools initially, got %d", len(tools))
	}

	// Register 2 tools
	tr.RegisterTool(&ToolDefinition{Name: "lint"})
	tr.RegisterTool(&ToolDefinition{Name: "scanner"})

	tools, err = tr.ListTools()
	if err != nil {
		t.Fatalf("ListTools failed: %v", err)
	}
	if len(tools) != 2 {
		t.Errorf("expected 2 tools, got %d", len(tools))
	}
}

func TestToolRegion_DeleteTool(t *testing.T) {
	tr := newTestToolRegion(20007)
	tr.Cleanup()

	tr.RegisterTool(&ToolDefinition{Name: "lint"})
	tr.RegisterTool(&ToolDefinition{Name: "scanner"})

	// Delete lint
	if err := tr.DeleteTool("lint"); err != nil {
		t.Fatalf("DeleteTool failed: %v", err)
	}

	// lint should be gone
	_, err := tr.GetTool("lint")
	if err == nil {
		t.Error("expected error after deleting lint")
	}

	// scanner should still exist
	got, err := tr.GetTool("scanner")
	if err != nil {
		t.Fatalf("scanner should still exist: %v", err)
	}
	if got.Name != "scanner" {
		t.Errorf("expected 'scanner', got '%s'", got.Name)
	}

	// List should show only 1
	tools, _ := tr.ListTools()
	if len(tools) != 1 {
		t.Errorf("expected 1 tool after delete, got %d", len(tools))
	}
}

func TestToolRegion_FindToolsByTags(t *testing.T) {
	tr := newTestToolRegion(20008)
	tr.Cleanup()

	tr.RegisterTool(&ToolDefinition{Name: "lint", Tags: []string{"static-analysis", "code-tools"}})
	tr.RegisterTool(&ToolDefinition{Name: "scanner", Tags: []string{"security", "scanning"}})
	tr.RegisterTool(&ToolDefinition{Name: "complexity", Tags: []string{"static-analysis", "code-tools", "metrics"}})
	tr.RegisterTool(&ToolDefinition{Name: "formatter", Tags: []string{"code-tools", "formatting"}})

	tests := []struct {
		name     string
		tags     []string
		expected int
		contains []string
	}{
		{
			name:     "match static-analysis + code-tools",
			tags:     []string{"static-analysis", "code-tools"},
			expected: 2,
			contains: []string{"lint", "complexity"},
		},
		{
			name:     "match security",
			tags:     []string{"security"},
			expected: 1,
			contains: []string{"scanner"},
		},
		{
			name:     "match code-tools broad",
			tags:     []string{"code-tools"},
			expected: 3,
			contains: []string{"lint", "complexity", "formatter"},
		},
		{
			name:     "no match",
			tags:     []string{"database"},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matched, err := tr.FindToolsByTags(tt.tags)
			if err != nil {
				t.Fatalf("FindToolsByTags failed: %v", err)
			}
			if len(matched) != tt.expected {
				names := make([]string, len(matched))
				for i, m := range matched {
					names[i] = m.Name
				}
				t.Errorf("expected %d matches, got %d: %v", tt.expected, len(matched), names)
				return
			}

			nameSet := map[string]bool{}
			for _, m := range matched {
				nameSet[m.Name] = true
			}
			for _, expected := range tt.contains {
				if !nameSet[expected] {
					t.Errorf("expected '%s' in results", expected)
				}
			}
		})
	}
}

func TestToolRegion_SaveAndLoadToolDAG(t *testing.T) {
	tr := newTestToolRegion(20009)
	tr.Cleanup()

	dag := &ToolDAG{
		Nodes: []ToolDAGNode{
			{ToolName: "lint"},
			{ToolName: "complexity"},
			{ToolName: "report"},
		},
		Edges: []ToolDAGEdge{
			{From: "lint", To: "report", Fields: []string{"violations"}},
			{From: "complexity", To: "report", Fields: []string{"metrics"}},
		},
	}

	if err := tr.SaveToolDAG(dag); err != nil {
		t.Fatalf("SaveToolDAG failed: %v", err)
	}

	loaded, err := tr.LoadToolDAG()
	if err != nil {
		t.Fatalf("LoadToolDAG failed: %v", err)
	}

	if len(loaded.Nodes) != 3 {
		t.Errorf("expected 3 nodes, got %d", len(loaded.Nodes))
	}
	if len(loaded.Edges) != 2 {
		t.Errorf("expected 2 edges, got %d", len(loaded.Edges))
	}
	if loaded.Edges[0].From != "lint" || loaded.Edges[0].To != "report" {
		t.Errorf("unexpected edge: from=%s to=%s", loaded.Edges[0].From, loaded.Edges[0].To)
	}
}

func TestToolRegion_RecordAndCompleteExec_Success(t *testing.T) {
	tr := newTestToolRegion(20010)
	tr.Cleanup()

	input := map[string]interface{}{
		"path": "/src/main.go",
	}

	seq, err := tr.RecordToolExec(42, "lint", input)
	if err != nil {
		t.Fatalf("RecordToolExec failed: %v", err)
	}
	if seq == 0 {
		t.Error("expected non-zero sequence number")
	}

	output := map[string]interface{}{
		"violations": []interface{}{"unused import"},
	}
	if err := tr.CompleteToolExec(seq, output, ""); err != nil {
		t.Fatalf("CompleteToolExec failed: %v", err)
	}

	// Check history
	history, err := tr.GetToolExecHistory("lint")
	if err != nil {
		t.Fatalf("GetToolExecHistory failed: %v", err)
	}
	if len(history) != 1 {
		t.Fatalf("expected 1 lint exec record, got %d", len(history))
	}
	if history[0].Status != ToolExecCompleted {
		t.Errorf("expected completed, got %s", history[0].Status)
	}
	if history[0].AgentID != 42 {
		t.Errorf("expected AgentID 42, got %d", history[0].AgentID)
	}
	if history[0].DoneAt == 0 {
		t.Error("expected DoneAt to be set")
	}
}

func TestToolRegion_RecordAndCompleteExec_Failure(t *testing.T) {
	tr := newTestToolRegion(20011)
	tr.Cleanup()

	seq, err := tr.RecordToolExec(42, "scanner", nil)
	if err != nil {
		t.Fatalf("RecordToolExec failed: %v", err)
	}

	if err := tr.CompleteToolExec(seq, nil, "timeout"); err != nil {
		t.Fatalf("CompleteToolExec failed: %v", err)
	}

	history, err := tr.GetToolExecHistory("scanner")
	if err != nil {
		t.Fatalf("GetToolExecHistory failed: %v", err)
	}
	if len(history) != 1 {
		t.Fatalf("expected 1 scanner exec record, got %d", len(history))
	}
	if history[0].Status != ToolExecFailed {
		t.Errorf("expected failed, got %s", history[0].Status)
	}
	if history[0].Error != "timeout" {
		t.Errorf("expected error 'timeout', got '%s'", history[0].Error)
	}
}

func TestToolRegion_SequenceIncrement(t *testing.T) {
	tr := newTestToolRegion(20012)
	tr.Cleanup()

	seq1, _ := tr.RecordToolExec(1, "lint", nil)
	seq2, _ := tr.RecordToolExec(1, "lint", nil)
	seq3, _ := tr.RecordToolExec(1, "scanner", nil)

	if seq2 != seq1+1 {
		t.Errorf("expected seq2=%d, got %d", seq1+1, seq2)
	}
	if seq3 != seq2+1 {
		t.Errorf("expected seq3=%d, got %d", seq2+1, seq3)
	}
}

func TestToolRegion_ExecHistory_FilterByTool(t *testing.T) {
	tr := newTestToolRegion(20013)
	tr.Cleanup()

	// Record multiple tools
	seq1, _ := tr.RecordToolExec(1, "lint", nil)
	seq2, _ := tr.RecordToolExec(1, "scanner", nil)
	seq3, _ := tr.RecordToolExec(1, "lint", nil)

	tr.CompleteToolExec(seq1, map[string]interface{}{"ok": true}, "")
	tr.CompleteToolExec(seq2, map[string]interface{}{"ok": true}, "")
	tr.CompleteToolExec(seq3, map[string]interface{}{"ok": true}, "")

	// lint should have 2 records
	lintHistory, err := tr.GetToolExecHistory("lint")
	if err != nil {
		t.Fatalf("GetToolExecHistory(lint) failed: %v", err)
	}
	if len(lintHistory) != 2 {
		t.Errorf("expected 2 lint records, got %d", len(lintHistory))
	}

	// scanner should have 1 record
	scanHistory, err := tr.GetToolExecHistory("scanner")
	if err != nil {
		t.Fatalf("GetToolExecHistory(scanner) failed: %v", err)
	}
	if len(scanHistory) != 1 {
		t.Errorf("expected 1 scanner record, got %d", len(scanHistory))
	}
}

func TestToolRegion_ToolDAGOverwrite(t *testing.T) {
	tr := newTestToolRegion(20014)
	tr.Cleanup()

	dag1 := &ToolDAG{
		Nodes: []ToolDAGNode{{ToolName: "A"}},
		Edges: []ToolDAGEdge{},
	}
	tr.SaveToolDAG(dag1)

	dag2 := &ToolDAG{
		Nodes: []ToolDAGNode{{ToolName: "X"}, {ToolName: "Y"}},
		Edges: []ToolDAGEdge{{From: "X", To: "Y"}},
	}
	tr.SaveToolDAG(dag2)

	loaded, err := tr.LoadToolDAG()
	if err != nil {
		t.Fatalf("LoadToolDAG failed: %v", err)
	}
	if len(loaded.Nodes) != 2 {
		t.Errorf("expected 2 nodes from overwritten DAG, got %d", len(loaded.Nodes))
	}
	if loaded.Nodes[0].ToolName != "X" {
		t.Errorf("expected first node 'X', got '%s'", loaded.Nodes[0].ToolName)
	}
}

func TestToolRegion_RegisterToolOverwrite(t *testing.T) {
	tr := newTestToolRegion(20015)
	tr.Cleanup()

	// Register lint v1
	tr.RegisterTool(&ToolDefinition{
		Name:        "lint",
		Description: "v1",
		Tags:        []string{"old"},
	})

	// Overwrite with v2
	tr.RegisterTool(&ToolDefinition{
		Name:        "lint",
		Description: "v2",
		Tags:        []string{"new", "updated"},
	})

	got, err := tr.GetTool("lint")
	if err != nil {
		t.Fatalf("GetTool failed: %v", err)
	}
	if got.Description != "v2" {
		t.Errorf("expected description 'v2', got '%s'", got.Description)
	}
	if len(got.Tags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(got.Tags))
	}

	// List should still show only 1 tool (overwritten, not duplicated)
	tools, _ := tr.ListTools()
	if len(tools) != 1 {
		t.Errorf("expected 1 tool after overwrite, got %d", len(tools))
	}
}
