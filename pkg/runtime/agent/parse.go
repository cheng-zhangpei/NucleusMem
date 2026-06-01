package agent

import (
	"encoding/json"
	"fmt"
	"strings"
)

func buildReActPrompt(
	query string,
	memorySummary string,
	memories []string,
	steps []ReActStep,
	iteration int,
	maxIterations int,
) string {
	var b strings.Builder
	b.WriteString("You are a reasoning agent. Use the ReAct framework:\n")
	b.WriteString("- Thought: reason about what to do next\n")
	b.WriteString("- Action: choose ONE action to take，give the tool name,you can see in the input\n")
	b.WriteString("- Action Input: parameters for the action\n")
	b.WriteString("- OR -\n")
	b.WriteString("- Final Answer: provide the final answer to the user,only the last response you think should fill this section\n\n")

	if memorySummary != "" {
		b.WriteString("## Memory Context\n")
		b.WriteString(memorySummary)
		b.WriteString("\n\n")
	}

	if len(memories) > 0 {
		b.WriteString("## Relevant Memories\n")
		for i, m := range memories {
			b.WriteString(fmt.Sprintf("%d. %s\n", i+1, m))
		}
		b.WriteString("\n")
	}

	b.WriteString(fmt.Sprintf("## Query\n%s\n\n", query))
	b.WriteString(fmt.Sprintf("## Iteration %d/%d\n\n", iteration+1, maxIterations))

	// 注入历史步骤
	if len(steps) > 0 {
		b.WriteString("## Previous Steps\n")
		for i, s := range steps {
			b.WriteString(fmt.Sprintf("### Step %d\n", i+1))
			b.WriteString(fmt.Sprintf("Thought: %s\n", s.Thought))
			b.WriteString(fmt.Sprintf("Action: %s\n", s.Action))
			b.WriteString(fmt.Sprintf("Action Input: %v\n", s.ActionInput))
			b.WriteString(fmt.Sprintf("Observation: %s\n\n", s.Observation))
		}
	}

	b.WriteString("## Your Response\n")
	b.WriteString("Respond in one of these exact formats:\n\n")
	b.WriteString("For taking an action:\n")
	b.WriteString("```\nThought: <your reasoning>\nAction: <action_name>\nAction Input: <json_parameters>\n```\n\n")
	b.WriteString("For giving the final answer:\n")
	b.WriteString("```\nThought: <your reasoning>\nFinal Answer: <your answer>\n```\n")

	return b.String()
}

type ReActParsed struct {
	IsFinalAnswer bool
	Thought       string
	FinalAnswer   string
	Action        string
	ActionInput   map[string]interface{}
}

func parseReActResponse(response string) ReActParsed {
	result := ReActParsed{}

	// 提取 Thought
	if idx := strings.Index(response, "Thought:"); idx >= 0 {
		rest := response[idx+len("Thought:"):]
		if endIdx := strings.Index(rest, "Action:"); endIdx >= 0 {
			result.Thought = strings.TrimSpace(rest[:endIdx])
		} else if endIdx := strings.Index(rest, "Final Answer:"); endIdx >= 0 {
			result.Thought = strings.TrimSpace(rest[:endIdx])
		}
	}

	// 判断是 Final Answer 还是 Action
	if idx := strings.Index(response, "Final Answer:"); idx >= 0 {
		result.IsFinalAnswer = true
		result.FinalAnswer = strings.TrimSpace(response[idx+len("Final Answer:"):])
		return result
	}

	// 提取 Action
	if idx := strings.Index(response, "Action:"); idx >= 0 {
		rest := response[idx+len("Action:"):]
		if endIdx := strings.Index(rest, "Action Input:"); endIdx >= 0 {
			result.Action = strings.TrimSpace(rest[:endIdx])
		}
	}

	// 提取 Action Input
	if idx := strings.Index(response, "Action Input:"); idx >= 0 {
		inputStr := strings.TrimSpace(response[idx+len("Action Input:"):])
		var input map[string]interface{}
		if json.Unmarshal([]byte(inputStr), &input) == nil {
			result.ActionInput = input
		} else {
			// fallback: 当作字符串 query
			result.ActionInput = map[string]interface{}{"query": inputStr}
		}
	}

	return result
}
