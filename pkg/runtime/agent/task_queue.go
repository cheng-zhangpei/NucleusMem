package agent

type TaskType string

const (
	TaskTypeComm     TaskType = "comm"
	TaskTypeTempChat TaskType = "temp_chat"
	TaskTypeChat     TaskType = "chat"
)

type AgentTask struct {
	ID         string
	Type       TaskType
	Key        string
	Content    string
	Timestamp  int64
	MemSpaceId uint64 // only work when communication
}
type TaskResult struct {
	Result string
	Error  error
	Done   chan struct{}
}
