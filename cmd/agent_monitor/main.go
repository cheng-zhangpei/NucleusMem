// Package main is the entry point for the NucleusMem AgentMonitor
package main

import (
	"flag"
	"os"

	"NucleusMem/pkg/configs"
	"NucleusMem/pkg/controller/monitor/agent_monitor"
	"github.com/pingcap-incubator/tinykv/log"
)

func main() {
	// Print working directory
	if wd, err := os.Getwd(); err != nil {
		log.Infof("Failed to get working directory: %v\n", err)
	} else {
		log.Infof("Current working directory: %s\n", wd)
	}

	// Parse command-line flag
	configFile := flag.String("config", "./pkg/configs/file/agent_monitor_1.yaml", "Path to monitor configuration file")
	flag.Parse()

	// Load monitor configuration
	cfg, err := configs.LoadAgentMonitorConfigFromYAML(*configFile)
	if err != nil {
		log.Fatalf("Failed to load monitor config from %s: %v", *configFile, err)
	}

	// Create monitor instance
	monitor := agent_monitor.NewAgentMonitor(cfg)

	// Start HTTP server
	server := agent_monitor.NewAgentMonitorHTTPServer(monitor)
	log.Infof("📡 Starting AgentMonitor (NodeID=%d) on %s", cfg.NodeID, cfg.MonitorUrl)

	// This will block until server stops
	if err := server.Start(); err != nil {
		log.Fatalf("AgentMonitor server failed: %v", err)
	}
}
