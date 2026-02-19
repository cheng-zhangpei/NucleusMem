package memspace_manager

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"NucleusMem/pkg/client"
	"NucleusMem/pkg/configs"
	"github.com/pingcap-incubator/tinykv/log"
)

// MemSpaceManager manages all MemSpace instances across the cluster
type MemSpaceManager struct {
	memSpaceCache         *MemSpaceCache
	memSpaceMonitorClient map[uint64]*client.MemSpaceMonitorClient
	monitorURLs           map[uint64]string
	mu                    sync.RWMutex
}

// NewMemSpaceManager creates a new MemSpaceManager instance
func NewMemSpaceManager(config *configs.MemSpaceManagerConfig) (*MemSpaceManager, error) {
	cache := NewMemSpaceCache()
	monitorClients := make(map[uint64]*client.MemSpaceMonitorClient)
	monitorURLs := make(map[uint64]string)

	for nodeID, addr := range config.MonitorURLs {
		monitorURLs[nodeID] = addr
		client := client.NewMemSpaceMonitorClient(addr)
		monitorClients[nodeID] = client
	}

	manager := &MemSpaceManager{
		memSpaceCache:         cache,
		memSpaceMonitorClient: monitorClients,
		monitorURLs:           monitorURLs,
	}
	manager.LoadMemSpaceInfo()
	return manager, nil
}

// LoadMemSpaceInfo loads all memspace info from monitors into cache
func (mm *MemSpaceManager) LoadMemSpaceInfo() {
	mm.mu.RLock()
	clients := make(map[uint64]*client.MemSpaceMonitorClient)
	for k, v := range mm.memSpaceMonitorClient {
		clients[k] = v
	}
	mm.mu.RUnlock()

	for nodeID, client := range clients {
		memspaces, err := client.ListMemSpaces()
		if err != nil {
			log.Warnf("Failed to load memspaces from monitor %d: %v", nodeID, err)
			continue
		}

		for _, ms := range memspaces {
			memspaceID, _ := strconv.ParseUint(ms.MemSpaceID, 10, 64)
			ownerID, _ := strconv.ParseUint(ms.OwnerID, 10, 64)

			cacheInfo := &MemSpaceInfo{
				MemSpaceID:   memspaceID,
				Name:         ms.Name,
				OwnerAgentID: ownerID,
				Type:         ms.Type,
				NodeID:       nodeID,
				HttpAddr:     ms.HttpAddr, // ← Already full address
				Status:       ms.Status,
				CreatedAt:    ms.LastSeen,
				Metadata:     nil,
			}
			mm.memSpaceCache.UpdateMemSpace(cacheInfo)
		}
	}
}

// ListMemSpaceInfo returns all cached memspace info
func (mm *MemSpaceManager) ListMemSpaceInfo() []*MemSpaceInfo {
	return mm.memSpaceCache.GetAllMemSpaces()
}

// BindMemSpaceToAgent binds an agent to a memspace (direct call to memspace)
func (mm *MemSpaceManager) BindMemSpaceToAgent(agentID, memspaceID uint64) error {
	info, ok := mm.memSpaceCache.GetMemSpace(memspaceID)
	if !ok {
		return fmt.Errorf("memspace %d not found in cache", memspaceID)
	}

	// Use HttpAddr directly (it's already the full address)
	memspaceAddr := info.HttpAddr
	if !strings.HasPrefix(memspaceAddr, "http://") && !strings.HasPrefix(memspaceAddr, "https://") {
		memspaceAddr = "http://" + memspaceAddr
	}

	memSpaceClient := client.NewMemSpaceClient(memspaceAddr)
	return memSpaceClient.BindAgent(agentID)
}

// UnBindMemSpaceWithAgent unbinds an agent from a memspace (direct call)
func (mm *MemSpaceManager) UnBindMemSpaceWithAgent(agentID, memspaceID uint64) error {
	info, ok := mm.memSpaceCache.GetMemSpace(memspaceID)
	if !ok {
		return fmt.Errorf("memspace %d not found in cache", memspaceID)
	}

	memspaceAddr := info.HttpAddr
	if !strings.HasPrefix(memspaceAddr, "http://") && !strings.HasPrefix(memspaceAddr, "https://") {
		memspaceAddr = "http://" + memspaceAddr
	}

	client := client.NewMemSpaceClient(memspaceAddr)
	return client.UnbindAgent(agentID)
}

// LaunchMemspace launches a new memspace via monitor
func (mm *MemSpaceManager) LaunchMemspace(config *configs.MemSpaceConfig) error {
	// Select a monitor (simple: first available)
	mm.mu.RLock()
	var targetMonitor *client.MemSpaceMonitorClient
	var targetNodeID uint64
	for nodeID, monitor := range mm.memSpaceMonitorClient {
		targetMonitor = monitor
		targetNodeID = nodeID
		break
	}
	mm.mu.RUnlock()

	if targetMonitor == nil {
		return fmt.Errorf("no available monitor")
	}

	// Launch via monitor
	_, addr, err := targetMonitor.LaunchMemSpace(
		config.BinPath,
		config.ConfigFilePath,
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to launch memspace on monitor %d: %w", targetNodeID, err)
	}

	// Update cache
	cacheInfo := &MemSpaceInfo{
		MemSpaceID:   config.MemSpaceID,
		Name:         config.Name,
		OwnerAgentID: config.OwnerID,
		Type:         config.Type,
		NodeID:       targetNodeID,
		HttpAddr:     addr, // ← Use returned address
		Status:       "active",
		CreatedAt:    time.Now().Unix(),
	}
	mm.memSpaceCache.UpdateMemSpace(cacheInfo)

	return nil
}

// ShutDownMemspace shuts down a memspace via monitor
func (mm *MemSpaceManager) ShutDownMemspace(config *configs.MemSpaceConfig) error {
	// Find which monitor hosts this memspace
	// (In real implementation, track this in cache)
	mm.mu.RLock()
	monitors := make(map[uint64]*client.MemSpaceMonitorClient)
	for k, v := range mm.memSpaceMonitorClient {
		monitors[k] = v
	}
	mm.mu.RUnlock()

	for nodeID, monitor := range monitors {
		err := monitor.StopMemSpace(config.MemSpaceID)
		if err == nil {
			// Successfully stopped
			mm.memSpaceCache.RemoveMemSpace(config.MemSpaceID)
			return nil
		}
		log.Debugf("Monitor %d failed to stop memspace: %v", nodeID, err)
	}

	return fmt.Errorf("memspace %d not found on any monitor", config.MemSpaceID)
}
