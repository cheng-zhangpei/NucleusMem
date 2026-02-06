package agent

import (
	"NucleusMem/pkg/configs"
	"fmt"
	"time"

	"github.com/pingcap-incubator/tinykv/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
)

// connectToMemSpace helper function: establish a single gRPC connection (Legacy Version)
func (a *Agent) connectToMemSpace(info *configs.MemSpaceInfo) error {
	a.memMu.Lock()
	defer a.memMu.Unlock()

	// Deduplication: prevent duplicate connections
	if _, exists := a.memConnPool[info.MemSpaceId]; exists {
		return nil
	}
	kacp := keepalive.ClientParameters{
		Time:                10 * time.Second, // Interval to send pings
		Timeout:             time.Second,      // Timeout waiting for pong
		PermitWithoutStream: true,             // Allow pings even without active streams
	}
	conn, err := grpc.Dial(
		info.MemSpaceAddr,
		grpc.WithKeepaliveParams(kacp),
		//grpc.WithBlock(), // we always can not ensure that every memSpace is accessible
	)

	if err != nil {
		return fmt.Errorf("failed to connect to memspace_manager %s: %v", info.MemSpaceAddr, err)
	}

	// Store connection in pool
	a.memConnPool[info.MemSpaceId] = conn

	log.Infof("Connected to MemSpace [ID: %d] at %s", info.MemSpaceId, info.MemSpaceAddr)

	return nil
}

// Close gracefully shuts down resources
func (a *Agent) Close() {
	a.memMu.Lock()
	defer a.memMu.Unlock()
	for id, conn := range a.memConnPool {
		err := conn.Close()
		if err != nil {
			log.Infof("Error closing connection to memspace_manager %d: %v", id, err)
		}
	}
	// Clear the map
	a.memConnPool = make(map[uint64]*grpc.ClientConn)
}
func (a *Agent) CloseById(id uint64) {
	a.memMu.Lock()
	defer a.memMu.Unlock()
	delete(a.memConnPool, id)
	return
}
