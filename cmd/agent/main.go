// Package main is the entry point for the NucleusMem Agent
package main

import (
	"flag"
	"fmt"
	"os"

	"NucleusMem/pkg/configs"
	"NucleusMem/pkg/runtime/agent"
	"github.com/pingcap-incubator/tinykv/log"
)

func main() {
	// Define command-line flag
	configPath := flag.String("config", "", "Path to agent configuration file (required)")
	flag.Parse()

	// Validate required flag
	if *configPath == "" {
		fmt.Println("Error: --config flag is required")
		fmt.Println("Usage: agent --config /path/to/agent-config.yaml")
		os.Exit(1)
	}

	// Print working directory
	if wd, err := os.Getwd(); err != nil {
		fmt.Printf("Failed to get working directory: %v\n", err)
	} else {
		fmt.Printf("🔧 Current working directory: %s\n", wd)
	}

	// Load agent configuration
	cfg, err := configs.LoadAgentConfigFromYAML(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config from %s: %v", *configPath, err)
	}

	// Create agent instance
	agt, err := agent.NewAgent(cfg)
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	// Start HTTP server
	server := agent.NewAgentHTTPServer(agt)
	log.Infof("🤖 Starting Agent (ID=%d, Role=%s, Job=%v) on %s",
		cfg.AgentId, cfg.Role, cfg.IsJob, cfg.HttpAddr)

	// This blocks until server stops
	if err := server.Start(cfg.HttpAddr); err != nil {
		log.Fatalf("Agent server failed: %v", err)
	}
}
