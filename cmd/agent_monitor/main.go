package main

import (
	"NucleusMem/pkg/configs"
	_ "NucleusMem/pkg/configs"
	"NucleusMem/pkg/controller/monitor/agent_monitor"
	"context"
	"fmt"
	"github.com/pingcap-incubator/tinykv/log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

func main() {
	// 定义 3 个 monitor 节点（nodeID -> port）
	filePath := "./pkg/configs/file/agent_manager_config.yaml"
	//filePath := "/home/chengzipi/Public/project/NucleusMem/cmd/agent_monitor/agent_manager_config.yaml"
	agentManagerConfig, err := configs.LoadAgentManagerConfigFromYAML(filePath)
	if err != nil {
		panic(err)
	}
	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(context.Background())

	// 启动每个 monitor
	for nodeID, url := range agentManagerConfig.MonitorURLs {
		monitorConfig := &configs.MonitorConfig{
			MonitorUrl: url,
		}
		wg.Add(1)
		go func(id uint64, config *configs.MonitorConfig) {
			defer wg.Done()
			startMonitor(ctx, id, config)
		}(nodeID, monitorConfig)
	}
	// 打印使用说明
	fmt.Println("✅ AgentMonitor Multi-Node Test Ready!\n")
	fmt.Println("Use these URLs in your AgentManager config:\n")
	for nodeID, addr := range agentManagerConfig.MonitorURLs {
		fmt.Printf("Node %d: http://%s\n", nodeID, addr)
	}

	fmt.Println("Press Ctrl+C to stop all monitors.")

	// 等待中断信号
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	fmt.Println("\nShutting down all monitors...")
	cancel()

	// 等待所有 monitor 退出
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	// 最多等待 5 秒
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		log.Info("Timeout waiting for monitors to shutdown")
	}

	fmt.Println("All monitors stopped.")
}

func startMonitor(ctx context.Context, nodeID uint64, agentMonitorConfig *configs.MonitorConfig) {
	agentMonitor := agent_monitor.NewAgentMonitor(agentMonitorConfig)
	// 假设 NewAgentMonitor 支持 nil config
	server := agent_monitor.NewAgentMonitorHTTPServer(agentMonitor)
	// 启动 HTTP 服务（非阻塞）
	go func() {
		log.Info("[Monitor %d] Starting HTTP server on %s", nodeID, agentMonitor.Config.MonitorUrl)
		if err := server.Start(); err != nil && err != http.ErrServerClosed {
			log.Info("[Monitor %d] Server error: %v", nodeID, err)
		}
	}()

	// 等待 ctx 取消
	<-ctx.Done()

	// 这里可以扩展：优雅关闭（但 HTTP server 没有 Shutdown 方法，简单 kill 即可）
	log.Info("[Monitor %d] Stopped.", nodeID)
}
