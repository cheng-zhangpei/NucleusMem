// pkg/runtime/memspace/region/tool_region.go
package memspace_region

import (
	"NucleusMem/pkg/configs"
	"NucleusMem/pkg/storage"
	tinykv_client "NucleusMem/pkg/storage/tinykv-client"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

const ToolSeqKey = "tool_seq"
const ToolDAGKey = "tool/dag"

// ToolDefinition describes a single tool that agents can invoke

// ToolExecStatus tracks the execution state of a tool call

// ToolExecRecord records one execution of a tool

// ToolDAG describes dependency ordering between tools within an Atomic ViewSpace
// Reuses the same DAG structure as TaskRegion for consistency

type ToolRegion struct {
	memSpaceID uint64
	seq        uint64
	mu         sync.RWMutex
	kvClient   *tinykv_client.MemClient
}

func NewToolRegion(kvClient *tinykv_client.MemClient, memSpaceID uint64) *ToolRegion {
	tr := &ToolRegion{
		kvClient:   kvClient,
		memSpaceID: memSpaceID,
	}
	if seq, err := tr.loadSeq(); err == nil {
		tr.seq = seq
	} else {
		tr.seq = 1
	}
	return tr
}

// ============================================================
// Tool Definition Management
// ============================================================

func (tr *ToolRegion) toolDefKey(toolName string) []byte {
	userKey := []byte(fmt.Sprintf("tool/def/%s", toolName))
	return configs.EncodeKey(configs.ZoneTool, tr.memSpaceID, userKey)
}

// RegisterTool stores a tool definition
func (tr *ToolRegion) RegisterTool(tool *configs.ToolDefinition) error {
	if tool.Name == "" {
		return fmt.Errorf("tool name cannot be empty")
	}
	if tool.CreatedAt == 0 {
		tool.CreatedAt = time.Now().Unix()
	}

	data, err := json.Marshal(tool)
	if err != nil {
		return fmt.Errorf("failed to marshal tool definition: %w", err)
	}

	rawKey := tr.toolDefKey(tool.Name)
	return tr.kvClient.Update(func(txn storage.Transaction) error {
		return txn.Put(rawKey, data)
	})
}

// GetTool retrieves a single tool definition by name
func (tr *ToolRegion) GetTool(toolName string) (*configs.ToolDefinition, error) {
	rawKey := tr.toolDefKey(toolName)
	var tool configs.ToolDefinition

	err := tr.kvClient.Update(func(txn storage.Transaction) error {
		data, err := txn.Get(rawKey)
		if err != nil {
			return err
		}
		return json.Unmarshal(data, &tool)
	})
	if err != nil {
		return nil, fmt.Errorf("tool '%s' not found: %w", toolName, err)
	}
	return &tool, nil
}

// GetTools retrieves multiple tool definitions by name
func (tr *ToolRegion) GetTools(toolNames []string) ([]*configs.ToolDefinition, error) {
	var tools []*configs.ToolDefinition
	for _, name := range toolNames {
		tool, err := tr.GetTool(name)
		if err != nil {
			return nil, fmt.Errorf("failed to get tool '%s': %w", name, err)
		}
		tools = append(tools, tool)
	}
	return tools, nil
}

// ListTools returns all tool definitions in this MemSpace
func (tr *ToolRegion) ListTools() ([]*configs.ToolDefinition, error) {
	prefix := configs.EncodeKey(configs.ZoneTool, tr.memSpaceID, []byte("tool/def/"))
	var tools []*configs.ToolDefinition

	err := tr.kvClient.Update(func(txn storage.Transaction) error {
		kvPairs, err := txn.Scan(prefix)
		if err != nil {
			return err
		}
		for _, pair := range kvPairs {
			var tool configs.ToolDefinition
			if err := json.Unmarshal(pair.Value, &tool); err != nil {
				continue
			}
			tools = append(tools, &tool)
		}
		return nil
	})
	return tools, err
}

// DeleteTool removes a tool definition
func (tr *ToolRegion) DeleteTool(toolName string) error {
	rawKey := tr.toolDefKey(toolName)
	return tr.kvClient.Update(func(txn storage.Transaction) error {
		return txn.Delete(rawKey)
	})
}

// FindToolsByTags returns tools matching all given tags
func (tr *ToolRegion) FindToolsByTags(tags []string) ([]*configs.ToolDefinition, error) {
	allTools, err := tr.ListTools()
	if err != nil {
		return nil, err
	}

	tagSet := make(map[string]bool, len(tags))
	for _, t := range tags {
		tagSet[t] = true
	}

	var matched []*configs.ToolDefinition
	for _, tool := range allTools {
		allMatch := true
		for _, required := range tags {
			found := false
			for _, toolTag := range tool.Tags {
				if toolTag == required {
					found = true
					break
				}
			}
			if !found {
				allMatch = false
				break
			}
		}
		if allMatch {
			matched = append(matched, tool)
		}
	}
	return matched, nil
}

// ============================================================
// Tool DAG Management
// ============================================================

// SaveToolDAG persists the tool dependency graph
func (tr *ToolRegion) SaveToolDAG(dag *configs.ToolDAG) error {
	data, err := json.Marshal(dag)
	if err != nil {
		return fmt.Errorf("failed to marshal tool DAG: %w", err)
	}

	rawKey := configs.EncodeKey(configs.ZoneTool, tr.memSpaceID, []byte(ToolDAGKey))
	return tr.kvClient.Update(func(txn storage.Transaction) error {
		return txn.Put(rawKey, data)
	})
}

// LoadToolDAG retrieves the tool dependency graph
func (tr *ToolRegion) LoadToolDAG() (*configs.ToolDAG, error) {
	rawKey := configs.EncodeKey(configs.ZoneTool, tr.memSpaceID, []byte(ToolDAGKey))
	var dag configs.ToolDAG

	err := tr.kvClient.Update(func(txn storage.Transaction) error {
		data, err := txn.Get(rawKey)
		if err != nil {
			return err
		}
		return json.Unmarshal(data, &dag)
	})
	if err != nil {
		return nil, err
	}
	return &dag, nil
}

// ============================================================
// Tool Execution Records
// ============================================================

// RecordToolExec logs a tool execution attempt
//func (tr *ToolRegion) RecordToolExec(agentID uint64, toolName string, input map[string]interface{}) (uint64, error) {
//	tr.mu.Lock()
//	seq := tr.seq
//	tr.seq++
//	err := tr.saveSeq(tr.seq)
//	tr.mu.Unlock()
//
//	if err != nil {
//		return 0, fmt.Errorf("failed to save tool exec seq: %w", err)
//	}
//
//	record := &ToolExecRecord{
//		Seq:       seq,
//		ToolName:  toolName,
//		AgentID:   agentID,
//		Input:     input,
//		Status:    ToolExecRunning,
//		StartedAt: time.Now().Unix(),
//	}
//
//	data, err := json.Marshal(record)
//	if err != nil {
//		return 0, fmt.Errorf("failed to marshal exec record: %w", err)
//	}
//
//	execKey := fmt.Sprintf("tool/exec/%d/status", seq)
//	rawKey := configs.EncodeKey(configs.ZoneTool, tr.memSpaceID, []byte(execKey))
//
//	err = tr.kvClient.Update(func(txn storage.Transaction) error {
//		return txn.Put(rawKey, data)
//	})
//	return seq, err
//}

// CompleteToolExec marks a tool execution as completed with result
func (tr *ToolRegion) CompleteToolExec(seq uint64, output map[string]interface{}, errMsg string) error {
	execKey := fmt.Sprintf("tool/exec/%d/status", seq)
	rawKey := configs.EncodeKey(configs.ZoneTool, tr.memSpaceID, []byte(execKey))

	// Read existing record
	var record configs.ToolExecRecord
	err := tr.kvClient.Update(func(txn storage.Transaction) error {
		data, err := txn.Get(rawKey)
		if err != nil {
			return err
		}
		return json.Unmarshal(data, &record)
	})
	if err != nil {
		return fmt.Errorf("failed to read exec record: %w", err)
	}

	// Update
	record.DoneAt = time.Now().Unix()
	record.Output = output
	if errMsg != "" {
		record.Status = configs.ToolExecFailed
		record.Error = errMsg
	} else {
		record.Status = configs.ToolExecCompleted
	}

	data, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("failed to marshal updated exec record: %w", err)
	}

	return tr.kvClient.Update(func(txn storage.Transaction) error {
		return txn.Put(rawKey, data)
	})
}

// pkg/runtime/memspace/region/tool_region.go

func (tr *ToolRegion) GetToolExecHistory(toolName string) ([]*configs.ToolExecRecord, error) {
	prefix := configs.EncodeKey(configs.ZoneTool, tr.memSpaceID, []byte("tool/exec/"))
	var records []*configs.ToolExecRecord

	err := tr.kvClient.Update(func(txn storage.Transaction) error {
		kvPairs, err := txn.Scan(prefix)
		if err != nil {
			return err
		}
		for _, pair := range kvPairs {
			var record configs.ToolExecRecord
			// Ignore unmarshal errors for non-record keys (e.g., seq keys)
			if err := json.Unmarshal(pair.Value, &record); err != nil {
				continue
			}
			if record.ToolName == toolName {
				records = append(records, &record)
			}
		}
		return nil
	})

	// Return empty slice instead of error if scan fails
	if err != nil {
		return []*configs.ToolExecRecord{}, nil
	}
	return records, nil
}

// ============================================================
// Helpers
// ============================================================

func (tr *ToolRegion) loadSeq() (uint64, error) {
	rawKey := configs.EncodeKey(configs.ZoneTool, tr.memSpaceID, []byte(ToolSeqKey))
	var seq uint64 = 1
	err := tr.kvClient.Update(func(txn storage.Transaction) error {
		data, err := txn.Get(rawKey)
		if err != nil {
			return nil
		}
		if len(data) < 8 {
			return nil
		}
		seq = binary.LittleEndian.Uint64(data)
		return nil
	})
	return seq, err
}

func (tr *ToolRegion) saveSeq(seq uint64) error {
	rawKey := configs.EncodeKey(configs.ZoneTool, tr.memSpaceID, []byte(ToolSeqKey))
	data := make([]byte, 8)
	binary.LittleEndian.PutUint64(data, seq)
	return tr.kvClient.Update(func(txn storage.Transaction) error {
		return txn.Put(rawKey, data)
	})
}

// Add to tool_region.go

// Cleanup deletes all data in this ToolRegion (for testing only)
func (tr *ToolRegion) Cleanup() error {
	prefix := configs.EncodeKey(configs.ZoneTool, tr.memSpaceID, []byte(""))
	return tr.kvClient.Update(func(txn storage.Transaction) error {
		kvPairs, err := txn.Scan(prefix)
		if err != nil {
			return err
		}
		for _, pair := range kvPairs {
			if err := txn.Delete(pair.Key); err != nil {
				return err
			}
		}
		return nil
	})
}
func (tr *ToolRegion) RecordToolExec(agentID uint64, toolName string, input map[string]interface{}) (uint64, error) {
	tr.mu.Lock()
	seq := tr.seq
	tr.seq++
	if err := tr.saveSeq(tr.seq); err != nil {
		tr.mu.Unlock()
		return 0, fmt.Errorf("failed to save tool exec seq: %w", err)
	}
	tr.mu.Unlock()

	record := &configs.ToolExecRecord{
		Seq:       seq,
		ToolName:  toolName,
		AgentID:   agentID,
		Input:     input,
		Status:    configs.ToolExecRunning,
		StartedAt: time.Now().Unix(),
	}

	data, err := json.Marshal(record)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal exec record: %w", err)
	}

	execKey := fmt.Sprintf("tool/exec/%d/status", seq)
	rawKey := configs.EncodeKey(configs.ZoneTool, tr.memSpaceID, []byte(execKey))

	return seq, tr.kvClient.Update(func(txn storage.Transaction) error {
		return txn.Put(rawKey, data)
	})
}

// RecordToolExecBatch persists multiple tool execution results in a single transaction
// Useful for recording an entire DAG's results atomically
func (tr *ToolRegion) RecordToolExecBatch(results map[string]*configs.ToolExecResult) error {
	return tr.kvClient.Update(func(txn storage.Transaction) error {
		for toolName, result := range results {
			// For batch mode, we use toolName as identifier since seq might not be tracked
			execKey := fmt.Sprintf("tool/exec/batch/%s", toolName)
			rawKey := configs.EncodeKey(configs.ZoneTool, tr.memSpaceID, []byte(execKey))

			record := configs.ToolExecRecord{
				ToolName: toolName,
				Output:   result.Output,
				Status:   configs.ToolExecStatus(result.Status),
				Error:    result.Error,
				DoneAt:   result.DoneAt,
			}

			data, err := json.Marshal(record)
			if err != nil {
				return fmt.Errorf("failed to marshal record for %s: %w", toolName, err)
			}

			if err := txn.Put(rawKey, data); err != nil {
				return fmt.Errorf("failed to write record for %s: %w", toolName, err)
			}
		}
		return nil
	})
}
