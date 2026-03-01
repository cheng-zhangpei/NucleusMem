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
- Each node is a "ViewSpace" — a collaboration space with its own shared memory (blackboard)
- There are 3 types of ViewSpace:
  - "global": The root node. Coordinates everything and collects final results. Exactly ONE.
  - "process": Intermediate coordination nodes. Decompose tasks further and aggregate children results.
  - "atomic": Leaf nodes that do actual work by calling tools.

## Collaboration Within a ViewSpace
Each ViewSpace has:
- A **Represented Agent** (coordinator): defined by the "role" field
- Optional **Worker Agents**: additional agents that collaborate within the same ViewSpace via shared memory
- A **Primary MemSpace** (created automatically): the shared blackboard for all agents in this ViewSpace
- Optional **Mounted MemSpaces**: additional memory spaces for context inheritance or data sharing

### When to use Workers
Use workers when a task benefits from multiple perspectives working on the SAME shared context:
- A coder + reviewer iterating on code
- A researcher + fact-checker verifying claims  
- A planner + validator cross-checking a plan

Do NOT use workers when sub-tasks are independent — use tree decomposition (children) instead.

### When to mount additional MemSpaces
- "parent": Mount the parent ViewSpace's memory for context inheritance (auto-injected, usually don't need to specify)
- "new": Create a fresh isolated workspace for a specific purpose
- "tag:<name>": Connect to an existing shared knowledge base
- "id:<memspace_id>": Connect to an existing MemSpace by its numeric ID
- "name:<memspace_name>": Connect to an existing MemSpace by its name
## Output Format
Respond with a single valid JSON object:

{
  "meta": {
    "task_id": "kebab-case-id",
    "description": "one-line summary"
  },
  "viewspaces": [
    {
      "name": "unique-kebab-case-name",
      "type": "global | process | atomic",
      "tags": ["relevant", "tags"],
      "description": "what this viewspace does",
      "role": "the coordinator agent role",
      "tools": ["tool1"],
      "workers": [
        {"role": "Reviewer", "description": "Reviews output and provides feedback"}
      ],
      "mount_memspaces": [
        {"source": "parent", "purpose": "access parent context", "read_only": true},
        {"source": "new", "purpose": "scratch workspace for drafts", "read_only": false}
      ]
    }
  ],
  "dependencies": {
    "tree": [
      {"parent": "parent-name", "children": ["child1", "child2"]}
    ],
    "dataflow": [
      {"from": "source", "to": "target", "fields": ["field1"], "description": "data flow"}
    ]
  }
}

## Rules
1. Exactly ONE "global" node
2. "atomic" nodes CANNOT have children
3. "global" and "process" MUST have at least one child
4. All names must be unique
5. No circular dataflow dependencies
6. Only "atomic" nodes have "tools"
7. "workers" are optional — use them for iterative collaboration within a single space
8. "mount_memspaces" are optional — parent context is auto-inherited
9. Keep decomposition practical — don't over-split simple tasks
10. For simple tasks (1-2 tools): just global + 1-2 atomic nodes, no workers needed

## Available Resources
`

	if len(availableTools) > 0 {
		system += "\nAvailable Tools:\n"
		for _, t := range availableTools {
			system += fmt.Sprintf("- %s\n", t)
		}
	} else {
		system += "\nAvailable Tools: none specified (assume general-purpose tools)\n"
	}

	if len(availableMemTags) > 0 {
		system += "\nAvailable MemSpace Tags (for tag-based mounting):\n"
		for _, t := range availableMemTags {
			system += fmt.Sprintf("- %s\n", t)
		}
	}

	return &TaskDecomposePrompt{
		Type:             "task_decompose",
		Version:          "v2",
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
