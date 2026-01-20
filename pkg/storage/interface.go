package storage

import "context"

// DBClient: 原生 KV 操作接口 (无状态)
type DBClient interface {
	Get(key []byte) ([]byte, error)
	Put(key, val []byte) error
	Delete(key []byte) error
	Scan(prefix []byte) ([]KVPair, error)
	PrefixList(prefix []byte) ([][]byte, error)
	Close() error
}

type KVPair struct {
	Key   []byte
	Value []byte
}

// TxnClient: 事务工厂接口
type TxnClient interface {
	Begin() (Transaction, error)
	Close() error
}

// Transaction: 具体的事务会话接口
type Transaction interface {
	Get(key []byte) ([]byte, error)
	Put(key, val []byte) error
	Delete(key []byte) error
	Commit(ctx context.Context) error
	Rollback() error
}
