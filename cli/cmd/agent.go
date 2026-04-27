package cmd

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"NucleusMem/pkg/api"
	"NucleusMem/pkg/client"

	"github.com/spf13/cobra"
)

// NewAgentCmd creates the agent command group
func NewAgentCmd(agentClientPtr **client.AgentClient) *cobra.Command {
	agentCmd := &cobra.Command{
		Use:   "agent",
		Short: "Interact with a running agent",
		Long:  `Commands to chat, bind/unbind memspaces, submit tasks, communicate, and more.`,
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Help()
		},
	}

	// 添加所有子命令
	agentCmd.AddCommand(newAgentChatCmd(agentClientPtr))
	agentCmd.AddCommand(newAgentTempChatCmd(agentClientPtr))
	agentCmd.AddCommand(newAgentBindCmd(agentClientPtr))
	agentCmd.AddCommand(newAgentUnbindCmd(agentClientPtr))
	agentCmd.AddCommand(newAgentNotifyCmd(agentClientPtr))
	agentCmd.AddCommand(newAgentHealthCmd(agentClientPtr))
	agentCmd.AddCommand(newAgentShutdownCmd(agentClientPtr))
	agentCmd.AddCommand(newAgentCommunicateCmd(agentClientPtr))
	agentCmd.AddCommand(newAgentTaskCmd(agentClientPtr))

	return agentCmd
}

// ---------- chat ----------
func newAgentChatCmd(agentClientPtr **client.AgentClient) *cobra.Command {
	var message string
	cmd := &cobra.Command{
		Use:   "chat",
		Short: "Send a persistent chat message",
		Long:  "Sends a message to the agent that is stored and can access memspace context (future).",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := *agentClientPtr
			resp, err := client.Chat(message)
			if err != nil {
				return fmt.Errorf("chat failed: %w", err)
			}
			printJSON(resp)
			return nil
		},
	}
	cmd.Flags().StringVarP(&message, "message", "m", "", "Chat message (required)")
	cmd.MarkFlagRequired("message")
	return cmd
}

// ---------- tempchat ----------
func newAgentTempChatCmd(agentClientPtr **client.AgentClient) *cobra.Command {
	var message string
	cmd := &cobra.Command{
		Use:   "tempchat",
		Short: "Send a temporary chat message (in-memory only)",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := *agentClientPtr
			resp, err := client.TempChat(message)
			if err != nil {
				return fmt.Errorf("temp chat failed: %w", err)
			}
			printJSON(resp)
			return nil
		},
	}
	cmd.Flags().StringVarP(&message, "message", "m", "", "Chat message (required)")
	cmd.MarkFlagRequired("message")
	return cmd
}

// ---------- bind ----------
func newAgentBindCmd(agentClientPtr **client.AgentClient) *cobra.Command {
	var memspaceID uint64
	cmd := &cobra.Command{
		Use:   "bind",
		Short: "Bind the agent to a MemSpace",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := *agentClientPtr
			req := &api.BindMemSpaceRequest{
				MemSpaceID: strconv.FormatUint(memspaceID, 10),
			}
			if err := client.BindMemSpace(req); err != nil {
				return fmt.Errorf("bind failed: %w", err)
			}
			fmt.Printf("Successfully bound to memspace %d\n", memspaceID)
			return nil
		},
	}
	cmd.Flags().Uint64Var(&memspaceID, "memspace-id", 0, "MemSpace ID to bind to (required)")
	cmd.MarkFlagRequired("memspace-id")
	return cmd
}

// ---------- unbind ----------
func newAgentUnbindCmd(agentClientPtr **client.AgentClient) *cobra.Command {
	var memspaceID uint64
	cmd := &cobra.Command{
		Use:   "unbind",
		Short: "Unbind the agent from a MemSpace",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := *agentClientPtr
			if err := client.UnbindMemSpace(memspaceID); err != nil {
				return fmt.Errorf("unbind failed: %w", err)
			}
			fmt.Printf("Successfully unbound from memspace %d\n", memspaceID)
			return nil
		},
	}
	cmd.Flags().Uint64Var(&memspaceID, "memspace-id", 0, "MemSpace ID to unbind from (required)")
	cmd.MarkFlagRequired("memspace-id")
	return cmd
}

// ---------- notify ----------
func newAgentNotifyCmd(agentClientPtr **client.AgentClient) *cobra.Command {
	var key, content string
	cmd := &cobra.Command{
		Use:   "notify",
		Short: "Send a key-content notification to the agent",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := *agentClientPtr
			result, err := client.Notify(key, content)
			if err != nil {
				return fmt.Errorf("notify failed: %w", err)
			}
			fmt.Printf("Notify acknowledged: %s\n", result)
			return nil
		},
	}
	cmd.Flags().StringVarP(&key, "key", "k", "", "Notification key (required)")
	cmd.Flags().StringVarP(&content, "content", "c", "", "Notification content (required)")
	cmd.MarkFlagRequired("key")
	cmd.MarkFlagRequired("content")
	return cmd
}

// ---------- health ----------
func newAgentHealthCmd(agentClientPtr **client.AgentClient) *cobra.Command {
	var monitorID uint64
	cmd := &cobra.Command{
		Use:   "health",
		Short: "Check agent health (optionally bind to a monitor)",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := *agentClientPtr
			resp, err := client.HealthCheckWithMonitor(monitorID)
			if err != nil {
				return fmt.Errorf("health check failed: %w", err)
			}
			printJSON(resp)
			return nil
		},
	}
	cmd.Flags().Uint64Var(&monitorID, "monitor-id", 0, "Monitor ID to bind to (0 means no monitor)")
	return cmd
}

// ---------- shutdown ----------
func newAgentShutdownCmd(agentClientPtr **client.AgentClient) *cobra.Command {
	return &cobra.Command{
		Use:   "shutdown",
		Short: "Gracefully shut down the agent",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := *agentClientPtr
			if err := client.Shutdown(); err != nil {
				return fmt.Errorf("shutdown failed: %w", err)
			}
			fmt.Println("Agent shutdown initiated")
			return nil
		},
	}
}

// ---------- communicate ----------
func newAgentCommunicateCmd(agentClientPtr **client.AgentClient) *cobra.Command {
	var targetID uint64
	var key, content string
	cmd := &cobra.Command{
		Use:   "communicate",
		Short: "Send a message to another agent via the current agent",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := *agentClientPtr
			result, err := client.Communicate(targetID, key, content)
			if err != nil {
				return fmt.Errorf("communicate failed: %w", err)
			}
			fmt.Printf("Agent %d responded: %s\n", targetID, result)
			return nil
		},
	}
	cmd.Flags().Uint64Var(&targetID, "target", 0, "Target agent ID (required)")
	cmd.Flags().StringVarP(&key, "key", "k", "", "Message key (required)")
	cmd.Flags().StringVarP(&content, "content", "c", "", "Message content (required)")
	cmd.MarkFlagRequired("target")
	cmd.MarkFlagRequired("key")
	cmd.MarkFlagRequired("content")
	return cmd
}

// ---------- task ----------
func newAgentTaskCmd(agentClientPtr **client.AgentClient) *cobra.Command {
	taskCmd := &cobra.Command{
		Use:   "task",
		Short: "Manage agent tasks",
		Long:  "Subcommands to submit tasks and retrieve results.",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Help()
		},
	}
	taskCmd.AddCommand(newAgentTaskSubmitCmd(agentClientPtr))
	taskCmd.AddCommand(newAgentTaskResultCmd(agentClientPtr))
	return taskCmd
}

func newAgentTaskSubmitCmd(agentClientPtr **client.AgentClient) *cobra.Command {
	var (
		taskType  string
		content   string
		tools     []string
		memTags   []string
		maxRetry  int
		toolName  string
		paramsRaw string
	)

	cmd := &cobra.Command{
		Use:   "submit",
		Short: "Submit a task to the agent queue",
		Long: `Submits a task with type, content, available tools, memory tags,
retry settings, tool name, and optional parameters.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			client := *agentClientPtr

			// 解析 params JSON 字符串 -> map[string]interface{}
			var paramsMap map[string]interface{}
			if paramsRaw != "" {
				if err := json.Unmarshal([]byte(paramsRaw), &paramsMap); err != nil {
					return fmt.Errorf("invalid --params JSON: %w", err)
				}
			}

			req := &api.SubmitTaskRequest{
				Type:             taskType,
				Content:          content,
				AvailableTools:   tools,
				AvailableMemTags: memTags,
				MaxRetry:         maxRetry,
				ToolName:         toolName,
				Params:           paramsMap,
			}

			resp, err := client.SubmitTask(req)
			if err != nil {
				return fmt.Errorf("submit task failed: %w", err)
			}

			fmt.Printf("Task submitted successfully, task ID: %s\n", resp.TaskID)
			return nil
		},
	}

	cmd.Flags().StringVarP(&taskType, "type", "t", "", "Task type (required)")
	cmd.Flags().StringVarP(&content, "content", "c", "", "Task content description")
	cmd.Flags().StringSliceVarP(&tools, "tools", "T", nil, "Available tools (comma-separated)")
	cmd.Flags().StringSliceVarP(&memTags, "mem-tags", "M", nil, "Memory tags (comma-separated)")
	cmd.Flags().IntVarP(&maxRetry, "max-retry", "r", 0, "Max retry count")
	cmd.Flags().StringVarP(&toolName, "tool-name", "n", "", "Designated tool name")
	cmd.Flags().StringVarP(&paramsRaw, "params", "p", "", "Task parameters as JSON object")

	cmd.MarkFlagRequired("type")
	cmd.MarkFlagRequired("content") // 根据业务，content 很可能也是必填的

	return cmd
}

func newAgentTaskResultCmd(agentClientPtr **client.AgentClient) *cobra.Command {
	var taskID string
	var maxWait time.Duration
	cmd := &cobra.Command{
		Use:   "result",
		Short: "Get the result of a previously submitted task",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := *agentClientPtr
			resp, err := client.GetTaskResult(taskID, maxWait)
			if err != nil {
				return fmt.Errorf("get task result failed: %w", err)
			}
			printJSON(resp)
			return nil
		},
	}
	cmd.Flags().StringVar(&taskID, "task-id", "", "Task ID (required)")
	cmd.Flags().DurationVar(&maxWait, "timeout", 60*time.Second, "Maximum wait time")
	cmd.MarkFlagRequired("task-id")
	return cmd
}

// ---------- helper ----------
func printJSON(v interface{}) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		fmt.Printf("Unable to format output: %v\n", err)
		return
	}
	fmt.Println(string(b))
}
