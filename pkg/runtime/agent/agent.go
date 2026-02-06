package agent

import (
	"NucleusMem/pkg/configs"
	"NucleusMem/pkg/runtime/agent/client"
	"fmt"
	"net"
	"sync"

	"github.com/pingcap-incubator/tinykv/log"
	"google.golang.org/grpc"
)

type Agent struct {
	agentManagerAddr    string
	memSpaceManagerAddr string
	chatClient          *client.ChatServerClient
	embeddingClient     *client.EmbeddingServerClient
	memMu               sync.RWMutex
	// memSpaceID -> gRPC Client Connection
	memConnPool map[uint64]*grpc.ClientConn
	grpcServer  *grpc.Server
}

func NewAgent(config *configs.AgentConfig) (*Agent, error) {
	// Initialize the memory space connection pool
	agent := &Agent{
		agentManagerAddr:    config.AgentManagerAddr,
		memSpaceManagerAddr: config.MemSpaceManagerAddr,
		chatClient:          client.NewChatServerClient(config.ChatServerAddr),
		embeddingClient:     client.NewEmbeddingServerClient(config.VectorServerAddr),
		memConnPool:         make(map[uint64]*grpc.ClientConn),
	}
	// connect the privateMem(must succeed)
	if config.PrivateMemSpaceInfo != nil {
		err := agent.connectToMemSpace(config.PrivateMemSpaceInfo)
		if err != nil {
			return nil, fmt.Errorf("failed to connect to private memspace_manager: %v", err)
		}
	}
	for _, info := range config.PublicMemSpaceInfo {

		err := agent.connectToMemSpace(info)
		if err != nil {
			// clean the log
			agent.CloseById(info.MemSpaceId)
			log.Errorf("failed to connect to public memspace_manager %d: %v", info.MemSpaceId, err)
		}
	}
	err := agent.Start(config.GRPCServerAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to start agent_manager: %v", err)
	}
	return agent, nil
}

func (a *Agent) Start(addr string) error {
	// 1. listen the port
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %v", addr, err)
	}
	// 2.  gRPC Server instance
	// You can add some ServerOptions, such as interceptors, for authentication or logging.	a.grpcServer = grpc.NewServer()
	// 3.  Service Registration
	log.Infof("Agent maintenance server listening on %s", addr)
	// 4. Start the server within a coroutine to prevent blocking.
	go func() {
		if err := a.grpcServer.Serve(lis); err != nil {
			log.Fatalf("failed to serve maintenance server: %v", err)
		}
	}()
	// go a.startWatcherLoop()
	return nil
}

func (a *Agent) Stop() {
	if a.grpcServer != nil {
		a.grpcServer.GracefulStop()
	}
	a.Close()
}
