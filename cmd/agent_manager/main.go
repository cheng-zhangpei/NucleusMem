// Package main is the entry point for the NucleusMem AgentManager
package main

import (
	"flag"
	"os"

	"NucleusMem/pkg/configs"
	"NucleusMem/pkg/controller/agent_manager"
	"github.com/pingcap-incubator/tinykv/log"
)

func main() {
	// Print working directory
	if wd, err := os.Getwd(); err != nil {
		log.Infof("Failed to get working directory: %v", err)
	} else {
		log.Infof("Current working directory: %s", wd)
	}

	// Parse command-line flags
	configFile := flag.String("config", "./pkg/configs/file/agent_manager.yaml", "Path to agent manager configuration file")
	flag.Parse()

	// Load configuration
	cfg, err := configs.LoadAgentManagerConfigFromYAML(*configFile)
	if err != nil {
		log.Fatalf("Failed to load agent manager config from %s: %v", *configFile, err)
	}
	log.Infof("addr %s", cfg.HttpAddr)
	// Create AgentManager instance
	manager, err := agent_manager.NewAgentManager(cfg)
	if err != nil {
		log.Fatalf("Failed to create agent manager: %v", err)
	}

	// Start HTTP server
	server := agent_manager.NewAgentManagerHTTPServer(manager)
	log.Infof("Starting AgentManager on %s", cfg.HttpAddr)

	// This will block until server stops
	if err := server.Start(cfg.HttpAddr); err != nil {
		log.Fatalf("AgentManager server failed: %v", err)
	}
}
