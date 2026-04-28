package cmd

import (
	_ "encoding/json"
	"fmt"
	_ "strconv"

	"NucleusMem/pkg/api"
	"NucleusMem/pkg/client"

	"github.com/spf13/cobra"
)

// NewManagerCmd creates the manager command group
func NewManagerCmd(
	agentMgrClient **client.AgentManagerClient,
	memMgrClient **client.MemSpaceManagerClient,
) *cobra.Command {
	managerCmd := &cobra.Command{
		Use:   "manager",
		Short: "Manage agents and memspaces lifecycle via Manager",
		Long: `Provide lifecycle management of agents and memspaces through the central Manager service.
		
Requires the Manager service address (--manager flag).`,
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Help()
		},
	}
	// agent 子命令组
	agentSubCmd := &cobra.Command{
		Use:   "agent",
		Short: "Manage agents lifecycle",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Help()
		},
	}

	// nucleuscli manager agent list
	agentSubCmd.AddCommand(newManagerAgentListCmd(agentMgrClient))

	// nucleuscli manager agent launch --name myagent --role worker --bin /path/to/agent --image ubuntu:latest --mount-memspace ms1,ms2 --env KEY1=VAL1,KEY2=VAL2 --http-addr :9000 --config /path/to/config --is-job
	agentSubCmd.AddCommand(newManagerAgentLaunchCmd(agentMgrClient))

	// nucleuscli manager agent destroy --agent-id 3
	agentSubCmd.AddCommand(newManagerAgentDestroyCmd(agentMgrClient))
	managerCmd.AddCommand(agentSubCmd)

	// memspace 子命令组
	memspaceSubCmd := &cobra.Command{
		Use:   "memspace",
		Short: "Manage memspaces lifecycle",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Help()
		},
	}

	// nucleuscli manager memspace list
	memspaceSubCmd.AddCommand(newManagerMemSpaceListCmd(memMgrClient))

	// nucleuscli manager memspace launch --name myms --type volatile --owner-id 1 --description "test" --http-addr :8080 --pd-addr :2379 --embedding-addr :5000 --light-model-addr :6000 --summary-cnt 10 --summary-threshold 100 --bin /path/to/memspace --config /path/to/config
	memspaceSubCmd.AddCommand(newManagerMemSpaceLaunchCmd(memMgrClient))

	// nucleuscli manager memspace shutdown --memspace-id 5
	memspaceSubCmd.AddCommand(newManagerMemSpaceShutdownCmd(memMgrClient))
	managerCmd.AddCommand(memspaceSubCmd)
	return managerCmd
}

// =============================================================================
// Agent subcommands
// =============================================================================

func newManagerAgentListCmd(agentMgrClient **client.AgentManagerClient) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all agents registered with the Manager",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := *agentMgrClient
			agents, err := client.ListAgents()
			if err != nil {
				return fmt.Errorf("list agents failed: %w", err)
			}
			if len(agents) == 0 {
				fmt.Println("No agents found.")
				return nil
			}
			for _, a := range agents {
				printJSON(cmd, a)
			}
			return nil
		},
	}
}
func newManagerAgentLaunchCmd(agentMgrClient **client.AgentManagerClient) *cobra.Command {
	var (
		binPath    string
		configFile string
	)

	cmd := &cobra.Command{
		Use:   "launch",
		Short: "Launch a new agent process via Manager using binary and config file",
		Long: `Start an agent by providing the binary path and configuration file.
All agent settings (role, ports, memspace, etc.) are read from the config file.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			client := *agentMgrClient
			if binPath == "" {
				return fmt.Errorf("--bin is required")
			}
			if configFile == "" {
				return fmt.Errorf("--config is required")
			}
			req := &api.LaunchAgentRequestHTTP{
				BinPath:        binPath,
				ConfigFilePath: configFile,
				// 其他字段留空，服务端会从配置文件加载
			}
			resp, err := client.LaunchAgent(req)
			if err != nil {
				return fmt.Errorf("launch agent failed: %w", err)
			}
			fmt.Printf("Agent launched successfully:\n")
			printJSON(cmd, resp)
			return nil
		},
	}

	cmd.Flags().StringVar(&binPath, "bin", "bin/agent", "Path to agent binary (required)")
	cmd.Flags().StringVar(&configFile, "config", "", "Path to agent configuration file (required)")
	//cmd.MarkFlagRequired("bin")
	cmd.MarkFlagRequired("config")
	return cmd
}
func newManagerAgentDestroyCmd(agentMgrClient **client.AgentManagerClient) *cobra.Command {
	var agentID uint64
	cmd := &cobra.Command{
		Use:   "destroy",
		Short: "Stop and destroy an agent",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := *agentMgrClient
			if agentID == 0 {
				return fmt.Errorf("agent-id is required")
			}
			if err := client.StopAgent(agentID); err != nil {
				return fmt.Errorf("destroy agent failed: %w", err)
			}
			fmt.Printf("Agent %d destroyed successfully.\n", agentID)
			return nil
		},
	}
	cmd.Flags().Uint64Var(&agentID, "agent-id", 0, "ID of the agent to destroy (required)")
	cmd.MarkFlagRequired("agent-id")
	return cmd
}

// =============================================================================
// MemSpace subcommands
// =============================================================================

func newManagerMemSpaceListCmd(memMgrClient **client.MemSpaceManagerClient) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all memspaces managed by the Manager",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := *memMgrClient
			memspaces, err := client.ListMemSpaces()
			if err != nil {
				return fmt.Errorf("list memspaces failed: %w", err)
			}
			if len(memspaces) == 0 {
				fmt.Println("No memspaces found.")
				return nil
			}
			for _, m := range memspaces {
				printJSON(cmd, m)
			}
			return nil
		},
	}
}
func newManagerMemSpaceLaunchCmd(memMgrClient **client.MemSpaceManagerClient) *cobra.Command {
	var configFile string
	var bin string

	cmd := &cobra.Command{
		Use:   "launch",
		Short: "Launch a new MemSpace process via Manager using a config file",
		Long:  `All MemSpace parameters are defined in the configuration file. Only the config file path is needed.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			client := *memMgrClient
			if configFile == "" {
				return fmt.Errorf("--config is required")
			}
			req := &api.LaunchMemSpaceRequestManager{
				ConfigFilePath: configFile,
				BinPath:        bin,
				// 其他字段全部留空，服务端会从配置文件读取
			}
			if err := client.LaunchMemSpace(req); err != nil {
				return fmt.Errorf("launch memspace failed: %w", err)
			}
			fmt.Printf("MemSpace launched successfully.\n")
			return nil
		},
	}

	cmd.Flags().StringVar(&configFile, "config", "", "Path to MemSpace configuration file (required)")
	cmd.Flags().StringVar(&bin, "bin", "bin/memspace", "Path to agent binary (required)")
	cmd.MarkFlagRequired("config")
	return cmd
}

func newManagerMemSpaceShutdownCmd(memMgrClient **client.MemSpaceManagerClient) *cobra.Command {
	var memspaceID uint64
	cmd := &cobra.Command{
		Use:   "shutdown",
		Short: "Shutdown a MemSpace gracefully",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := *memMgrClient
			if memspaceID == 0 {
				return fmt.Errorf("memspace-id is required")
			}
			if err := client.ShutdownMemSpace(memspaceID); err != nil {
				return fmt.Errorf("shutdown memspace failed: %w", err)
			}
			fmt.Printf("MemSpace %d shutdown initiated.\n", memspaceID)
			return nil
		},
	}
	cmd.Flags().Uint64Var(&memspaceID, "memspace-id", 0, "ID of the memspace to shutdown (required)")
	cmd.MarkFlagRequired("memspace-id")
	return cmd
}

// printJSON is a helper (defined elsewhere, e.g., in agent.go)
// Make sure it's accessible from this file or define again.
