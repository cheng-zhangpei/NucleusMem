package main

import (
	"NucleusMem/pkg/configs"
	"NucleusMem/pkg/controller/memspace_manager"
	"flag"
	"github.com/pingcap-incubator/tinykv/log"
)

func main() {
	configPath := flag.String("config", "", "path to memspace manager config file")
	flag.Parse()
	if *configPath == "" {
		log.Fatal("Error: --config is required")
	}
	// Load config
	cfg, err := configs.LoadMemSpaceManagerConfigFromYAML(*configPath)
	if err != nil {
		log.Fatalf("Failed to load manager config: %v", err)
	}
	// Create and start manager
	mgr, err := memspace_manager.NewMemSpaceManager(cfg)
	server := memspace_manager.NewMemSpaceManagerHTTPServer(mgr)
	err = server.Start(cfg.ListenAddr)
	if err != nil {
		log.Fatalf("Failed to start http server: %v", err)
	}
}
