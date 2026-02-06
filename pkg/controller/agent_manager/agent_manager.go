package agent_manager

import (
	"NucleusMem/pkg/configs"
	"fmt"
	"net"

	"github.com/pingcap-incubator/tinykv/log"
	"google.golang.org/grpc"
)

type AgentManager struct {
	serverAddr  string
	monitorPool *monitorConns // maintain the message and conn of monitors in each node
	agentCache  *AgentCache   // maintain the information of every agent which enable the manager to connect every agent
	grpcServer  *grpc.Server
	// todo (cheng proto define) monitorClient\agentClient

}

func NewAgentManager(grpcServerAddr string, monitorConfig *configs.MonitorConfig) (*AgentManager, error) {
	// 1, build monitor conn
	monitorConns := NewMonitorConns()
	for id, addr := range monitorConfig.GrpcServerAddrs {
		err := monitorConns.AddConn(id, addr)
		if err != nil {
			log.Errorf("the monitor %d is not ready!,err:%v \n", id, err)
		}
	}
	agentCache := NewAgentCache()
	// todo(cheng) 2、 using monitor client to update the agentCache
	return &AgentManager{grpcServerAddr, monitorConns, agentCache, nil}, nil
}

// Start launch the grpc service
func (am *AgentManager) Start() error {
	lis, err := net.Listen("tcp", am.serverAddr)
	if err != nil {
		return fmt.Errorf("failed to listen: %v", err)
	}
	am.grpcServer = grpc.NewServer()
	// todo(cheng) after the proto file finish
	// pb.RegisterAgentManagerServer(am.grpcServer, newManagerService(am))
	log.Infof("AgentManager listening on %s\n", am.serverAddr)
	return am.grpcServer.Serve(lis)
}

// CreateAgent create agent_manager instance by Agent config(local env)
func (*AgentManager) CreateAgent(config configs.AgentConfig) error {
	return nil
}

// DeleteAgent delete agent_manager service in local System
func (*AgentManager) DeleteAgent(id uint64) error {
	return nil
}

// BingMemSpace binding the memspace and agent
func (*AgentManager) BingMemSpace() error {
	return nil
}

// GenerateAgentService create agents resource in a cloud-native environment (distributed env)
func (*AgentManager) GenerateAgentService(config configs.AgentConfig) error {
	return nil
}

func (*AgentManager) destroyAgentService(config configs.AgentConfig) error {
	return nil
}

// LoadAgent load the agentCache for each monitors
func (*AgentManager) LoadAgent() {

}
