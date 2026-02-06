package monitor

import (
	"NucleusMem/pkg/configs"

	"google.golang.org/grpc"
)

type AgentMonitorInfo struct {
}
type AgentMonitor struct {
	grpcServer *grpc.Server
}

func NewAgentMonitor() *AgentMonitor {
	return &AgentMonitor{}
}

func (am *AgentMonitor) CreateAgent(config *configs.AgentConfig) error {
	return nil
}

func (am *AgentMonitor) DestroyAgent(config *configs.AgentConfig) error {
	return nil
}

func (am *AgentMonitor) GetAgentInfo() *AgentInfo {
	return nil
}

func (am *AgentMonitor) GetMonitorInfo() *AgentMonitorInfo {
	return nil
}

// Start start the grpc server of the agentMonitor
func (am *AgentMonitor) Start() {

}
