package agent

import "time"

// pkg/agent/temp_memory.go
type TempMemory struct {
	History []Message
}

type Message struct {
	Role      string // "user", "assistant", "system"
	Content   string
	Timestamp time.Time
}
