// cmd/agent/main.go
package main

import (
	"NucleusMem/pkg/runtime/agent"
	"flag"
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

	// Parse flags
	configFile := flag.String("config", "pkg/configs/file/agent.yaml", "Base agent config file")
	numAgents := flag.Int("num", 4, "Number of agents to launch")
	basePort := flag.Int("base-port", 9001, "Base port for agents (e.g., 9001, 9002, ...)")
	flag.Parse()

	// Load base config
	baseCfg, err := configs.LoadAgentConfigFromYAML(*configFile)
	if err != nil {
		log.Fatalf("Failed to load base agent config: %v", err)
	}

	var wg sync.WaitGroup
	agents := make([]*agent.Agent, *numAgents)
	servers := make([]*agent.AgentHTTPServer, *numAgents)

	// Launch multiple agents
	for i := 0; i < *numAgents; i++ {
		wg.Add(1)
		go func(agentIndex int) {
			defer wg.Done()
			// Create agent config
			cfg := *baseCfg // Copy base config
			cfg.AgentId = baseCfg.AgentId + uint64(agentIndex)
			cfg.HttpAddr = fmt.Sprintf(":%d", *basePort+agentIndex)
			// Set role based on index
			if agentIndex%2 == 0 {
				cfg.IsJob = false
				cfg.Role = "ResidentAgent"
			} else {
				cfg.IsJob = true
				cfg.Role = "JobAgent"
			}
			// Override MemSpace addresses to avoid conflicts
			if cfg.PrivateMemSpaceInfo != nil {
				cfg.PrivateMemSpaceInfo.MemSpaceId = 1000 + uint64(agentIndex+1)
				cfg.PrivateMemSpaceInfo.MemSpaceAddr = fmt.Sprintf("http://localhost:%d", 8000+agentIndex+1)
			}
			// Create agent
			agt, err := agent.NewAgent(&cfg)
			if err != nil {
				log.Errorf("Failed to create agent %d: %v", agentIndex+1, err)
				return
			}
			agents[agentIndex] = agt
			// Start HTTP server
			server := agent.NewAgentHTTPServer(agt)
			servers[agentIndex] = server
			log.Infof("🤖 Starting Agent %d (ID=%d, Job=%v) on %s",
				agentIndex+1, cfg.AgentId, cfg.IsJob, cfg.HttpAddr)
			// This will block the goroutine, but that's okay for demo
			// In production, you'd want proper signal handling
			server.Start(cfg.HttpAddr)
		}(i)
	}
	log.Infof(" Launched %d agents (base port: %d)", *numAgents, *basePort)
	// Wait for all agents (this will actually never return because servers block)
	wg.Wait()
}
