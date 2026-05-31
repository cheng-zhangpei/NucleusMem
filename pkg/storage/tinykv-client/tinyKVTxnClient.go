package tinykv_client

import (
	"NucleusMem/pkg/storage"
	"context"
	"fmt"
	"strings"
	"time"
)

type TinyKVTxnClient struct {
	pdAddr string
}

func NewTinyKVTxnClient(pdAddr string) *TinyKVTxnClient {
	return &TinyKVTxnClient{pdAddr: pdAddr}
}

func (c *TinyKVTxnClient) Close() error { return nil }

func (c *TinyKVTxnClient) Begin() (storage.Transaction, error) {
	startTS := uint64(time.Now().UnixNano())
	return NewTinyKVTxn(startTS, c.pdAddr), nil
}

func (c *TinyKVTxnClient) Update(fn func(txn storage.Transaction) error) error {
	maxRetries := 3
	for i := 0; i < maxRetries; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Second)

		txn, err := c.Begin()
		if err != nil {
			cancel()
			return err
		}

		if err := fn(txn); err != nil {
			cancel()
			return err
		}

		err = txn.Commit(ctx)
		cancel()

		if err == nil {
			return nil
		}

		if isRetryableError(err) {
			time.Sleep(time.Duration(10*(i+1)) * time.Millisecond)
			continue
		}
		return err
	}
	return fmt.Errorf("transaction failed after %d retries", maxRetries)
}

func isRetryableError(err error) bool {
	msg := err.Error()
	for _, keyword := range []string{
		"write conflict",
		"region error",
		"network jitter",
		"lock is solved",
	} {
		if strings.Contains(msg, keyword) {
			return true
		}
	}
	return false
}
