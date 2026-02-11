package client

type MemSpaceClient struct {
	Addr string
}

func NewMemSpaceClient(addr string) *MemSpaceClient {
	return &MemSpaceClient{addr}
}
