package tinykv_client

import (
	"context"
	"fmt"
	"github.com/pingcap-incubator/tinykv/proto/pkg/errorpb"
	"sync"

	"github.com/pingcap-incubator/tinykv/proto/pkg/kvrpcpb"
	"github.com/pingcap-incubator/tinykv/proto/pkg/schedulerpb"
	"github.com/pingcap-incubator/tinykv/proto/pkg/tinykvpb"
	"google.golang.org/grpc"
)

type ClusterClient struct {
	pdClient    schedulerpb.SchedulerClient
	regionCache *RegionCache

	// connection pool
	conns map[string]*grpc.ClientConn
	mu    sync.Mutex
}

func NewClusterClient(pdAddr string) (*ClusterClient, error) {
	// 1. connect PD
	conn, err := grpc.Dial(pdAddr, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}
	pdClient := schedulerpb.NewSchedulerClient(conn)

	return &ClusterClient{
		pdClient:    pdClient,
		regionCache: NewRegionCache(pdClient),
		conns:       make(map[string]*grpc.ClientConn),
	}, nil
}

func (c *ClusterClient) getConn(addr string) (tinykvpb.TinyKvClient, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if conn, ok := c.conns[addr]; ok {
		return tinykvpb.NewTinyKvClient(conn), nil
	}

	conn, err := grpc.Dial(addr, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}
	c.conns[addr] = conn
	return tinykvpb.NewTinyKvClient(conn), nil
}

// SendRequest Generic Send Function (with Retry)
// The req parameter is interface{} because it could be Get/Scan/PreWrite/Commit
func (c *ClusterClient) SendRequest(ctx context.Context, key []byte, req interface{}) (interface{}, error) {
	for {
		// 1. locate the key in region
		regionInfo, addr, err := c.regionCache.LocateRegion(ctx, key)
		if err != nil {
			return nil, err
		}

		// 2. get conn by addr
		client, err := c.getConn(addr)
		if err != nil {
			return nil, err
		}

		// 3. send the req
		var regionErr *errorpb.Error
		var resp interface{}

		switch r := req.(type) {
		case *kvrpcpb.GetRequest:
			r.Context = &kvrpcpb.Context{RegionId: regionInfo.Region.Id, Peer: regionInfo.Leader, RegionEpoch: regionInfo.Region.RegionEpoch}
			getResp, err := client.KvGet(ctx, r)
			if err != nil {
				return nil, err
			}
			regionErr = getResp.RegionError
			resp = getResp

		case *kvrpcpb.ScanRequest:
			r.Context = &kvrpcpb.Context{RegionId: regionInfo.Region.Id, Peer: regionInfo.Leader, RegionEpoch: regionInfo.Region.RegionEpoch}
			getResp, err := client.KvScan(ctx, r)
			if err != nil {
				return nil, err
			}
			regionErr = getResp.RegionError
			resp = getResp
		}
		// 4. error handle
		if regionErr != nil {
			// NotLeader -> Update Leader Cache -> Retry
			if regionErr.NotLeader != nil {
				if regionErr.NotLeader.Leader != nil {
					c.regionCache.UpdateLeader(regionInfo.Region.Id, regionErr.NotLeader.Leader)
				} else {
					// Leader unknown,we should clear the cache and check in PD
					c.regionCache.InvalidateCache(key)
				}
				continue
			}
			// EpochNotMatch / RegionNotFound -> Invalidate Cache -> Retry
			if regionErr.EpochNotMatch != nil || regionErr.RegionNotFound != nil {
				c.regionCache.InvalidateCache(key)
				continue
			}
			return nil, fmt.Errorf("region error: %v", regionErr)
		}

		return resp, nil
	}
}
