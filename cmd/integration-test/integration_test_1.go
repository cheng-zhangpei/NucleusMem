// cmd/integration-test/monitor_connect_test.go
package main

import (
	"NucleusMem/pkg/controller/agent_manager"
	"context"
	"fmt"
	"os"
	"time"

	"NucleusMem/pkg/configs"
	"NucleusMem/pkg/controller/monitor/agent_monitor"
	"github.com/pingcap-incubator/tinykv/log"
)

func main() {
	if wd, err := os.Getwd(); err != nil {
		fmt.Printf("Failed to get working directory: %v\n", err)
	} else {
		fmt.Printf("🔧 Current working directory: %s\n", wd)
	}

	// === Step 1: Create Monitor ===
	monitorConfig := &configs.MonitorConfig{
		NodeID:     1,
		MonitorUrl: "localhost:8081",
	}
	monitor := agent_monitor.NewAgentMonitor(monitorConfig)

	// Connect to running agents
	agents := []struct {
		id   uint64
		addr string
	}{
		{101, "localhost:9001"},
		{102, "localhost:9002"},
		{103, "localhost:9003"},
		{104, "localhost:9004"},
	}

	log.Infof("🧪 Connecting Monitor (ID=1) to %d agents...", len(agents))
	for _, agent := range agents {
		err := monitor.ConnectToAgent(agent.id, agent.addr)
		if err != nil {
			log.Errorf(" Failed to connect to Agent %d at %s: %v", agent.id, agent.addr, err)
		} else {
			log.Infof(" Successfully connected to Agent %d at %s", agent.id, agent.addr)
		}
	}
	// Start Monitor HTTP server
	server := agent_monitor.NewAgentMonitorHTTPServer(monitor)
	go func() {
		log.Infof("Monitor HTTP server starting on %s", monitorConfig.MonitorUrl)
		if err := server.Start(); err != nil {
			log.Errorf("Monitor server error: %v", err)
		}
	}()

	// Give server time to start
	time.Sleep(500 * time.Millisecond)
	// === Step 2: Create AgentManager ===
	// Create a mock agent manager config that points to our monitor
	managerConfig := &configs.AgentManagerConfig{
		MonitorURLs: map[uint64]string{
			1: "localhost:8081", // Point to our in-process monitor
		},
	}

	log.Infof(" Creating AgentManager...")
	manager, err := agent_manager.NewAgentManager(managerConfig)
	if err != nil {
		log.Fatalf("Failed to create AgentManager: %v", err)
	}

	// === Step 3: Test Stop Agent via Manager ===
	manager.DisplayCache()

	ctx := context.Background()
	err = manager.StopAgentOnNode(ctx, 1, 101)

	manager.DisplayCache()

	if err != nil {
		log.Errorf(" Failed to stop Agent 101 via Manager: %v", err)
	} else {
		log.Infof("Successfully stopped Agent 101 via Manager!")
	}

	select {}
}
