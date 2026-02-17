package prompt

import (
	"encoding/json"
	"fmt"
	"time"
)

type ChatPrompt struct {
	Type        string        `json:"type"`
	Version     string        `json:"version"`
	Timestamp   int64         `json:"timestamp"`
	System      string        `json:"system"`
	Summary     string        `json:"summary"`      // 来自 SummaryRegion 的压缩记忆
	TempHistory []ChatMessage `json:"temp_history"` // 短期对话历史
	UserInput   string        `json:"user_input"`
}

// NewChatPrompt creates a new chat prompt with memory context
func NewChatPrompt(system, summary, userInput string, tempHistory []ChatMessage) *ChatPrompt {
	return &ChatPrompt{
		Type:        "chat",
		Version:     "v1",
		Timestamp:   time.Now().Unix(),
		System:      system,
		Summary:     summary,
		TempHistory: tempHistory,
		UserInput:   userInput,
	}
}

// Encode serializes to JSON string
func (p *ChatPrompt) Encode() (string, error) {
	data, err := json.Marshal(p)
	if err != nil {
		return "", fmt.Errorf("failed to encode chat prompt: %w", err)
	}
	return string(data), nil
}

// DecodeChatPrompt deserializes from JSON string
func DecodeChatPrompt(jsonStr string) (*ChatPrompt, error) {
	var p ChatPrompt
	err := json.Unmarshal([]byte(jsonStr), &p)
	if err != nil {
		return nil, fmt.Errorf("failed to decode chat prompt: %w", err)
	}
	if p.Type != "chat" {
		return nil, fmt.Errorf("invalid prompt type: expected 'chat', got '%s'", p.Type)
	}
	return &p, nil
}
