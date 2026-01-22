package tinykv_client

import (
	"bytes"
	"context"
	"time"

	"NucleusMem/pkg/storage"
	"github.com/pingcap-incubator/tinykv/proto/pkg/kvrpcpb"
	"github.com/pingcap-incubator/tinykv/proto/pkg/tinykvpb"
)

type TinyKVRawClient struct {
	client tinykvpb.TinyKvClient
}

func NewTinyKVRawClient(client tinykvpb.TinyKvClient) *TinyKVRawClient {
	return &TinyKVRawClient{client: client}
}

func (c *TinyKVRawClient) Close() error { return nil }

func (c *TinyKVRawClient) getStartTS() uint64 { return uint64(time.Now().UnixNano()) }

// Put start a new txn and commit
func (c *TinyKVRawClient) Put(key, val []byte) error {
	txn := NewTinyKVTxn(c.client, c.getStartTS())
	if err := txn.Put(key, val); err != nil {
		return err
	}
	return txn.Commit(context.Background())
}

// Get start a txn to get
func (c *TinyKVRawClient) Get(key []byte) ([]byte, error) {
	txn := NewTinyKVTxn(c.client, c.getStartTS())
	return txn.Get(key)
}

// Delete delete
func (c *TinyKVRawClient) Delete(key []byte) error {
	txn := NewTinyKVTxn(c.client, c.getStartTS())
	if err := txn.Delete(key); err != nil {
		return err
	}
	return txn.Commit(context.Background())
}

// PrefixList get list  by prefix
func (c *TinyKVRawClient) PrefixList(prefix []byte) ([][]byte, error) {
	pairs, err := c.Scan(prefix)
	if err != nil {
		return nil, err
	}
	var res [][]byte
	for _, p := range pairs {
		res = append(res, p.Key) // 假设 KVPair 有 Key 字段
	}
	return res, nil
}

// Scan: 对应 Txn 的 Scan (需要 Txn 实现 Scan)
// 如果 Txn 没实现 Scan，这里也没法实现。
// 假设你给 Txn 加了 Scan：
func (tc *TinyKVRawClient) Scan(prefix []byte) ([]storage.KVPair, error) {
	// 1. 获取 StartTS (模拟 Begin)
	startTS := tc.getStartTS()

	// 2. 构造 ScanRequest
	req := &kvrpcpb.ScanRequest{
		StartKey: prefix,
		Limit:    1000,
		Version:  startTS, // 这里用生成好的 startTS
	}

	// 3. 发送 RPC
	resp, err := tc.client.KvScan(context.Background(), req)
	if err != nil {
		return nil, err
	}

	// 4. 处理结果
	var pairs []storage.KVPair
	for _, p := range resp.Pairs {
		// 如果有 Key Error (锁等)，这里应该处理或者忽略
		if p.Error != nil {
			continue
		}
		// 简单的 Prefix 过滤
		if !bytes.HasPrefix(p.Key, prefix) {
			break
		}
		pairs = append(pairs, storage.KVPair{Key: p.Key, Value: p.Value})
	}

	return pairs, nil
}
