package memspace_monitor

import (
	"NucleusMem/pkg/client"
	"NucleusMem/pkg/configs"
	"fmt"
	"strings"
	"sync"
)

// MemSpaceInfo represents a running MemSpace instance
type MemSpaceInfo struct {
	MemSpaceID uint64
	Name       string
	OwnerID    uint64 // Owner Agent ID
	Type       string // "private", "public", "shared"
	Status     string // "active", "stopped"
	Addr       string // MemSpace service address
}

// MemSpaceMonitor manages MemSpaces on a single node
type MemSpaceMonitor struct {
	id     uint64
	Config *configs.MemSpaceMonitorConfig

	mu        sync.RWMutex
	memspaces map[uint64]*MemSpaceInfo          // MemSpace instances
	clients   map[uint64]*client.MemSpaceClient // Clients for external MemSpaces
}

// NewMemSpaceMonitor creates a new MemSpaceMonitor
func NewMemSpaceMonitor(config *configs.MemSpaceMonitorConfig) *MemSpaceMonitor {
	return &MemSpaceMonitor{
		id:        config.NodeID,
		Config:    config,
		memspaces: make(map[uint64]*MemSpaceInfo),
		clients:   make(map[uint64]*client.MemSpaceClient),
	}
}

// CreateMemSpace creates a new MemSpace instance
func (mm *MemSpaceMonitor) CreateMemSpace(memspaceID uint64, name string, ownerID uint64, memspaceType string) error {
	mm.mu.Lock()
	defer mm.mu.Unlock()
	if _, exists := mm.memspaces[memspaceID]; exists {
		return fmt.Errorf("memspace %d already exists", memspaceID)
	}
	// TODO: 实际启动 MemSpace 进程/服务
	// For now, just register it in memory
	addr := fmt.Sprintf("localhost:%d", 10000+memspaceID) // Mock address
	mm.memspaces[memspaceID] = &MemSpaceInfo{
		MemSpaceID: memspaceID,
		Name:       name,
		OwnerID:    ownerID,
		Type:       memspaceType,
		Status:     "active",
		Addr:       addr,
	}

	return nil
}

// DeleteMemSpace deletes a MemSpace instance
func (mm *MemSpaceMonitor) DeleteMemSpace(memspaceID uint64) error {
	mm.mu.Lock()
	defer mm.mu.Unlock()
	if _, exists := mm.memspaces[memspaceID]; !exists {
		return fmt.Errorf("memspace %d not found", memspaceID)
	}
	// TODO: 实际停止 MemSpace 进程/服务
	delete(mm.memspaces, memspaceID)
	return nil
}

// ConnectToMemSpace connects to an externally running MemSpace
func (mm *MemSpaceMonitor) ConnectToMemSpace(memspaceID uint64, addr string) error {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	if _, exists := mm.clients[memspaceID]; exists {
		return fmt.Errorf("memspace %d already connected", memspaceID)
	}
	baseURL := addr
	if !strings.HasPrefix(addr, "http://") && !strings.HasPrefix(addr, "https://") {
		baseURL = "http://" + addr
	}
	client := client.NewMemSpaceClient(baseURL)
	// TODO: Add health check with binding(after memspace finished)
	mm.clients[memspaceID] = client
	return nil
}
