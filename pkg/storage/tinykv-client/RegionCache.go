package tinykv_client

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/pingcap-incubator/tinykv/proto/pkg/metapb"
	"github.com/pingcap-incubator/tinykv/proto/pkg/schedulerpb"
)

// RegionInfo region cache
type RegionInfo struct {
	Region *metapb.Region
	Leader *metapb.Peer // 缓存 Leader 也是关键，不然总要重试 NotLeader
}

type RegionCache struct {
	pdClient schedulerpb.SchedulerClient
	mu       sync.RWMutex
	// regions Sort by StartKey
	regions []*RegionInfo
	// storeAddrs save StoreID -> Address
	storeAddrs map[uint64]string
}

func NewRegionCache(pdClient schedulerpb.SchedulerClient) *RegionCache {
	return &RegionCache{
		pdClient:   pdClient,
		regions:    make([]*RegionInfo, 0),
		storeAddrs: make(map[uint64]string),
	}
}

// LocateRegion core：give Key，find the addr of  Region and Leader 的地址
func (c *RegionCache) LocateRegion(ctx context.Context, key []byte) (*RegionInfo, string, error) {
	c.mu.RLock()
	// 1.  binary search
	idx := sort.Search(len(c.regions), func(i int) bool {
		return bytes.Compare(c.regions[i].Region.EndKey, key) > 0 || len(c.regions[i].Region.EndKey) == 0
	})

	var target *RegionInfo
	if idx < len(c.regions) {
		r := c.regions[idx]
		//  StartKey <= key
		if bytes.Compare(r.Region.StartKey, key) <= 0 {
			target = r
		}
	}
	c.mu.RUnlock()

	// 2. cache can not find ，ask PD
	if target == nil {
		var err error
		target, err = c.loadRegionFromPD(ctx, key)
		if err != nil {
			return nil, "", err
		}
	}

	// 3. Resolve Leader Address
	// If no Leader information is available, arbitrarily select a Peer (to have it inform us it is NotLeader)
	// Or default to the first Peer found via PD lookup
	peer := target.Leader
	if peer == nil && len(target.Region.Peers) > 0 {
		peer = target.Region.Peers[0]
	}
	if peer == nil {
		return nil, "", fmt.Errorf("no peer available for region %d", target.Region.Id)
	}

	addr, err := c.getStoreAddr(ctx, peer.StoreId)
	if err != nil {
		return nil, "", err
	}

	return target, addr, nil
}

// load Region from PD
func (c *RegionCache) loadRegionFromPD(ctx context.Context, key []byte) (*RegionInfo, error) {
	req := &schedulerpb.GetRegionRequest{
		RegionKey: key,
	}
	resp, err := c.pdClient.GetRegion(ctx, req)
	if err != nil {
		return nil, err
	}
	if resp.Region == nil {
		return nil, fmt.Errorf("region not found for key %v", key)
	}

	info := &RegionInfo{
		Region: resp.Region,
		Leader: resp.Leader,
	}

	c.updateCache(info)
	return info, nil
}

// update cache
func (c *RegionCache) updateCache(info *RegionInfo) {
	c.mu.Lock()
	defer c.mu.Unlock()

	newRegions := make([]*RegionInfo, 0, len(c.regions)+1)
	for _, r := range c.regions {
		if isOverlap(r.Region, info.Region) {
			continue
		}
		newRegions = append(newRegions, r)
	}

	newRegions = append(newRegions, info)
	sort.Slice(newRegions, func(i, j int) bool {
		return bytes.Compare(newRegions[i].Region.StartKey, newRegions[j].Region.StartKey) < 0
	})

	c.regions = newRegions
}

func isOverlap(r1, r2 *metapb.Region) bool {
	// Conditions for overlap between two intervals [s1, e1) and [s2, e2): s1 < e2 && s2 < e1
	// Note: An empty EndKey indicates infinity
	return (len(r2.EndKey) == 0 || bytes.Compare(r1.StartKey, r2.EndKey) < 0) &&
		(len(r1.EndKey) == 0 || bytes.Compare(r2.StartKey, r1.EndKey) < 0)
}

// get Store addr (with cache)
func (c *RegionCache) getStoreAddr(ctx context.Context, storeID uint64) (string, error) {
	c.mu.RLock()
	addr, ok := c.storeAddrs[storeID]
	c.mu.RUnlock()
	if ok {
		return addr, nil
	}

	// Cache Miss，ask PD
	resp, err := c.pdClient.GetStore(ctx, &schedulerpb.GetStoreRequest{StoreId: storeID})
	if err != nil {
		return "", err
	}
	if resp.Store == nil {
		return "", fmt.Errorf("store %d not found", storeID)
	}

	c.mu.Lock()
	c.storeAddrs[storeID] = resp.Store.Address
	c.mu.Unlock()
	return resp.Store.Address, nil
}

// InvalidateCache clear cache
func (c *RegionCache) InvalidateCache(key []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Find the region containing the key and remove it
	// Simple implementation: Clear everything (it'll be pulled again next time anyway)
	// Optimized implementation: Remove only the one at idx
	// c.regions = ...

	// For demonstration purposes, we only set Leader to nil, forcing a lookup at PD next time
	// Or directly remove it from the slice
	// Lazy approach: c.regions = nil (too aggressive)

	// Correct approach: Find and delete
	idx := sort.Search(len(c.regions), func(i int) bool {
		return bytes.Compare(c.regions[i].Region.EndKey, key) > 0 || len(c.regions[i].Region.EndKey) == 0
	})
	if idx < len(c.regions) && bytes.Compare(c.regions[idx].Region.StartKey, key) <= 0 {
		// delete idx
		c.regions = append(c.regions[:idx], c.regions[idx+1:]...)
	}
}

// UpdateLeader when encounter NotLeader, update Leader
func (c *RegionCache) UpdateLeader(regionID uint64, leader *metapb.Peer) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, r := range c.regions {
		if r.Region.Id == regionID {
			r.Leader = leader
			return
		}
	}
}
