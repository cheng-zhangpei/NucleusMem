package tinykv_client

import (
	"NucleusMem/pkg/storage"
	"time"
)

// TinyKVTxnClient factory of the txn
type TinyKVTxnClient struct {
	pdAddr string
}

func NewTinyKVTxnClient(pdAddr string) *TinyKVTxnClient {
	return &TinyKVTxnClient{pdAddr: pdAddr}
}

func (c *TinyKVTxnClient) Close() error { return nil }

// Begin  generate TinyKVTxn
func (c *TinyKVTxnClient) Begin() (storage.Transaction, error) {
	startTS := uint64(time.Now().UnixNano())
	return NewTinyKVTxn(startTS, c.pdAddr), nil
}
