package client

type AgentManagerClient struct {
	addr string
}

func NewAgentManagerClient(addr string) *AgentManagerClient {
	return &AgentManagerClient{addr}
}
