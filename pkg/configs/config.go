package configs

type AgentConfig struct {
	AgentManagerAddr    string
	MemSpaceManagerAddr string
	PrivateMemSpaceInfo *MemSpaceInfo
	PublicMemSpaceInfo  []*MemSpaceInfo
	ChatServerAddr      string
	VectorServerAddr    string
	GRPCServerAddr      string
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
