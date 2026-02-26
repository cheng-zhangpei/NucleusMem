package agent

type TaskType string

const (
	TaskTypeTempChat  = "temp_chat"
	TaskTypeChat      = "chat"
	TaskTypeComm      = "comm"
	TaskTypeTool      = "tool"
	TaskTypeDecompose = "decompose"
)

type AgentTask struct {
	ID        string
	Type      string
	Content   string
	Key       string
	ToolName  string
	Params    map[string]interface{}
	ParentID  string
	Timestamp int64

	// New fields for decompose task
	AvailableTools   []string `json:"available_tools,omitempty"`
	AvailableMemTags []string `json:"available_mem_tags,omitempty"`
	MaxRetry         int      `json:"max_retry,omitempty"`
}
type TaskResult struct {
	Result         string
	Error          error
	TaskDefinition interface{} `json:"-"` // *viewspace.TaskDefinition

	Done chan struct{}
}
