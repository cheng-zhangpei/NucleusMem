package agent

import "NucleusMem/pkg/configs"

type TaskType string

const (
	TaskTypeTempChat     = "temp_chat"
	TaskTypeChat         = "chat"
	TaskTypeComm         = "comm"
	TaskTypeTool         = "tool"
	TaskTypeDecompose    = "decompose"
	TaskTypeToolDAG      = "tool_dag" // New: For concurrent tool execution based on DAG
	TaskTypeStandardTool = "standard_tool"
	TaskTypeReAct        = "react"
	TaskTypeAttack       = "attack"
)

// ReActStep 记录一轮推理的完整信息
type ReActStep struct {
	Thought     string                 `json:"thought"`
	Action      string                 `json:"action"`
	ActionInput map[string]interface{} `json:"action_input"`
	Observation string                 `json:"observation"`
}

// ReActState 在迭代之间传递的状态
type ReActState struct {
	OriginalQuery string      `json:"original_query"`
	Steps         []ReActStep `json:"steps"`
	Iteration     int         `json:"iteration"`
	MaxIterations int         `json:"max_iterations"`
	ParentTaskID  string      `json:"parent_task_id"`

	KnowledgeBase   map[string]string `json:"knowledge_base"`
	FailedPaths     []string          `json:"failed_paths"`
	AttackPhase     string            `json:"attack_phase"`
	PreviousHistory string            `json:"previous_history"` // ← 新增：历史报告内容
	HistorySummary  string            `json:"history_summary"`
}

type AgentTask struct {
	ID         string
	Type       string
	Content    string
	Key        string
	ToolName   string
	Params     map[string]interface{}
	ParentID   string
	Timestamp  int64
	MemSpaceID uint64
	// New fields for decompose task
	AvailableTools   []string         `json:"available_tools,omitempty"`
	AvailableMemTags []string         `json:"available_mem_tags,omitempty"`
	MaxRetry         int              `json:"max_retry,omitempty"`
	ToolGraph        *configs.ToolDAG `json:"tool_graph,omitempty"`
	ReActState       *ReActState      `json:"react_state,omitempty"`
}
type TaskResult struct {
	Result         string
	Error          error
	TaskDefinition interface{} `json:"-"` // *viewspace.TaskDefinition

	Done chan struct{}
}
