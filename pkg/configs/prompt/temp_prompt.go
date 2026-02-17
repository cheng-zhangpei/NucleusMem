package prompt

import (
	"NucleusMem/pkg/client"
	"encoding/json"
	"fmt"
	"time"
)

type TempChatPrompt struct {
	Type      string               `json:"type"`
	Version   string               `json:"version"`
	Timestamp int64                `json:"timestamp"`
	System    string               `json:"system"`
	Messages  []client.ChatMessage `json:"messages"`
	UserInput string               `json:"user_input"`
}

// NewTempChatPrompt creates a new temp chat prompt
func NewTempChatPrompt(system, userInput string, history []client.ChatMessage) *TempChatPrompt {
	return &TempChatPrompt{
		Type:      "temp_chat",
		Version:   "v1",
		Timestamp: time.Now().Unix(),
		System:    system,
		Messages:  history,
		UserInput: userInput,
	}
}

// Encode serializes the prompt to JSON string
func (p *TempChatPrompt) Encode() (string, error) {
	data, err := json.Marshal(p)
	if err != nil {
		return "", fmt.Errorf("failed to encode temp chat prompt: %w", err)
	}
	return string(data), nil
}

// DecodeTempChatPrompt deserializes JSON string to TempChatPrompt
func DecodeTempChatPrompt(jsonStr string) (*TempChatPrompt, error) {
	var p TempChatPrompt
	err := json.Unmarshal([]byte(jsonStr), &p)
	if err != nil {
		return nil, fmt.Errorf("failed to decode temp chat prompt: %w", err)
	}
	if p.Type != "temp_chat" {
		return nil, fmt.Errorf("invalid prompt type: expected 'temp_chat', got '%s'", p.Type)
	}
	return &p, nil
}
