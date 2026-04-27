package cmd

import (
	"NucleusMem/pkg/configs"
	"encoding/json"
	"fmt"
	//"strconv"
	//"strings"
	//
	//"NucleusMem/pkg/api"
	"NucleusMem/pkg/client"

	"github.com/spf13/cobra"
)

func NewMemSpaceCmd(memspaceClientPtr **client.MemSpaceClient) *cobra.Command {
	memspaceCmd := &cobra.Command{
		Use:   "memspace",
		Short: "Interact with a running MemSpace",
		Long:  `Commands to manage tools, DAGs, memory, agents, and messages inside a MemSpace.`,
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Help()
		},
	}

	// 工具管理
	toolCmd := &cobra.Command{
		Use:   "tool",
		Short: "Manage standard tools (non-standard-tool)",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Help()
		},
	}
	toolCmd.AddCommand(newMemSpaceToolListCmd(memspaceClientPtr))
	toolCmd.AddCommand(newMemSpaceToolGetCmd(memspaceClientPtr))
	toolCmd.AddCommand(newMemSpaceToolRegisterCmd(memspaceClientPtr))
	toolCmd.AddCommand(newMemSpaceToolDeleteCmd(memspaceClientPtr))
	toolCmd.AddCommand(newMemSpaceToolFindByTagsCmd(memspaceClientPtr))
	memspaceCmd.AddCommand(toolCmd)

	// 标准工具管理
	stdToolCmd := &cobra.Command{
		Use:   "standard-tool",
		Short: "Manage standard tools with extended definitions",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Help()
		},
	}
	stdToolCmd.AddCommand(newMemSpaceStandardToolListCmd(memspaceClientPtr))
	stdToolCmd.AddCommand(newMemSpaceStandardToolGetCmd(memspaceClientPtr))
	stdToolCmd.AddCommand(newMemSpaceStandardToolRegisterCmd(memspaceClientPtr))
	stdToolCmd.AddCommand(newMemSpaceStandardToolDeleteCmd(memspaceClientPtr))
	memspaceCmd.AddCommand(stdToolCmd)

	// DAG
	dagCmd := &cobra.Command{
		Use:   "dag",
		Short: "Manage tool dependency graphs (DAGs)",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Help()
		},
	}
	dagCmd.AddCommand(newMemSpaceDAGSaveCmd(memspaceClientPtr))
	dagCmd.AddCommand(newMemSpaceDAGLoadCmd(memspaceClientPtr))
	memspaceCmd.AddCommand(dagCmd)

	// 执行记录
	execCmd := &cobra.Command{
		Use:   "exec",
		Short: "Record and query tool execution results",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Help()
		},
	}
	execCmd.AddCommand(newMemSpaceExecRecordCmd(memspaceClientPtr))
	execCmd.AddCommand(newMemSpaceExecBatchRecordCmd(memspaceClientPtr))
	execCmd.AddCommand(newMemSpaceExecHistoryCmd(memspaceClientPtr))
	memspaceCmd.AddCommand(execCmd)

	// 内存操作
	memoryCmd := &cobra.Command{
		Use:   "memory",
		Short: "Write and query memory entries",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Help()
		},
	}
	memoryCmd.AddCommand(newMemSpaceMemoryWriteCmd(memspaceClientPtr))
	memoryCmd.AddCommand(newMemSpaceMemoryContextCmd(memspaceClientPtr))
	memoryCmd.AddCommand(newMemSpaceMemoryGetByKeyCmd(memspaceClientPtr))
	memspaceCmd.AddCommand(memoryCmd)

	// Agent 管理（在 MemSpace 内的注册/绑定）
	agentCmd := &cobra.Command{
		Use:   "agent",
		Short: "Manage agents registered in this MemSpace",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Help()
		},
	}
	agentCmd.AddCommand(newMemSpaceAgentListCmd(memspaceClientPtr))
	agentCmd.AddCommand(newMemSpaceAgentRegisterCmd(memspaceClientPtr))
	agentCmd.AddCommand(newMemSpaceAgentUnregisterCmd(memspaceClientPtr))
	agentCmd.AddCommand(newMemSpaceAgentBindCmd(memspaceClientPtr))
	agentCmd.AddCommand(newMemSpaceAgentUnbindCmd(memspaceClientPtr))
	memspaceCmd.AddCommand(agentCmd)

	// 消息
	msgCmd := &cobra.Command{
		Use:   "message",
		Short: "Send messages between agents",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Help()
		},
	}
	msgCmd.AddCommand(newMemSpaceMessageSendCmd(memspaceClientPtr))
	memspaceCmd.AddCommand(msgCmd)

	// 健康检查 & 关闭
	memspaceCmd.AddCommand(newMemSpaceHealthCmd(memspaceClientPtr))
	memspaceCmd.AddCommand(newMemSpaceShutdownCmd(memspaceClientPtr))

	return memspaceCmd
}

// ============================================================
// 工具命令
// ============================================================
func newMemSpaceToolListCmd(c **client.MemSpaceClient) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all tools in this MemSpace",
		RunE: func(cmd *cobra.Command, args []string) error {
			tools, err := (*c).ListTools()
			if err != nil {
				return err
			}
			printJSON(tools)
			return nil
		},
	}
}

func newMemSpaceToolGetCmd(c **client.MemSpaceClient) *cobra.Command {
	var name string
	cmd := &cobra.Command{
		Use:   "get",
		Short: "Get a tool by name",
		RunE: func(cmd *cobra.Command, args []string) error {
			tool, err := (*c).GetTool(name)
			if err != nil {
				return err
			}
			printJSON(tool)
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Tool name (required)")
	cmd.MarkFlagRequired("name")
	return cmd
}
func newMemSpaceToolRegisterCmd(c **client.MemSpaceClient) *cobra.Command {
	var (
		name         string
		description  string
		tags         []string
		execType     string
		paramsJSON   string
		returnType   string
		endpoint     string
		metadataJSON string
	)
	cmd := &cobra.Command{
		Use:   "register",
		Short: "Register a new tool",
		RunE: func(cmd *cobra.Command, args []string) error {
			// 解析 Parameters JSON 数组
			var params []configs.ToolParam
			if paramsJSON != "" {
				if err := json.Unmarshal([]byte(paramsJSON), &params); err != nil {
					return fmt.Errorf("invalid --parameters JSON: %w", err)
				}
			}
			// 解析 Metadata JSON map
			var metadata map[string]string
			if metadataJSON != "" {
				if err := json.Unmarshal([]byte(metadataJSON), &metadata); err != nil {
					return fmt.Errorf("invalid --metadata JSON: %w", err)
				}
			}

			tool := &configs.ToolDefinition{
				Name:        name,
				Description: description,
				Tags:        tags,
				Parameters:  params,
				ReturnType:  returnType,
				Endpoint:    endpoint,
				Metadata:    metadata,
				// CreatedAt 通常由服务端生成
				ExecType: execType,
			}
			if err := (*c).RegisterTool(tool); err != nil {
				return err
			}
			fmt.Println("Tool registered successfully.")
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Tool name (required)")
	cmd.Flags().StringVar(&description, "description", "", "Tool description")
	cmd.Flags().StringSliceVar(&tags, "tags", nil, "Comma-separated tags")
	cmd.Flags().StringVar(&execType, "exec-type", "", "Execution type: http, shell, mcp, grpc (required)")
	cmd.Flags().StringVar(&paramsJSON, "parameters", "", "JSON array of tool parameters, e.g. '[{\"name\":\"input\",\"type\":\"string\"}]'")
	cmd.Flags().StringVar(&returnType, "return-type", "", "Return type: string, object, list")
	cmd.Flags().StringVar(&endpoint, "endpoint", "", "HTTP endpoint or function name")
	cmd.Flags().StringVar(&metadataJSON, "metadata", "", "JSON object for metadata, e.g. '{\"key\":\"value\"}'")
	cmd.MarkFlagRequired("name")
	cmd.MarkFlagRequired("exec-type")
	return cmd
}

func newMemSpaceToolDeleteCmd(c **client.MemSpaceClient) *cobra.Command {
	var name string
	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a tool by name",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := (*c).DeleteTool(name); err != nil {
				return err
			}
			fmt.Println("Tool deleted.")
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Tool name (required)")
	cmd.MarkFlagRequired("name")
	return cmd
}

func newMemSpaceToolFindByTagsCmd(c **client.MemSpaceClient) *cobra.Command {
	var tags []string
	cmd := &cobra.Command{
		Use:   "find-by-tags",
		Short: "Find tools by tags",
		RunE: func(cmd *cobra.Command, args []string) error {
			tools, err := (*c).FindToolsByTags(tags)
			if err != nil {
				return err
			}
			printJSON(tools)
			return nil
		},
	}
	cmd.Flags().StringSliceVar(&tags, "tags", nil, "Tags to search (comma-separated, required)")
	cmd.MarkFlagRequired("tags")
	return cmd
}

// ============================================================
// 标准工具命令
// ============================================================
func newMemSpaceStandardToolListCmd(c **client.MemSpaceClient) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all standard tools",
		RunE: func(cmd *cobra.Command, args []string) error {
			tools, err := (*c).ListStandardTools()
			if err != nil {
				return err
			}
			printJSON(tools)
			return nil
		},
	}
}

func newMemSpaceStandardToolGetCmd(c **client.MemSpaceClient) *cobra.Command {
	var name string
	cmd := &cobra.Command{
		Use:   "get",
		Short: "Get a standard tool by name",
		RunE: func(cmd *cobra.Command, args []string) error {
			tool, err := (*c).GetStandardTool(name)
			if err != nil {
				return err
			}
			printJSON(tool)
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Tool name (required)")
	cmd.MarkFlagRequired("name")
	return cmd
}
func newMemSpaceStandardToolRegisterCmd(c **client.MemSpaceClient) *cobra.Command {
	var (
		name        string
		description string
		tags        []string
		toolType    string // 对应 ToolType，如 "http","shell","mcp","delegate"
		paramsJSON  string
		timeout     int
		retryCount  int
		configJSON  string // 整个执行器配置的 JSON，根据 toolType 解析
	)
	cmd := &cobra.Command{
		Use:   "register",
		Short: "Register a new standard tool with extended executor configs",
		Long: `Register a standard tool. The --executor-config must be a valid JSON matching the --type:
- http: {"url":"...", "method":"GET", "headers":{}, "param_location":"body"}
- shell: {"command_template":"...", "work_dir":"/tmp", "interactive":false}
- mcp: {"server_name":"...", "tool_name":"...", "metadata":{}}
- delegate: {"agent_type":"...", "instruction_prefix":"...", "task_input_field":"...", "sandbox_mode":"none"}`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// 解析 Parameters JSON 数组
			var params []configs.StandardToolParam
			if paramsJSON != "" {
				if err := json.Unmarshal([]byte(paramsJSON), &params); err != nil {
					return fmt.Errorf("invalid --parameters JSON: %w", err)
				}
			}

			tool := &configs.StandardToolDefinition{
				Name:           name,
				Description:    description,
				Tags:           tags,
				Type:           configs.ToolType(toolType),
				Parameters:     params,
				TimeoutSeconds: timeout,
				RetryCount:     retryCount,
			}

			// 根据类型解析 executor-config 到对应的指针字段
			if configJSON != "" {
				switch configs.ToolType(toolType) {
				case "http":
					var cfg configs.HTTPExecutorConfig
					if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
						return fmt.Errorf("invalid HTTP config JSON: %w", err)
					}
					tool.HTTPConfig = &cfg
				case "shell":
					var cfg configs.ShellExecutorConfig
					if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
						return fmt.Errorf("invalid Shell config JSON: %w", err)
					}
					tool.ShellConfig = &cfg
				case "mcp":
					var cfg configs.MCPExecutorConfig
					if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
						return fmt.Errorf("invalid MCP config JSON: %w", err)
					}
					tool.MCPConfig = &cfg
				case "delegate":
					var cfg configs.DelegateExecutorConfig
					if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
						return fmt.Errorf("invalid Delegate config JSON: %w", err)
					}
					tool.DelegateConfig = &cfg
				default:
					return fmt.Errorf("unsupported type %q, must be http, shell, mcp, or delegate", toolType)
				}
			}

			if err := (*c).RegisterStandardTool(tool); err != nil {
				return err
			}
			fmt.Println("Standard tool registered.")
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Tool name (required)")
	cmd.Flags().StringVar(&description, "description", "", "Tool description")
	cmd.Flags().StringSliceVar(&tags, "tags", nil, "Comma-separated tags")
	cmd.Flags().StringVar(&toolType, "type", "", "Tool type: http, shell, mcp, delegate (required)")
	cmd.Flags().StringVar(&paramsJSON, "parameters", "", "JSON array of StandardToolParam, e.g. '[{\"name\":\"input\",\"type\":\"string\",\"required\":true}]'")
	cmd.Flags().IntVar(&timeout, "timeout", 30, "Timeout in seconds")
	cmd.Flags().IntVar(&retryCount, "retry-count", 0, "Retry count")
	cmd.Flags().StringVar(&configJSON, "executor-config", "", "Executor configuration JSON, must match --type")

	cmd.MarkFlagRequired("name")
	cmd.MarkFlagRequired("type")
	cmd.MarkFlagRequired("executor-config")
	return cmd
}

func newMemSpaceStandardToolDeleteCmd(c **client.MemSpaceClient) *cobra.Command {
	var name string
	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a standard tool",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := (*c).DeleteStandardTool(name); err != nil {
				return err
			}
			fmt.Println("Standard tool deleted.")
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Tool name (required)")
	cmd.MarkFlagRequired("name")
	return cmd
}

// ============================================================
// DAG 命令
// ============================================================
func newMemSpaceDAGSaveCmd(c **client.MemSpaceClient) *cobra.Command {
	var dagJSON string
	cmd := &cobra.Command{
		Use:   "save",
		Short: "Save a tool DAG to MemSpace",
		RunE: func(cmd *cobra.Command, args []string) error {
			var dag configs.ToolDAG
			if err := json.Unmarshal([]byte(dagJSON), &dag); err != nil {
				return fmt.Errorf("invalid DAG JSON: %w", err)
			}
			if err := (*c).SaveToolDAG(&dag); err != nil {
				return err
			}
			fmt.Println("DAG saved.")
			return nil
		},
	}
	cmd.Flags().StringVar(&dagJSON, "dag", "", "DAG definition as JSON string (required)")
	cmd.MarkFlagRequired("dag")
	return cmd
}

func newMemSpaceDAGLoadCmd(c **client.MemSpaceClient) *cobra.Command {
	return &cobra.Command{
		Use:   "load",
		Short: "Load the current tool DAG",
		RunE: func(cmd *cobra.Command, args []string) error {
			dag, err := (*c).LoadToolDAG()
			if err != nil {
				return err
			}
			printJSON(dag)
			return nil
		},
	}
}

// ============================================================
// 执行记录命令
// ============================================================
func newMemSpaceExecRecordCmd(c **client.MemSpaceClient) *cobra.Command {
	var (
		toolName string
		output   string
		errMsg   string
	)
	cmd := &cobra.Command{
		Use:   "record",
		Short: "Record a single tool execution",
		RunE: func(cmd *cobra.Command, args []string) error {
			var outMap map[string]interface{}
			if output != "" {
				if err := json.Unmarshal([]byte(output), &outMap); err != nil {
					return fmt.Errorf("invalid output JSON: %w", err)
				}
			}
			if err := (*c).RecordToolExec(toolName, outMap, errMsg); err != nil {
				return err
			}
			fmt.Println("Exec record saved.")
			return nil
		},
	}
	cmd.Flags().StringVar(&toolName, "tool-name", "", "Tool name (required)")
	cmd.Flags().StringVar(&output, "output", "{}", "Tool output as JSON")
	cmd.Flags().StringVar(&errMsg, "error", "", "Error message if any")
	cmd.MarkFlagRequired("tool-name")
	return cmd
}

func newMemSpaceExecBatchRecordCmd(c **client.MemSpaceClient) *cobra.Command {
	var resultsJSON string
	cmd := &cobra.Command{
		Use:   "batch-record",
		Short: "Record multiple tool execution results at once",
		RunE: func(cmd *cobra.Command, args []string) error {
			var results map[string]*configs.ToolExecResult
			if err := json.Unmarshal([]byte(resultsJSON), &results); err != nil {
				return fmt.Errorf("invalid results JSON: %w", err)
			}
			if err := (*c).RecordToolExecBatch(results); err != nil {
				return err
			}
			fmt.Println("Batch exec records saved.")
			return nil
		},
	}
	cmd.Flags().StringVar(&resultsJSON, "results", "", "JSON mapping tool_name -> {output, error} (required)")
	cmd.MarkFlagRequired("results")
	return cmd
}

func newMemSpaceExecHistoryCmd(c **client.MemSpaceClient) *cobra.Command {
	var toolName string
	cmd := &cobra.Command{
		Use:   "history",
		Short: "Get execution history for a tool",
		RunE: func(cmd *cobra.Command, args []string) error {
			records, err := (*c).GetToolExecHistory(toolName)
			if err != nil {
				return err
			}
			printJSON(records)
			return nil
		},
	}
	cmd.Flags().StringVar(&toolName, "tool-name", "", "Tool name (required)")
	cmd.MarkFlagRequired("tool-name")
	return cmd
}

// ============================================================
// 内存命令
// ============================================================
func newMemSpaceMemoryWriteCmd(c **client.MemSpaceClient) *cobra.Command {
	var (
		content string
		agentID uint64
	)
	cmd := &cobra.Command{
		Use:   "write",
		Short: "Write a memory entry",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := (*c).WriteMemory(content, agentID); err != nil {
				return err
			}
			fmt.Println("Memory written.")
			return nil
		},
	}
	cmd.Flags().StringVar(&content, "content", "", "Memory content (required)")
	cmd.Flags().Uint64Var(&agentID, "agent-id", 0, "Associate this memory with an agent (optional)")
	cmd.MarkFlagRequired("content")
	return cmd
}

func newMemSpaceMemoryContextCmd(c **client.MemSpaceClient) *cobra.Command {
	var (
		summaryBefore int64
		query         string
		n             int
	)
	cmd := &cobra.Command{
		Use:   "context",
		Short: "Retrieve memory context",
		RunE: func(cmd *cobra.Command, args []string) error {
			summary, memories, err := (*c).GetMemoryContext(summaryBefore, query, n)
			if err != nil {
				return err
			}
			// 格式化输出
			result := map[string]interface{}{
				"summary":  summary,
				"memories": memories,
			}
			printJSON(result)
			return nil
		},
	}
	cmd.Flags().Int64Var(&summaryBefore, "summary-before", 0, "Timestamp before which summaries are considered")
	cmd.Flags().StringVar(&query, "query", "", "Search query (required)")
	cmd.Flags().IntVar(&n, "n", 10, "Number of memory entries to return")
	cmd.MarkFlagRequired("query")
	return cmd
}

func newMemSpaceMemoryGetByKeyCmd(c **client.MemSpaceClient) *cobra.Command {
	var rawKey string
	cmd := &cobra.Command{
		Use:   "get-by-key",
		Short: "Get raw memory value by exact key",
		RunE: func(cmd *cobra.Command, args []string) error {
			val, err := (*c).GetMemoryByKey([]byte(rawKey))
			if err != nil {
				return err
			}
			fmt.Println(val)
			return nil
		},
	}
	cmd.Flags().StringVar(&rawKey, "key", "", "Raw key (string, required)")
	cmd.MarkFlagRequired("key")
	return cmd
}

// ============================================================
// Agent 在 MemSpace 内的注册与绑定
// ============================================================
func newMemSpaceAgentListCmd(c **client.MemSpaceClient) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List agents registered in this MemSpace",
		RunE: func(cmd *cobra.Command, args []string) error {
			agents, err := (*c).ListAgents()
			if err != nil {
				return err
			}
			printJSON(agents)
			return nil
		},
	}
}

func newMemSpaceAgentRegisterCmd(c **client.MemSpaceClient) *cobra.Command {
	var (
		agentID uint64
		addr    string
		role    string
	)
	cmd := &cobra.Command{
		Use:   "register",
		Short: "Register an agent in this MemSpace",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := (*c).RegisterAgent(agentID, addr, role); err != nil {
				return err
			}
			fmt.Println("Agent registered.")
			return nil
		},
	}
	cmd.Flags().Uint64Var(&agentID, "agent-id", 0, "Agent ID (required)")
	cmd.Flags().StringVar(&addr, "addr", "", "Agent network address (required)")
	cmd.Flags().StringVar(&role, "role", "", "Agent role (optional)")
	cmd.MarkFlagRequired("agent-id")
	cmd.MarkFlagRequired("addr")
	return cmd
}

func newMemSpaceAgentUnregisterCmd(c **client.MemSpaceClient) *cobra.Command {
	var agentID uint64
	cmd := &cobra.Command{
		Use:   "unregister",
		Short: "Unregister an agent from this MemSpace",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := (*c).UnregisterAgent(agentID); err != nil {
				return err
			}
			fmt.Println("Agent unregistered.")
			return nil
		},
	}
	cmd.Flags().Uint64Var(&agentID, "agent-id", 0, "Agent ID (required)")
	cmd.MarkFlagRequired("agent-id")
	return cmd
}

func newMemSpaceAgentBindCmd(c **client.MemSpaceClient) *cobra.Command {
	var (
		agentID uint64
		addr    string
		role    string
	)
	cmd := &cobra.Command{
		Use:   "bind",
		Short: "Bind an agent to this MemSpace",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := (*c).BindAgent(agentID, addr, role); err != nil {
				return err
			}
			fmt.Println("Agent bound to memspace.")
			return nil
		},
	}
	cmd.Flags().Uint64Var(&agentID, "agent-id", 0, "Agent ID (required)")
	cmd.Flags().StringVar(&addr, "addr", "", "Agent network address (required)")
	cmd.Flags().StringVar(&role, "role", "", "Agent role (optional)")
	cmd.MarkFlagRequired("agent-id")
	cmd.MarkFlagRequired("addr")
	return cmd
}

func newMemSpaceAgentUnbindCmd(c **client.MemSpaceClient) *cobra.Command {
	var agentID uint64
	cmd := &cobra.Command{
		Use:   "unbind",
		Short: "Unbind an agent from this MemSpace",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := (*c).UnbindAgent(agentID); err != nil {
				return err
			}
			fmt.Println("Agent unbound.")
			return nil
		},
	}
	cmd.Flags().Uint64Var(&agentID, "agent-id", 0, "Agent ID (required)")
	cmd.MarkFlagRequired("agent-id")
	return cmd
}

// ============================================================
// 消息发送
// ============================================================
func newMemSpaceMessageSendCmd(c **client.MemSpaceClient) *cobra.Command {
	var (
		fromAgent uint64
		toAgent   uint64
		key       string
		content   string
	)
	cmd := &cobra.Command{
		Use:   "send",
		Short: "Send a message from one agent to another via MemSpace",
		RunE: func(cmd *cobra.Command, args []string) error {
			response, err := (*c).SendMessage(fromAgent, toAgent, key, content)
			if err != nil {
				return err
			}
			fmt.Println(response)
			return nil
		},
	}
	cmd.Flags().Uint64Var(&fromAgent, "from", 0, "Sender agent ID (required)")
	cmd.Flags().Uint64Var(&toAgent, "to", 0, "Receiver agent ID (required)")
	cmd.Flags().StringVar(&key, "key", "", "Message key (required)")
	cmd.Flags().StringVar(&content, "content", "", "Message content (required)")
	cmd.MarkFlagRequired("from")
	cmd.MarkFlagRequired("to")
	cmd.MarkFlagRequired("key")
	cmd.MarkFlagRequired("content")
	return cmd
}

// ============================================================
// 健康检查与关闭
// ============================================================
func newMemSpaceHealthCmd(c **client.MemSpaceClient) *cobra.Command {
	return &cobra.Command{
		Use:   "health",
		Short: "Check MemSpace health",
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := (*c).HealthCheckWithInfo()
			if err != nil {
				return err
			}
			printJSON(resp)
			return nil
		},
	}
}

func newMemSpaceShutdownCmd(c **client.MemSpaceClient) *cobra.Command {
	return &cobra.Command{
		Use:   "shutdown",
		Short: "Shutdown the MemSpace",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := (*c).Shutdown(); err != nil {
				return err
			}
			fmt.Println("MemSpace shutdown initiated.")
			return nil
		},
	}
}
