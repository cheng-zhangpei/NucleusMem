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
