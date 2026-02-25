// pkg/configs/prompt/chat_prompt.go

package prompt

import (
	"NucleusMem/pkg/client"
	"NucleusMem/pkg/configs"
	_ "NucleusMem/pkg/runtime/memspace/region"
	"encoding/json"
	"fmt"
	"time"
)

// ToolCallSchema defines the expected JSON output format for tool calls
type ToolCallSchema struct {
	Thought    string                 `json:"thought"`    // 模型的思考过程
	Action     string                 `json:"action"`     // "tool_call" 或 "chat_response"
	ToolName   string                 `json:"tool_name"`  // 工具名称（如果是 tool_call）
	Parameters map[string]interface{} `json:"parameters"` // 工具参数
	Response   string                 `json:"response"`   // 直接回复（如果是 chat_response）
}

type ChatPrompt struct {
	Type        string               `json:"type"`
	Version     string               `json:"version"`
	Timestamp   int64                `json:"timestamp"`
	System      string               `json:"system"`
	Summary     string               `json:"summary"`
	TempHistory []client.ChatMessage `json:"temp_history"`
	UserInput   string               `json:"user_input"`
	Tools       []ToolInfo           `json:"tools,omitempty"`
}

// ToolInfo 简化版工具信息（用于 Prompt）
type ToolInfo struct {
	Name        string              `json:"name"`
	Description string              `json:"description"`
	Parameters  []configs.ToolParam `json:"parameters"`
}

// NewChatPrompt creates a new chat prompt with memory context and tools
func NewChatPrompt(system, summary, userInput string, tempHistory []client.ChatMessage, tools []*configs.ToolDefinition) *ChatPrompt {
	// 转换工具定义为简化版
	var toolInfos []ToolInfo
	for _, tool := range tools {
		toolInfos = append(toolInfos, ToolInfo{
			Name:        tool.Name,
			Description: tool.Description,
			Parameters:  tool.Parameters,
		})
	}

	// ✅ 增强 System Prompt，引导 JSON 输出
	enhancedSystem := system + `

## Output Format
You MUST respond in valid JSON format with the following structure:
{
  "thought": "Your reasoning process here",
  "action": "tool_call" or "chat_response",
  "tool_name": "name of tool to call (only if action is tool_call)",
  "parameters": {"param1": "value1", ...} (only if action is tool_call),
  "response": "Your direct response to user (only if action is chat_response)"
}

## Tool Usage Rules
- If the user's request can be fulfilled by available tools, use "action": "tool_call"
- If no tool is needed, use "action": "chat_response"
- Always think step-by-step in the "thought" field
- Only call tools that are listed in the available tools below

## Available Tools
`
	// 添加工具列表到 System Prompt
	if len(toolInfos) > 0 {
		for _, tool := range toolInfos {
			enhancedSystem += fmt.Sprintf("\n- **%s**: %s\n  Parameters: %v",
				tool.Name, tool.Description, tool.Parameters)
		}
	} else {
		enhancedSystem += "\n- No tools available"
	}

	return &ChatPrompt{
		Type:        "chat",
		Version:     "v2", // 升级到 v2
		Timestamp:   time.Now().Unix(),
		System:      enhancedSystem,
		Summary:     summary,
		TempHistory: tempHistory,
		UserInput:   userInput,
		Tools:       toolInfos,
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

// ParseToolCallFromResponse parses LLM response to extract tool call request
func ParseToolCallFromResponse(response string) (*ToolCallSchema, error) {
	var schema ToolCallSchema
	err := json.Unmarshal([]byte(response), &schema)
	if err != nil {
		return nil, fmt.Errorf("failed to parse tool call: %w", err)
	}
	return &schema, nil
}
