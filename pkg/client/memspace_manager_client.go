package client

type MemSpaceManagerClient struct {
	Addr string
}

func NewMemSpaceManagerClient(addr string) *MemSpaceManagerClient {
	return &MemSpaceManagerClient{addr}
}
