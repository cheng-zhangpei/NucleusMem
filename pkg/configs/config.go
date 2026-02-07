package configs

type AgentConfig struct {
	AgentId             uint64
	AgentManagerAddr    string
	MemSpaceManagerAddr string
	PrivateMemSpaceInfo *MemSpaceInfo
	PublicMemSpaceInfo  []*MemSpaceInfo
	ChatServerAddr      string
	VectorServerAddr    string
	GRPCServerAddr      string
	Role                string
	Image               string
	Path                string
}

type MemSpaceInfo struct {
	MemSpaceId   uint64
	MemSpaceAddr string
}
type AgentManagerConfig struct {
	// the
	//
	//agent info provided by the user
	monitors []*MonitorConfig
	// grpcServerAddr

}
type MonitorConfig struct {
	GrpcServerAddrs map[uint64]string
}
