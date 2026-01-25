package storage

import (
	tinykv_client "NucleusMem/pkg/storage/tinykv-client"
	"context"
	"fmt"
	"github.com/pingcap-incubator/tinykv/proto/pkg/tinykvpb"
	"google.golang.org/grpc"
	"strings"
	"time"
)

type MemClient struct {
	Raw    DBClient
	Txn    TxnClient
	conn   *grpc.ClientConn
	pdAddr string
}
type RetryableError struct {
	Msg string
}

func (e *RetryableError) Error() string { return e.Msg }
func NewMemClient(addr string, pdAddr string) (*MemClient, error) {
	conn, err := grpc.Dial(addr, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}

	pbClient := tinykvpb.NewTinyKvClient(conn)
	// call the factory
	rawClient := tinykv_client.NewTinyKVRawClient(pbClient, pdAddr)
	txnClient := tinykv_client.NewTinyKVTxnClient(pbClient, pdAddr)

	return &MemClient{
		Raw:    rawClient,
		Txn:    txnClient,
		conn:   conn,
		pdAddr: pdAddr,
	}, nil
}

func (c *MemClient) Close() error {
	return c.conn.Close()
}

// Update the entry for user to execute a txn
func (c *MemClient) Update(fn func(txn Transaction) error) error {
	// maxReTryTime
	maxRetries := 3
	for i := 0; i < maxRetries; i++ {
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
		err = txn.Commit(context.Background())

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
