package prompt

import (
	"encoding/json"
	"fmt"
	"time"
)

type SummaryPrompt struct {
	Type      string   `json:"type"`
	Version   string   `json:"version"`
	Timestamp int64    `json:"timestamp"`
	Messages  []string `json:"messages"`
	MaxWords  int      `json:"max_words"`
}

// NewSummaryPrompt creates a new summary prompt
func NewSummaryPrompt(messages []string, maxWords int) *SummaryPrompt {
	return &SummaryPrompt{
		Type:      "summary",
		Version:   "v1",
		Timestamp: time.Now().Unix(),
		Messages:  messages,
		MaxWords:  maxWords,
	}
}

// Encode serializes the prompt to JSON string
func (p *SummaryPrompt) Encode() (string, error) {
	data, err := json.Marshal(p)
	if err != nil {
		return "", fmt.Errorf("failed to encode summary prompt: %w", err)
	}
	return string(data), nil
}

// DecodeSummaryPrompt deserializes JSON string to SummaryPrompt
func DecodeSummaryPrompt(jsonStr string) (*SummaryPrompt, error) {
	var p SummaryPrompt
	err := json.Unmarshal([]byte(jsonStr), &p)
	if err != nil {
		return nil, fmt.Errorf("failed to decode summary prompt: %w", err)
	}
	if p.Type != "summary" {
		return nil, fmt.Errorf("invalid prompt type: expected 'summary', got '%s'", p.Type)
	}
	return &p, nil
}
