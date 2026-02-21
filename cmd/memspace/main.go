package main

import (
	"NucleusMem/pkg/configs"
	"NucleusMem/pkg/runtime/memspace"
	"flag"
	"github.com/pingcap-incubator/tinykv/log"
	"os"
)

func main() {
	configPath := flag.String("config", "", "Path to agent configuration file (required)")
	flag.Parse()

	// Validate required flag
	if *configPath == "" {
		log.Infof("Error: --config flag is required")
		log.Infof("Usage: agent --config /path/to/agent-config.yaml")
		os.Exit(1)
	}
	// Print working directory
	if wd, err := os.Getwd(); err != nil {
		log.Infof("Failed to get working directory: %v", err)
	} else {
		log.Infof("Current working directory: %s", wd)
	}
	config, err := configs.LoadMemSpaceConfigFromYAML(*configPath)
	if err != nil {
		panic(err)
	}
	mp, err := memspace.NewMemSpace(config)
	if err != nil {
		panic(err)
	}
	httpServer := memspace.NewMemSpaceHTTPServer(mp)
	err = httpServer.Start()
	if err != nil {
		panic(err)
	}
}
