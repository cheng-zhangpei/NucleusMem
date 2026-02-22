package main

import (
	"NucleusMem/pkg/configs"
	"NucleusMem/pkg/controller/monitor/memspace_monitor"
	"flag"
	_ "fmt"
	"github.com/pingcap-incubator/tinykv/log"
)

func main() {
	configPath := flag.String("config", "", "path to memspace monitor config file")
	flag.Parse()
	if *configPath == "" {
		log.Fatal("Error: --config is required")
	}
	// Load config
	cfg, err := configs.LoadMemSpaceMonitorConfigFromYAML(*configPath)
	if err != nil {
		log.Fatalf("Failed to load monitor config: %v", err)
	}
	// Create and start monitor
	monitor := memspace_monitor.NewMemSpaceMonitor(cfg)
	log.Infof("MemSpaceMonitor (node_id=%d) started on %s", cfg.NodeID, cfg.MonitorUrl)
	httpServer := memspace_monitor.NewMemSpaceMonitorHTTPServer(monitor)
	err = httpServer.Start(cfg.MonitorUrl)
	if err != nil {
		log.Fatalf("Failed to start http server: %v", err)
	}
}
