package memspace_manager

import "sync"

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
	c.memspaces[info.MemSpaceID] = info
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
