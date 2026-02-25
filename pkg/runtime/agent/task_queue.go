package agent

type TaskType string

const (
	TaskTypeTempChat = "temp_chat"
	TaskTypeChat     = "chat"
	TaskTypeComm     = "comm"
	TaskTypeTool     = "tool"
)

type AgentTask struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"`
	Key       string                 `json:"key,omitempty"`
	Content   string                 `json:"content,omitempty"`
	ToolName  string                 `json:"tool_name,omitempty"`
	Params    map[string]interface{} `json:"params,omitempty"`
	ParentID  string                 `json:"parent_id,omitempty"`
	Timestamp int64                  `json:"timestamp"`
}
type TaskResult struct {
	Result string
	Error  error
	Done   chan struct{}
}
