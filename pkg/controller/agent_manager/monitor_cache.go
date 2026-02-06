package agent_manager

import (
	"fmt"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
)

type monitorConns struct {
	mu    sync.RWMutex
	conns map[uint64]*grpc.ClientConn
}

func NewMonitorConns() *monitorConns {
	return &monitorConns{conns: make(map[uint64]*grpc.ClientConn)}
}

// AddConn 建立并缓存连接
func (mc *monitorConns) AddConn(nodeID uint64, addr string) error {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	if _, ok := mc.conns[nodeID]; ok {
		return nil // 已存在
	}

	kacp := keepalive.ClientParameters{
		Time:                10 * time.Second,
		Timeout:             time.Second,
		PermitWithoutStream: true,
	}

	conn, err := grpc.Dial(
		addr,
		grpc.WithKeepaliveParams(kacp),
	)
	if err != nil {
		return fmt.Errorf("failed to dial monitor %s: %v", addr, err)
	}

	mc.conns[nodeID] = conn
	return nil
}

// GetConn 获取连接以供调用
func (mc *monitorConns) GetConn(nodeID uint64) (*grpc.ClientConn, error) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	conn, ok := mc.conns[nodeID]
	if !ok {
		return nil, fmt.Errorf("connection not found for node: %s", nodeID)
	}
	return conn, nil
}

// RemoveConn 移除并关闭连接
func (mc *monitorConns) RemoveConn(nodeID uint64) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	if conn, ok := mc.conns[nodeID]; ok {
		conn.Close()
		delete(mc.conns, nodeID)
	}
}
