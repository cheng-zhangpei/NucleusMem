package storage

import (
	tinykv_client "NucleusMem/pkg/storage/tinykv-client"
	"github.com/pingcap-incubator/tinykv/proto/pkg/tinykvpb"
	"google.golang.org/grpc"
)

type MemClient struct {
	Raw  DBClient
	Txn  TxnClient
	conn *grpc.ClientConn
}

func NewMemClient(addr string) (*MemClient, error) {
	conn, err := grpc.Dial(addr, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}

	pbClient := tinykvpb.NewTinyKvClient(conn)
	// call the factory
	rawClient := tinykv_client.NewTinyKVRawClient(pbClient)
	txnClient := tinykv_client.NewTinyKVTxnClient(pbClient)

	return &MemClient{
		Raw:  rawClient,
		Txn:  txnClient,
		conn: conn,
	}, nil
}

func (c *MemClient) Close() error {
	return c.conn.Close()
}
