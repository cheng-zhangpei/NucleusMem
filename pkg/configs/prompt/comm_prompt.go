package prompt

import (
	"encoding/json"
	"fmt"
	"time"
)

type CommPrompt struct {
	Version   string   `json:"version"`
	Timestamp int64    `json:"timestamp"`
	FromAgent uint64   `json:"from_agent"`
	MemoryKey string   `json:"memory_key,omitempty"` // 主记忆 Key（可选）
	RefKeys   []string `json:"ref_keys"`             // 引用的 Key 列表
	//RefTypes   []string          `json:"ref_types"`            // 对应每个 Key 的类型
	//Metadata   map[string]string `json:"metadata,omitempty"`
}

// NewCommPrompt creates a new communication prompt
func NewCommPrompt(fromAgent uint64, memoryKey string, refKeys []string) *CommPrompt {
	//if metadata == nil {
	//	metadata = make(map[string]string)
	//}

	// Validate refKeys and refTypes length match
	//if len(refKeys) != len(refTypes) {
	//	panic("refKeys and refTypes must have same length")
	//}
	//
	//// Validate refTypes
	//for _, rt := range refTypes {
	//	if rt != "memory" && rt != "summary" {
	//		panic("ref_type must be 'memory' or 'summary'")
	//	}
	//}

	return &CommPrompt{
		Version:   "v1",
		Timestamp: time.Now().Unix(),
		FromAgent: fromAgent,
		MemoryKey: memoryKey,
		RefKeys:   refKeys,
		//RefTypes:  refTypes,
		//Metadata:  metadata,
	}
}

// Encode serializes to JSON string
func (p *CommPrompt) Encode() (string, error) {
	data, err := json.Marshal(p)
	if err != nil {
		return "", fmt.Errorf("failed to encode comm prompt: %w", err)
	}
	return string(data), nil
}

// DecodeCommPrompt deserializes from JSON string
func DecodeCommPrompt(jsonStr string) (*CommPrompt, error) {
	var p CommPrompt
	err := json.Unmarshal([]byte(jsonStr), &p)
	if err != nil {
		return nil, fmt.Errorf("failed to decode comm prompt: %w", err)
	}

	// Validate refKeys and refTypes length
	//if len(p.RefKeys) != len(p.RefTypes) {
	//	return nil, fmt.Errorf("ref_keys and ref_types length mismatch")
	//}
	//
	//// Validate refTypes
	//for _, rt := range p.RefTypes {
	//	if rt != "memory" && rt != "summary" {
	//		return nil, fmt.Errorf("invalid ref_type: must be 'memory' or 'summary', got '%s'", rt)
	//	}
	//}

	return &p, nil
}
