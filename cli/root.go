package cli

import (
	"fmt"
	"os"

	"NucleusMem/cli/cmd"
	"NucleusMem/pkg/client"

	"github.com/spf13/cobra"
)

var (
	serverAddr  string // 用于 Agent / MemSpace 直接操作
	managerAddr string // 用于 Manager 管理操作

	agentClient           *client.AgentClient
	memspaceClient        *client.MemSpaceClient
	agentManagerClient    *client.AgentManagerClient
	memspaceManagerClient *client.MemSpaceManagerClient
)

var rootCmd = &cobra.Command{
	Use:   "nucleuscli",
	Short: "CLI for NucleusMem operations",
	Long: `nucleuscli provides a command-line interface for managing and interacting with
Agents, MemSpaces, and the Manager.
	
Basic flow:
  - Use 'manager' commands to launch/destroy agents and memspaces.
  - Use 'agent' / 'memspace' commands to interact with running instances.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// 默认地址
		if serverAddr == "" {
			serverAddr = "localhost:8080"
		}
		if managerAddr == "" {
			managerAddr = "localhost:8080" // 可设为同一地址，但通常 Manager 端口不同
		}

		// 面向具体实例的客户端
		agentClient = client.NewAgentClient(serverAddr)
		memspaceClient = client.NewMemSpaceClient(serverAddr)

		// 面向 Manager 的客户端
		agentManagerClient = client.NewAgentManagerClient(managerAddr)
		memspaceManagerClient = client.NewMemSpaceManagerClient(managerAddr)
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&serverAddr, "server", "localhost:8080",
		"Address of the Agent/MemSpace instance (host:port)")
	rootCmd.PersistentFlags().StringVar(&managerAddr, "manager", "localhost:8080",
		"Address of the Manager service (host:port)")

	// 注册命令组
	rootCmd.AddCommand(cmd.NewAgentCmd(&agentClient))
	rootCmd.AddCommand(cmd.NewMemSpaceCmd(&memspaceClient))
	rootCmd.AddCommand(cmd.NewManagerCmd(&agentManagerClient, &memspaceManagerClient))
}
