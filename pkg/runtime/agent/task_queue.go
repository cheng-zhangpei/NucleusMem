package agent

type TaskType string

const (
	TaskTypeComm     TaskType = "comm"
	TaskTypeTempChat TaskType = "temp_chat"
	TaskTypeChat     TaskType = "chat"
)

type AgentTask struct {
	Type       TaskType
	Key        string
	Content    string
	Timestamp  int64
	MemSpaceId uint64 // only work when communication
}
