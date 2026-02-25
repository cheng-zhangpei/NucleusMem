// pkg/runtime/memspace/region/task_region.go

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

const TaskSeqKey = "task_seq"
const TaskDAGKey = "task/dag"

// TaskNodeStatus tracks the execution state of a single node in the DAG
type TaskNodeStatus string

const (
	TaskNodePending   TaskNodeStatus = "pending"
	TaskNodeRunning   TaskNodeStatus = "running"
	TaskNodeCompleted TaskNodeStatus = "completed"
	TaskNodeFailed    TaskNodeStatus = "failed"
)

// TaskDAG represents the dependency graph stored in Task Region
// For Global/Process: nodes are child ViewSpace names
// The same structure, different granularity
type TaskDAG struct {
	Nodes []TaskDAGNode `json:"nodes"`
	Edges []TaskDAGEdge `json:"edges"`
}

type TaskDAGNode struct {
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

type TaskDAGEdge struct {
	From   string   `json:"from"`
	To     string   `json:"to"`
	Fields []string `json:"fields,omitempty"` // dataflow fields carried on this edge
}

// TaskNodeState is the persisted state of a single DAG node
type TaskNodeState struct {
	Name      string                 `json:"name"`
	Status    TaskNodeStatus         `json:"status"`
	StartedAt int64                  `json:"started_at,omitempty"`
	DoneAt    int64                  `json:"done_at,omitempty"`
	Result    map[string]interface{} `json:"result,omitempty"`
	Error     string                 `json:"error,omitempty"`
}

// TaskAuditEntry records a single auditable event
type TaskAuditEntry struct {
	Seq       uint64 `json:"seq"`
	Timestamp int64  `json:"timestamp"`
	AgentID   uint64 `json:"agent_id"`
	Action    string `json:"action"`    // "task_start", "task_complete", "tool_call", "decision"
	NodeName  string `json:"node_name"` // which ViewSpace or tool
	Detail    string `json:"detail"`    // free-form description or JSON blob
}

type TaskRegion struct {
	memSpaceID uint64
	seq        uint64
	mu         sync.RWMutex
	kvClient   *tinykv_client.MemClient
}

func NewTaskRegion(kvClient *tinykv_client.MemClient, memSpaceID uint64) *TaskRegion {
	tr := &TaskRegion{
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
// DAG Management
// ============================================================

// SaveDAG persists the dependency graph for this ViewSpace's children
func (tr *TaskRegion) SaveDAG(dag *TaskDAG) error {
	data, err := json.Marshal(dag)
	if err != nil {
		return fmt.Errorf("failed to marshal task DAG: %w", err)
	}
	// only one info in the
	rawKey := configs.EncodeKey(configs.ZoneTask, tr.memSpaceID, []byte(TaskDAGKey))
	return tr.kvClient.Update(func(txn storage.Transaction) error {
		return txn.Put(rawKey, data)
	})
}

// LoadDAG retrieves the persisted dependency graph
func (tr *TaskRegion) LoadDAG() (*TaskDAG, error) {
	rawKey := configs.EncodeKey(configs.ZoneTask, tr.memSpaceID, []byte(TaskDAGKey))
	var dag TaskDAG

	err := tr.kvClient.Update(func(txn storage.Transaction) error {
		data, err := txn.Get(rawKey)
		if err != nil {
			return err
		}
		return json.Unmarshal(data, &dag)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to load task DAG: %w", err)
	}
	return &dag, nil
}

// ============================================================
// Node State Management
// ============================================================

func (tr *TaskRegion) nodeStatusKey(nodeName string) []byte {
	userKey := []byte(fmt.Sprintf("task/node/%s/status", nodeName))
	return configs.EncodeKey(configs.ZoneTask, tr.memSpaceID, userKey)
}

func (tr *TaskRegion) nodeResultKey(nodeName string) []byte {
	userKey := []byte(fmt.Sprintf("task/node/%s/result", nodeName))
	return configs.EncodeKey(configs.ZoneTask, tr.memSpaceID, userKey)
}

// UpdateNodeStatus sets the execution status of a DAG node(record the detail of the task(ViewSpace) execute status)
func (tr *TaskRegion) UpdateNodeStatus(nodeName string, status TaskNodeStatus) error {
	state := &TaskNodeState{
		Name:   nodeName,
		Status: status,
	}
	switch status {
	case TaskNodeRunning:
		state.StartedAt = time.Now().Unix()
	case TaskNodeCompleted, TaskNodeFailed:
		state.DoneAt = time.Now().Unix()
	}

	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("failed to marshal node state: %w", err)
	}

	rawKey := tr.nodeStatusKey(nodeName)
	return tr.kvClient.Update(func(txn storage.Transaction) error {
		return txn.Put(rawKey, data)
	})
}

// UpdateNodeResult saves the execution result of a completed DAG node
func (tr *TaskRegion) UpdateNodeResult(nodeName string, result map[string]interface{}, errMsg string) error {
	state := &TaskNodeState{
		Name:   nodeName,
		DoneAt: time.Now().Unix(),
		Result: result,
		Error:  errMsg,
	}
	if errMsg != "" {
		state.Status = TaskNodeFailed
	} else {
		state.Status = TaskNodeCompleted
	}

	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("failed to marshal node result: %w", err)
	}

	// Write both status and result
	return tr.kvClient.Update(func(txn storage.Transaction) error {
		if err := txn.Put(tr.nodeStatusKey(nodeName), data); err != nil {
			return err
		}
		resultData, _ := json.Marshal(result)
		return txn.Put(tr.nodeResultKey(nodeName), resultData)
	})
}

// GetNodeState retrieves the current state of a DAG node
func (tr *TaskRegion) GetNodeState(nodeName string) (*TaskNodeState, error) {
	rawKey := tr.nodeStatusKey(nodeName)
	var state TaskNodeState

	err := tr.kvClient.Update(func(txn storage.Transaction) error {
		data, err := txn.Get(rawKey)
		if err != nil {
			return err
		}
		return json.Unmarshal(data, &state)
	})
	if err != nil {
		return nil, err
	}
	return &state, nil
}

// GetNodeResult retrieves the output of a completed node
func (tr *TaskRegion) GetNodeResult(nodeName string) (map[string]interface{}, error) {
	rawKey := tr.nodeResultKey(nodeName)
	var result map[string]interface{}

	err := tr.kvClient.Update(func(txn storage.Transaction) error {
		data, err := txn.Get(rawKey)
		if err != nil {
			return err
		}
		return json.Unmarshal(data, &result)
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// GetAllNodeStates returns the state of all nodes in the DAG
func (tr *TaskRegion) GetAllNodeStates() (map[string]*TaskNodeState, error) {
	prefix := configs.EncodeKey(configs.ZoneTask, tr.memSpaceID, []byte("task/node/"))
	states := make(map[string]*TaskNodeState)

	err := tr.kvClient.Update(func(txn storage.Transaction) error {
		kvPairs, err := txn.Scan(prefix)
		if err != nil {
			return err
		}
		for _, pair := range kvPairs {
			_, _, userKey, err := configs.DecodeKey(pair.Key)
			if err != nil {
				continue
			}
			keyStr := string(userKey)
			// Only process status keys, not result keys
			if len(keyStr) > len("task/node/") && keyStr[len(keyStr)-7:] == "/status" {
				var state TaskNodeState
				if err := json.Unmarshal(pair.Value, &state); err == nil {
					states[state.Name] = &state
				}
			}
		}
		return nil
	})
	return states, err
}

// ============================================================
// Audit Log (used by Global ViewSpace)
// ============================================================

// WriteAuditEntry appends an audit log entry
func (tr *TaskRegion) WriteAuditEntry(agentID uint64, action, nodeName, detail string) error {
	tr.mu.Lock()
	seq := tr.seq
	tr.seq++
	err := tr.saveSeq(tr.seq)
	tr.mu.Unlock()

	if err != nil {
		return fmt.Errorf("failed to save audit seq: %w", err)
	}

	entry := &TaskAuditEntry{
		Seq:       seq,
		Timestamp: time.Now().Unix(),
		AgentID:   agentID,
		Action:    action,
		NodeName:  nodeName,
		Detail:    detail,
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal audit entry: %w", err)
	}

	auditKey := fmt.Sprintf("task/audit/%d", seq)
	rawKey := configs.EncodeKey(configs.ZoneTask, tr.memSpaceID, []byte(auditKey))

	return tr.kvClient.Update(func(txn storage.Transaction) error {
		return txn.Put(rawKey, data)
	})
}

// GetAuditEntries retrieves audit log entries, optionally filtered by time range
func (tr *TaskRegion) GetAuditEntries(afterTimestamp int64) ([]*TaskAuditEntry, error) {
	prefix := configs.EncodeKey(configs.ZoneTask, tr.memSpaceID, []byte("task/audit/"))
	var entries []*TaskAuditEntry

	err := tr.kvClient.Update(func(txn storage.Transaction) error {
		kvPairs, err := txn.Scan(prefix)
		if err != nil {
			return err
		}
		for _, pair := range kvPairs {
			var entry TaskAuditEntry
			if err := json.Unmarshal(pair.Value, &entry); err != nil {
				continue
			}
			if entry.Timestamp >= afterTimestamp {
				entries = append(entries, &entry)
			}
		}
		return nil
	})
	return entries, err
}

// ============================================================
// Helpers => get the info of the task quickly
// ============================================================

// IsAllCompleted checks if all nodes in the DAG have completed
func (tr *TaskRegion) IsAllCompleted() (bool, error) {
	states, err := tr.GetAllNodeStates()
	if err != nil {
		return false, err
	}

	dag, err := tr.LoadDAG()
	if err != nil {
		return false, err
	}

	for _, node := range dag.Nodes {
		state, ok := states[node.Name]
		if !ok || state.Status != TaskNodeCompleted {
			return false, nil
		}
	}
	return true, nil
}

// GetReadyNodes returns nodes whose dependencies are all completed
func (tr *TaskRegion) GetReadyNodes() ([]string, error) {
	dag, err := tr.LoadDAG()
	if err != nil {
		return nil, err
	}

	states, err := tr.GetAllNodeStates()
	if err != nil {
		return nil, err
	}

	// Build dependency map: node -> list of nodes it depends on
	deps := make(map[string][]string)
	for _, node := range dag.Nodes {
		deps[node.Name] = []string{}
	}
	for _, edge := range dag.Edges {
		deps[edge.To] = append(deps[edge.To], edge.From)
	}

	var ready []string
	for _, node := range dag.Nodes {
		state, exists := states[node.Name]
		// Skip nodes that are already running, completed, or failed
		if exists && state.Status != TaskNodePending {
			continue
		}

		// Check if all dependencies completed
		allDepsComplete := true
		for _, dep := range deps[node.Name] {
			depState, depExists := states[dep]
			if !depExists || depState.Status != TaskNodeCompleted {
				allDepsComplete = false
				break
			}
		}

		if allDepsComplete {
			ready = append(ready, node.Name)
		}
	}

	return ready, nil
}

func (tr *TaskRegion) loadSeq() (uint64, error) {
	rawKey := configs.EncodeKey(configs.ZoneTask, tr.memSpaceID, []byte(TaskSeqKey))
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

func (tr *TaskRegion) saveSeq(seq uint64) error {
	rawKey := configs.EncodeKey(configs.ZoneTask, tr.memSpaceID, []byte(TaskSeqKey))
	data := make([]byte, 8)
	binary.LittleEndian.PutUint64(data, seq)
	return tr.kvClient.Update(func(txn storage.Transaction) error {
		return txn.Put(rawKey, data)
	})
}

// Cleanup deletes all data in this TaskRegion (for testing only)
func (tr *TaskRegion) Cleanup() error {
	prefix := configs.EncodeKey(configs.ZoneTask, tr.memSpaceID, []byte(""))
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
