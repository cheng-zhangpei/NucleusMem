package bbolt_client

import (
	"NucleusMem/pkg/storage"
	"bytes"
	"context"
	"github.com/pingcap-incubator/tinykv/proto/pkg/kvrpcpb"
	bolt "go.etcd.io/bbolt"
	"time"
)

var defaultBucket = []byte("kv")

// ---------- Client ----------

type Client struct {
	db *bolt.DB
}

// 编译期接口检查
var _ storage.TxnClient = (*Client)(nil)

func NewClient(path string) (*Client, error) {
	db, err := bolt.Open(path, 0600, &bolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		return nil, err
	}
	// 确保 bucket 存在
	if err := db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(defaultBucket)
		return err
	}); err != nil {
		db.Close()
		return nil, err
	}
	return &Client{db: db}, nil
}

func (c *Client) Begin() (storage.Transaction, error) {
	tx, err := c.db.Begin(true) // writable tx
	if err != nil {
		return nil, err
	}
	return &Txn{tx: tx}, nil
}

func (c *Client) Close() error {
	return c.db.Close()
}

// ---------- Transaction ----------

type Txn struct {
	tx *bolt.Tx
}

var _ storage.Transaction = (*Txn)(nil)

func (t *Txn) bkt() *bolt.Bucket {
	return t.tx.Bucket(defaultBucket)
}

func (t *Txn) Get(key []byte) ([]byte, error) {
	v := t.bkt().Get(key)
	if v == nil {
		return nil, nil
	}
	// bbolt 的 slice 在 tx 结束后失效，必须拷贝
	cp := make([]byte, len(v))
	copy(cp, v)
	return cp, nil
}

func (t *Txn) Put(key, val []byte) error {
	return t.bkt().Put(key, val)
}

func (t *Txn) Delete(key []byte) error {
	return t.bkt().Delete(key)
}

func (t *Txn) Commit(_ context.Context) error {
	return t.tx.Commit()
}

// mutations 参数在单机场景下无意义，直接忽略
func (t *Txn) Rollback(_ context.Context, _ []*kvrpcpb.Mutation) error {
	return t.tx.Rollback()
}

func (t *Txn) Scan(prefix []byte) ([]storage.KVPair, error) {
	var result []storage.KVPair
	c := t.bkt().Cursor()

	for k, v := c.Seek(prefix); k != nil; k, v = c.Next() {
		if !bytes.HasPrefix(k, prefix) {
			break
		}
		kcp := make([]byte, len(k))
		copy(kcp, k)
		vcp := make([]byte, len(v))
		copy(vcp, v)
		result = append(result, storage.KVPair{Key: kcp, Value: vcp})
	}
	return result, nil
}
func (c *Client) Update(fn func(txn storage.Transaction) error) error {
	return c.db.Update(func(tx *bolt.Tx) error {
		return fn(&Txn{tx: tx})
	})
}
