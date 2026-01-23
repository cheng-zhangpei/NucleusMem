package storage

import (
	"context"
	"github.com/pingcap-incubator/tinykv/proto/pkg/kvrpcpb"
)

type DBClient interface {
	Get(key []byte) ([]byte, error)
	Put(key, val []byte) error
	Delete(key []byte) error
	Scan(prefix []byte) ([]KVPair, error)
	PrefixList(prefix []byte) ([][]byte, error)
	Close() error
}

// TxnClient the factory of the txn
type TxnClient interface {
	Begin() (Transaction, error)
	Close() error
}

// Transaction interface for a txn
type Transaction interface {
	Get(key []byte) ([]byte, error)
	Put(key, val []byte) error
	Delete(key []byte) error
	Commit(ctx context.Context) error
	Rollback(ctx context.Context, mutations []*kvrpcpb.Mutation) error
}

type KVPair struct {
	Key   []byte
	Value []byte
}
