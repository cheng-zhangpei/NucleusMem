package memspace_manager

import (
	"NucleusMem/pkg/client"
	"NucleusMem/pkg/configs"
	"sync"
)

// MemSpaceManager manages all MemSpace instances across the cluster
type MemSpaceManager struct {
	memSpaceCache      *MemSpaceCache
	agentMonitorClient map[uint64]*client.MemSpaceMonitorClient
	monitorURLs        map[uint64]string
	mu                 sync.RWMutex
}

// NewMemSpaceManager creates a new MemSpaceManager instance
func NewMemSpaceManager(config *configs.MemSpaceManagerConfig) (*MemSpaceManager, error) {
	// Initialize cache
	cache := NewMemSpaceCache()
	// Initialize clients and URLs
	monitorClients := make(map[uint64]*client.MemSpaceMonitorClient)
	monitorURLs := make(map[uint64]string)

	for nodeID, addr := range config.MonitorURLs {
		monitorURLs[nodeID] = addr
		client := client.NewMemSpaceMonitorClient(addr)
		monitorClients[nodeID] = client
	}

	manager := &MemSpaceManager{
		memSpaceCache:      cache,
		agentMonitorClient: monitorClients,
		monitorURLs:        monitorURLs,
	}

	return manager, nil
}
