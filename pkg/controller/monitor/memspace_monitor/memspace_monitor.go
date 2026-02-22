package memspace_monitor

import (
	"NucleusMem/pkg/api"
	"NucleusMem/pkg/client"
	"NucleusMem/pkg/configs"
	"context"
	"fmt"
	"github.com/pingcap-incubator/tinykv/log"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// MemSpaceInfo represents a registered MemSpace instance
type MemSpaceInfo struct {
	MemSpaceID  uint64
	Name        string
	OwnerID     uint64
	Type        string
	Status      string
	Addr        string
	Description string
	LastSeen    int64
}

// MemSpaceMonitor manages connections to MemSpaces
type MemSpaceMonitor struct {
	id        uint64
	Config    *configs.MemSpaceMonitorConfig
	mu        sync.RWMutex
	memspaces map[uint64]*MemSpaceInfo          // Registered MemSpaces
	clients   map[uint64]*client.MemSpaceClient // HTTP clients

	ctx             context.Context
	cancel          context.CancelFunc
	wg              sync.WaitGroup
	isUpdateManager bool
	managerClient   *client.MemSpaceManagerClient
}

// NewMemSpaceMonitor creates a new monitor
func NewMemSpaceMonitor(config *configs.MemSpaceMonitorConfig) *MemSpaceMonitor {
	var managerClient *client.MemSpaceManagerClient
	if config.MemSpaceManagerURL != "" {
		managerClient = client.NewMemSpaceManagerClient(config.MemSpaceManagerURL)
	}
	ctx, cancel := context.WithCancel(context.Background())
	mm := &MemSpaceMonitor{
		id:              config.NodeID,
		Config:          config,
		memspaces:       make(map[uint64]*MemSpaceInfo),
		clients:         make(map[uint64]*client.MemSpaceClient),
		isUpdateManager: false,
		ctx:             ctx,
		cancel:          cancel,
		managerClient:   managerClient,
	}
	mm.startSyncWorker()
	return mm
}

// RegisterMemSpace registers a locally running MemSpace
func (mm *MemSpaceMonitor) RegisterMemSpace(memspaceID uint64, name string, ownerID uint64, memspaceType string, addr string) error {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	if _, exists := mm.memspaces[memspaceID]; exists {
		return fmt.Errorf("memspace %d already registered", memspaceID)
	}

	// Create HTTP client
	baseURL := addr
	if !strings.HasPrefix(addr, "http://") && !strings.HasPrefix(addr, "https://") {
		baseURL = "http://" + addr
	}
	client := client.NewMemSpaceClient(baseURL)

	// Store info
	mm.memspaces[memspaceID] = &MemSpaceInfo{
		MemSpaceID: memspaceID,
		Name:       name,
		OwnerID:    ownerID,
		Type:       memspaceType,
		Status:     "active",
		Addr:       addr,
	}
	mm.clients[memspaceID] = client

	return nil
}

// ConnectToMemSpace connects to a remote MemSpace
func (mm *MemSpaceMonitor) ConnectToMemSpace(memspaceID uint64, addr string) error {
	return mm.RegisterMemSpace(memspaceID, "", 0, "public", addr)
}

// GetMemSpaceClient returns the client for a given MemSpace ID
func (mm *MemSpaceMonitor) GetMemSpaceClient(memspaceID uint64) (*client.MemSpaceClient, bool) {
	mm.mu.RLock()
	defer mm.mu.RUnlock()
	client, ok := mm.clients[memspaceID]
	return client, ok
}

// ListMemSpaces returns all registered MemSpaces
func (mm *MemSpaceMonitor) ListMemSpaces() []*MemSpaceInfo {
	mm.mu.RLock()
	defer mm.mu.RUnlock()

	result := make([]*MemSpaceInfo, 0, len(mm.memspaces))
	for _, info := range mm.memspaces {
		// Copy to avoid external modification
		copied := *info
		result = append(result, &copied)
	}
	return result
}

// UnregisterMemSpace removes a MemSpace from the monitor
func (mm *MemSpaceMonitor) UnregisterMemSpace(memspaceID uint64) error {
	mm.mu.Lock()
	defer mm.mu.Unlock()
	if _, exists := mm.memspaces[memspaceID]; !exists {
		return fmt.Errorf("memspace %d not found", memspaceID)
	}
	delete(mm.memspaces, memspaceID)
	delete(mm.clients, memspaceID)
	return nil
}

func (mm *MemSpaceMonitor) startSyncWorker() {
	mm.wg.Add(1)
	go func() {
		defer mm.wg.Done()
		ticker := time.NewTicker(10 * time.Second) // sync every 10 seconds
		defer ticker.Stop()

		for {
			select {
			case <-mm.ctx.Done():
				return
			case <-ticker.C:
				mm.syncAllMemSpaces()
			}
		}
	}()
}

func (mm *MemSpaceMonitor) syncAllMemSpaces() {
	mm.mu.RLock()
	ids := make([]uint64, 0, len(mm.memspaces))
	for id := range mm.memspaces {
		ids = append(ids, id)
	}
	mm.mu.RUnlock()

	for _, id := range ids {
		mm.syncMemSpace(id)
	}
	if mm.isUpdateManager {
		mm.reportToManager()
		mm.isUpdateManager = false
	}
}

func (mm *MemSpaceMonitor) syncMemSpace(memspaceID uint64) {
	client, ok := mm.GetMemSpaceClient(memspaceID)
	if !ok {
		return
	}
	health, err := client.HealthCheckWithInfo()
	if err != nil {
		// tag inactive
		mm.updateMemSpaceStatus(memspaceID, "inactive")
		mm.isUpdateManager = true
		return
	}
	mm.updateMemSpaceInfo(memspaceID, health)
}

func (mm *MemSpaceMonitor) updateMemSpaceInfo(memspaceID uint64, health *api.MemSpaceHealthResponse) {
	mm.mu.Lock()
	defer mm.mu.Unlock()
	info, exists := mm.memspaces[memspaceID]
	if !exists {
		return
	}
	// Check if any field has changed
	hasChanged := false
	if info.Name != health.Name ||
		info.Type != health.Type ||
		info.OwnerID != health.OwnerID ||
		info.Description != health.Description ||
		info.Status != "active" || // Status always becomes "active" on success
		info.LastSeen != health.Timestamp {
		hasChanged = true
	}
	// Only update and mark if there's a change
	if hasChanged {
		info.Name = health.Name
		info.Type = health.Type
		info.OwnerID = health.OwnerID
		info.Description = health.Description
		info.Status = "active"
		info.LastSeen = health.Timestamp
		mm.isUpdateManager = true
	}
}

func (mm *MemSpaceMonitor) updateMemSpaceStatus(memspaceID uint64, status string) {
	mm.mu.Lock()
	defer mm.mu.Unlock()
	if info, exists := mm.memspaces[memspaceID]; exists {
		info.Status = status
	}
}

func (mm *MemSpaceMonitor) reportToManager() {
	if mm.managerClient == nil {
		return // No manager configured
	}
	// Get current memspace list
	memspaces := mm.ListMemSpaces()
	// Convert to API format
	updates := make([]api.MemSpaceInfo, len(memspaces))
	for i, ms := range memspaces {
		updates[i] = api.MemSpaceInfo{
			MemSpaceID:  fmt.Sprintf("%d", ms.MemSpaceID),
			Name:        ms.Name,
			OwnerID:     fmt.Sprintf("%d", ms.OwnerID),
			Type:        ms.Type,
			Status:      ms.Status,
			HttpAddr:    ms.Addr,
			Description: ms.Description,
			LastSeen:    ms.LastSeen,
		}
	}
	// Send to manager

	err := mm.managerClient.NotifyMemSpaceUpdate(updates)
	if err != nil {
		log.Warnf("Failed to notify manager: %v", err)
	} else {
		//log.Infof("Successfully reported %d memspaces to manager", len(updates))
	}
}

// LaunchMemSpace launches a MemSpace process
func (mm *MemSpaceMonitor) LaunchMemSpace(req *api.LaunchMemSpaceRequest) (*MemSpaceInfo, error) {
	// Step 1: Load config to get MemSpaceID
	memspaceCfg, err := configs.LoadMemSpaceConfigFromYAML(req.ConfigFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to load memspace config: %w", err)
	}

	mm.mu.Lock()
	defer mm.mu.Unlock()

	if _, exists := mm.memspaces[memspaceCfg.MemSpaceID]; exists {
		return nil, fmt.Errorf("memspace %d already running", memspaceCfg.MemSpaceID)
	}

	log.Infof("[memspace_monitor] Launching MemSpace ID=%d using config: %s",
		memspaceCfg.MemSpaceID, req.ConfigFilePath)

	// Step 2: Start process
	cmd := exec.Command(req.BinPath, "--config", req.ConfigFilePath)

	if req.Env != nil {
		cmd.Env = os.Environ()
		for k, v := range req.Env {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
		}
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start memspace process: %w", err)
	}

	// Step 3: Wait for startup
	time.Sleep(800 * time.Millisecond)

	// Step 4: Create client and health check
	httpAddr := memspaceCfg.HttpAddr
	baseURL := httpAddr
	if !strings.HasPrefix(httpAddr, "http://") && !strings.HasPrefix(httpAddr, "https://") {
		baseURL = "http://" + httpAddr
	}

	memspaceClient := client.NewMemSpaceClient(baseURL)
	_, err = memspaceClient.HealthCheckWithInfo()
	if err != nil {
		log.Warnf("MemSpace %d health check failed: %v", memspaceCfg.MemSpaceID, err)
	}

	// Step 5: Register in monitor
	info := &MemSpaceInfo{
		MemSpaceID:  memspaceCfg.MemSpaceID,
		Name:        memspaceCfg.Name,
		OwnerID:     memspaceCfg.OwnerID,
		Type:        memspaceCfg.Type,
		Status:      "active",
		Addr:        httpAddr,
		Description: memspaceCfg.Description,
	}

	mm.memspaces[memspaceCfg.MemSpaceID] = info
	mm.clients[memspaceCfg.MemSpaceID] = memspaceClient

	log.Infof("[memspace_monitor] Successfully launched MemSpace %d at %s",
		memspaceCfg.MemSpaceID, httpAddr)
	return info, nil
}

// StopMemSpace stops a MemSpace process
func (mm *MemSpaceMonitor) StopMemSpace(memspaceID uint64) error {
	mm.mu.Lock()
	memspaceClient, exists := mm.clients[memspaceID]
	mm.mu.Unlock()

	if !exists {
		return fmt.Errorf("memspace %d not found", memspaceID)
	}

	// Step 1: Notify MemSpace to shutdown
	// Note: MemSpace HTTP server needs a /shutdown endpoint
	err := memspaceClient.Shutdown()
	if err != nil {
		log.Warnf("[memspace_monitor] Failed to shutdown MemSpace %d: %v", memspaceID, err)
	}

	// Step 2: Clean local state
	mm.mu.Lock()
	defer mm.mu.Unlock()
	delete(mm.memspaces, memspaceID)
	delete(mm.clients, memspaceID)
	log.Infof("[memspace_monitor] Stopped MemSpace ID=%d", memspaceID)
	return nil
}
