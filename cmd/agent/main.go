// cmd/agent/main.go
package main

import (
	"NucleusMem/pkg/runtime/agent"
	"fmt"
	"os"
	"sync"

	"NucleusMem/pkg/configs"
	"github.com/pingcap-incubator/tinykv/log"
)

func main() {
	// Print working directory
	if wd, err := os.Getwd(); err != nil {
		fmt.Printf("Failed to get working directory: %v\n", err)
	} else {
		fmt.Printf("🔧 Current working directory: %s\n", wd)
	}

	// List of agent config files (for testing)
	configFiles := []string{
		"./pkg/configs/file/agent_101.yaml",
		"./pkg/configs/file/agent_102.yaml",
		"./pkg/configs/file/agent_103.yaml",
		"./pkg/configs/file/agent_104.yaml",
	}

	var wg sync.WaitGroup

	for _, configFile := range configFiles {
		wg.Add(1)
		go func(cfgFile string) {
			defer wg.Done()

			// Load config
			cfg, err := configs.LoadAgentConfigFromYAML(cfgFile)
			if err != nil {
				log.Errorf("Failed to load config %s: %v", cfgFile, err)
				return
			}

			// Create agent instance
			agt, err := agent.NewAgent(cfg)
			if err != nil {
				log.Errorf("Failed to create agent from %s: %v", cfgFile, err)
				return
			}

			// Start HTTP server
			server := agent.NewAgentHTTPServer(agt)
			log.Infof("🤖 Starting Agent (ID=%d, Job=%v) on %s", cfg.AgentId, cfg.IsJob, cfg.HttpAddr)

			// This blocks the goroutine, but that's fine for demo
			if err := server.Start(cfg.HttpAddr); err != nil {
				log.Errorf("Agent %d server failed: %v", cfg.AgentId, err)
			}
		}(configFile)
	}

	log.Infof("🚀 All 4 agents started! Press Ctrl+C to stop.")
	wg.Wait()
}
