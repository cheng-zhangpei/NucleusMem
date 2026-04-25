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
)

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
}
type TaskResult struct {
	Result         string
	Error          error
	TaskDefinition interface{} `json:"-"` // *viewspace.TaskDefinition

	Done chan struct{}
}
