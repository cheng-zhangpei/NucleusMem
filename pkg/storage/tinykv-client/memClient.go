package tinykv_client

import (
	"NucleusMem/pkg/storage"
	"context"
	"fmt"
	"strings"
	"time"
)

type MemClient struct {
	Txn    storage.TxnClient
	pdAddr string
}
type RetryableError struct {
	Msg string
}

func (e *RetryableError) Error() string { return e.Msg }
func NewMemClient(pdAddr string) (*MemClient, error) {
	// call the factory
	txnClient := NewTinyKVTxnClient(pdAddr)

	return &MemClient{
		Txn:    txnClient,
		pdAddr: pdAddr,
	}, nil
}

// Update the entry for user to execute a txn
func (c *MemClient) Update(fn func(txn storage.Transaction) error) error {
	// maxReTryTime
	maxRetries := 3
	for i := 0; i < maxRetries; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Second)
		defer cancel()
		// 1. start a new txn
		txn, err := c.Txn.Begin()
		if err != nil {
			return err
		}
		// 2.Execute the user-provided logic block
		if err := fn(txn); err != nil {
			return err
		}
		// 3. commit the txn
		err = txn.Commit(ctx)

		if err == nil {
			return nil
		}
		// 4. Check the error type to determine whether to retry
		// Here we need to determine if the error is a WriteConflict
		// For simplicity, we assume that any error string containing “conflict” warrants a retry
		if isRetryableError(err) {
			// Retreat Index
			time.Sleep(time.Duration(10*(i+1)) * time.Millisecond)
			continue
		}
		return err
	}
	return fmt.Errorf("transaction failed after %d retries", maxRetries)
}

// isRetryableError capture the retryable error
func isRetryableError(err error) bool {
	msg := err.Error()
	// Write Conflict
	if strings.Contains(msg, "write conflict") {
		return true
	}
	// Region Error
	if strings.Contains(msg, "region error") {
		return true
	}
	// Network jitter
	if strings.Contains(msg, "network jitter") {
		return true
	}
	// lock retry
	if strings.Contains(msg, "lock is solved") {
		return true
	}
	return false
}
