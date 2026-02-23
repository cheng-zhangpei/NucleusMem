package memspace_manager

import (
	"github.com/pingcap-incubator/tinykv/log"
	"sync"
)

// MemSpaceInfo represents cached information about a MemSpace
type MemSpaceInfo struct {
	MemSpaceID   uint64            `json:"memspace_id"`
	Name         string            `json:"name"`
	OwnerAgentID uint64            `json:"owner_agent_id"`
	Type         string            `json:"type"` // "private", "public", "shared"
	NodeID       uint64            `json:"node_id"`
	HttpAddr     string            `json:"addr"`
	Status       string            `json:"status"` // "active", "inactive"
	CreatedAt    int64             `json:"created_at"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// MemSpaceCache is a thread-safe cache for MemSpace metadata
type MemSpaceCache struct {
	mu        sync.RWMutex
	memspaces map[uint64]*MemSpaceInfo
}

// NewMemSpaceCache creates a new MemSpace cache
func NewMemSpaceCache() *MemSpaceCache {
	return &MemSpaceCache{
		memspaces: make(map[uint64]*MemSpaceInfo),
	}
}

// UpdateMemSpace updates or adds a MemSpace to the cache
func (c *MemSpaceCache) UpdateMemSpace(info *MemSpaceInfo) {
	c.mu.Lock()
	defer c.mu.Unlock()
	isNew := false
	if _, exists := c.memspaces[info.MemSpaceID]; !exists {
		isNew = true
	}
	c.memspaces[info.MemSpaceID] = info
	if isNew {
		log.Infof(
			"[MemSpaceCache] ➕ ADDED    | ID: %-6d | Name: %-20s | Type: %-8s | Owner: %-6d | Node: %-4d | Status: %-8s | Addr: %s",
			info.MemSpaceID,
			info.Name,
			info.Type,
			info.OwnerAgentID,
			info.NodeID,
			info.Status,
			info.HttpAddr,
		)
	} else {
		return
	}
}

// GetMemSpace retrieves a MemSpace from the cache
func (c *MemSpaceCache) GetMemSpace(memspaceID uint64) (*MemSpaceInfo, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	info, ok := c.memspaces[memspaceID]
	return info, ok
}

// GetAllMemSpaces returns all cached MemSpaces
func (c *MemSpaceCache) GetAllMemSpaces() []*MemSpaceInfo {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make([]*MemSpaceInfo, 0, len(c.memspaces))
	for _, info := range c.memspaces {
		result = append(result, info)
	}
	return result
}

// RemoveMemSpace removes a MemSpace from the cache
func (c *MemSpaceCache) RemoveMemSpace(memspaceID uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.memspaces, memspaceID)
}
