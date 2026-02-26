// pkg/configs/prompt/task_decompose_prompt.go

package prompt

import (
	"encoding/json"
	"fmt"
	"time"
)

// TaskDecomposePrompt generates a prompt that asks LLM to decompose a task
// into a ViewSpace Tree
type TaskDecomposePrompt struct {
	Type        string `json:"type"`
	Version     string `json:"version"`
	Timestamp   int64  `json:"timestamp"`
	System      string `json:"system"`
	UserRequest string `json:"user_request"`
	// Available context for decomposition
	AvailableTools   []string `json:"available_tools,omitempty"`
	AvailableAgents  []string `json:"available_agents,omitempty"`
	AvailableMemTags []string `json:"available_mem_tags,omitempty"`
}

func NewTaskDecomposePrompt(userRequest string, availableTools, availableAgents, availableMemTags []string) *TaskDecomposePrompt {
	system := `You are a task decomposition engine. Your job is to break down a complex task into a ViewSpace Tree.

## What is a ViewSpace Tree
A ViewSpace Tree is a hierarchical structure where:
- Each node is a "ViewSpace" representing a sub-task
- There are 3 types of ViewSpace:
  - "global": The root node. Coordinates everything and collects final results. There is exactly ONE global node.
  - "process": Intermediate coordination nodes. They decompose tasks further and aggregate results from children.
  - "atomic": Leaf nodes that do actual work by calling tools.
- For simple tasks that only need 1-2 tools, keep the structure minimal:
  just one global + one or two atomic nodes. Do NOT over-decompose.
## Output Format
You MUST respond with a single valid JSON object containing exactly 3 sections:

{
  "meta": {
    "task_id": "a short kebab-case id for this task",
    "description": "one-line summary of the overall task"
  },
  "viewspaces": [
    {
      "name": "unique-kebab-case-name",
      "type": "global | process | atomic",
      "tags": ["relevant", "tags"],
      "description": "what this viewspace does",
      "role": "the agent role needed for this viewspace",
      "tools": ["tool1", "tool2"]  // only for atomic nodes, omit for others
    }
  ],
  "dependencies": {
    "tree": [
      {"parent": "parent-name", "children": ["child1", "child2"]}
    ],
    "dataflow": [
      {"from": "source-name", "to": "target-name", "fields": ["field1"], "description": "what data flows"}
    ]
  }
}

## Rules
1. There must be exactly ONE node with type "global"
2. "atomic" nodes CANNOT have children in the tree
3. "global" and "process" nodes MUST have at least one child
4. All names must be unique across the entire tree
5. dataflow "from" and "to" must reference existing viewspace names
6. dataflow cannot have circular dependencies
7. Only "atomic" nodes should have "tools" field
8. Keep decomposition practical - don't over-split simple tasks
9. An atomic node should be created when:
   - The task can be done by calling one or a few tools
   - The task requires only a single agent role
   - No further sub-task coordination is needed

## Available Resources
`

	if len(availableTools) > 0 {
		system += "\nAvailable Tools:\n"
		for _, t := range availableTools {
			system += fmt.Sprintf("- %s\n", t)
		}
	} else {
		system += "\nAvailable Tools: none specified (you may assume general-purpose tools)\n"
	}

	if len(availableMemTags) > 0 {
		system += "\nAvailable MemSpace Tags (use these in your tags for matching):\n"
		for _, t := range availableMemTags {
			system += fmt.Sprintf("- %s\n", t)
		}
	}

	return &TaskDecomposePrompt{
		Type:             "task_decompose",
		Version:          "v1",
		Timestamp:        time.Now().Unix(),
		System:           system,
		UserRequest:      userRequest,
		AvailableTools:   availableTools,
		AvailableAgents:  availableAgents,
		AvailableMemTags: availableMemTags,
	}
}

// BuildMessages converts the prompt into ChatMessages for the LLM client
func (p *TaskDecomposePrompt) BuildMessages() []map[string]string {
	return []map[string]string{
		{"role": "system", "content": p.System},
		{"role": "user", "content": p.UserRequest},
	}
}

// Encode serializes the prompt itself (for logging/debugging)
func (p *TaskDecomposePrompt) Encode() (string, error) {
	data, err := json.Marshal(p)
	if err != nil {
		return "", fmt.Errorf("failed to encode task decompose prompt: %w", err)
	}
	return string(data), nil
}
