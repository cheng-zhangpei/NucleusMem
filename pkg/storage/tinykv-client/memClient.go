package tinykv_client

import (
	"NucleusMem/pkg/storage"
)

type RetryableError struct {
	Msg string
}

func (e *RetryableError) Error() string { return e.Msg }

type MemClient struct {
	Txn    storage.TxnClient
	pdAddr string
}

func NewMemClient(pdAddr string) (*MemClient, error) {
	txnClient := NewTinyKVTxnClient(pdAddr)
	return &MemClient{
		Txn:    txnClient,
		pdAddr: pdAddr,
	}, nil
}

func (c *MemClient) Update(fn func(txn storage.Transaction) error) error {
	return c.Txn.Update(fn)
}
