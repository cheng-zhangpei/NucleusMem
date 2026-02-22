package memspace_manager

import (
	"context"
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

	failedMonitors map[uint64]string // 失败的 Monitor (nodeID -> URL)
	ctx            context.Context
	cancel         context.CancelFunc
	wg             sync.WaitGroup
}

// NewMemSpaceManager creates a new MemSpaceManager instance
func NewMemSpaceManager(config *configs.MemSpaceManagerConfig) (*MemSpaceManager, error) {
	cache := NewMemSpaceCache()
	monitorClients := make(map[uint64]*client.MemSpaceMonitorClient)
	monitorURLs := make(map[uint64]string)
	failedMonitors := make(map[uint64]string)

	ctx, cancel := context.WithCancel(context.Background())

	manager := &MemSpaceManager{
		memSpaceCache:         cache,
		memSpaceMonitorClient: monitorClients,
		monitorURLs:           monitorURLs,
		failedMonitors:        failedMonitors,
		ctx:                   ctx,
		cancel:                cancel,
	}

	// Step 1: 初始化时尝试连接所有 Monitor
	for nodeID, addr := range config.MonitorURLs {
		manager.monitorURLs[nodeID] = addr

		client := client.NewMemSpaceMonitorClient(addr)

		// Health Check
		if err := client.HealthCheck(); err != nil {
			log.Warnf("Monitor %d health check failed: %v (will retry)", nodeID, err)
			manager.failedMonitors[nodeID] = addr
		} else {
			log.Infof("Monitor %d connected successfully", nodeID)
			manager.memSpaceMonitorClient[nodeID] = client
		}
	}

	// Step 2: 启动后台重试线程
	manager.startRetryWorker()
	// Step 3: 加载已连接 Monitor 的 MemSpace 信息
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
	log.Debugf("[manager] receive bind memspace to agent %d", agentID)
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

// DisplayMemSpaces prints all cached memspace info to the log (for debugging/status)
func (mm *MemSpaceManager) DisplayMemSpaces() {
	memspaces := mm.ListMemSpaceInfo()

	if len(memspaces) == 0 {
		log.Info("No memspaces in manager cache.")
		return
	}

	log.Infof("=== MemSpace Manager Cache (%d entries) ===", len(memspaces))
	for _, ms := range memspaces {
		log.Infof(
			"MemSpaceID: %d | Name: %-20s | Type: %-8s | Owner: %d | NodeID: %d | Status: %s | Addr: %s",
			ms.MemSpaceID,
			ms.Name,
			ms.Type,
			ms.OwnerAgentID,
			ms.NodeID,
			ms.Status,
			ms.HttpAddr,
		)
	}
	log.Info("============================================")
}

// startRetryWorker starts a background goroutine to retry failed monitors
func (mm *MemSpaceManager) startRetryWorker() {
	mm.wg.Add(1)
	go func() {
		defer mm.wg.Done()

		ticker := time.NewTicker(30 * time.Second) // 每 30 秒重试一次
		defer ticker.Stop()

		for {
			select {
			case <-mm.ctx.Done():
				log.Info("Monitor retry worker stopped")
				return
			case <-ticker.C:
				mm.retryFailedMonitors()
			}
		}
	}()
	log.Infof("Monitor retry worker started")
}

// retryFailedMonitors retries all failed monitors
func (mm *MemSpaceManager) retryFailedMonitors() {
	mm.mu.Lock()
	// Copy failed monitors to avoid holding lock during network calls
	failedCopy := make(map[uint64]string, len(mm.failedMonitors))
	for k, v := range mm.failedMonitors {
		failedCopy[k] = v
	}
	mm.mu.Unlock()

	if len(failedCopy) == 0 {
		return
	}

	log.Infof("Retrying %d failed monitors...", len(failedCopy))

	for nodeID, addr := range failedCopy {
		select {
		case <-mm.ctx.Done():
			return
		default:
		}

		client := client.NewMemSpaceMonitorClient(addr)

		if err := client.HealthCheck(); err != nil {
			log.Debugf("Retry Monitor %d failed: %v", nodeID, err)
			continue
		}

		// Success! Move from failed to active
		mm.mu.Lock()
		if _, exists := mm.failedMonitors[nodeID]; exists {
			delete(mm.failedMonitors, nodeID)
			mm.memSpaceMonitorClient[nodeID] = client
			log.Infof("Monitor %d reconnected successfully", nodeID)
			// Load MemSpace info from newly connected monitor
			go mm.loadMemSpaceFromMonitor(nodeID, client)
		}
		mm.mu.Unlock()
	}
}

// loadMemSpaceFromMonitor loads memspace info from a specific monitor
func (mm *MemSpaceManager) loadMemSpaceFromMonitor(nodeID uint64, client *client.MemSpaceMonitorClient) {
	memspaces, err := client.ListMemSpaces()
	if err != nil {
		log.Warnf("Failed to load memspaces from monitor %d: %v", nodeID, err)
		return
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
			HttpAddr:     ms.HttpAddr,
			Status:       ms.Status,
			CreatedAt:    ms.LastSeen,
		}
		mm.memSpaceCache.UpdateMemSpace(cacheInfo)
	}
	log.Infof("Loaded %d memspaces from monitor %d", len(memspaces), nodeID)
}
