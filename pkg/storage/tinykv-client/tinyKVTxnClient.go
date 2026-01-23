package tinykv_client

import (
	"NucleusMem/pkg/storage"
	"github.com/pingcap-incubator/tinykv/proto/pkg/tinykvpb"
	"time"
)

// TinyKVTxnClient factory of the txn
type TinyKVTxnClient struct {
	pdAddr string
	client tinykvpb.TinyKvClient
}

func NewTinyKVTxnClient(client tinykvpb.TinyKvClient, pdAddr string) *TinyKVTxnClient {
	return &TinyKVTxnClient{client: client, pdAddr: pdAddr}
}

func (c *TinyKVTxnClient) Close() error { return nil }

// Begin  generate TinyKVTxn
func (c *TinyKVTxnClient) Begin() (storage.Transaction, error) {
	startTS := uint64(time.Now().UnixNano())
	return NewTinyKVTxn(c.client, startTS, c.pdAddr), nil
}
