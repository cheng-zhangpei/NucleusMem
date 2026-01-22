package tinykv_client

import (
	"context"
	"time"

	"github.com/pingcap-incubator/tinykv/proto/pkg/kvrpcpb"
	"github.com/pingcap-incubator/tinykv/proto/pkg/tinykvpb"
)

// todo(Cheng) In cross-cluster scenarios, the Client serves as the decision-maker for transaction coordination.

// TinyKVTxn the txn of the tinyKV
type TinyKVTxn struct {
	client  tinykvpb.TinyKvClient
	startTS uint64
	puts    map[string][]byte
	deletes map[string]bool
	primary []byte
}

func NewTinyKVTxn(client tinykvpb.TinyKvClient, startTS uint64) *TinyKVTxn {
	return &TinyKVTxn{
		client:  client,
		startTS: startTS,
		puts:    make(map[string][]byte),
		deletes: make(map[string]bool),
	}
}

func (txn *TinyKVTxn) Put(key, val []byte) error {
	if txn.primary == nil {
		txn.primary = key
	}
	txn.puts[string(key)] = val
	delete(txn.deletes, string(key))
	return nil
}

func (txn *TinyKVTxn) Delete(key []byte) error {
	if txn.primary == nil {
		txn.primary = key
	}
	delete(txn.puts, string(key))
	txn.deletes[string(key)] = true
	return nil
}

func (txn *TinyKVTxn) Get(key []byte) ([]byte, error) {
	if val, ok := txn.puts[string(key)]; ok {
		return val, nil
	}
	if txn.deletes[string(key)] {
		return nil, nil
	}

	req := &kvrpcpb.GetRequest{Key: key, Version: txn.startTS}
	resp, err := txn.client.KvGet(context.Background(), req)
	if err != nil {
		return nil, err
	}
	return resp.Value, nil
}

func (t *TinyKVTxn) Commit(ctx context.Context) error {
	var mutations []*kvrpcpb.Mutation
	for k, v := range t.puts {
		mutations = append(mutations, &kvrpcpb.Mutation{
			Op:    kvrpcpb.Op_Put,
			Key:   []byte(k),
			Value: v,
		})
	}
	for k := range t.deletes {
		mutations = append(mutations, &kvrpcpb.Mutation{
			Op:  kvrpcpb.Op_Del,
			Key: []byte(k),
		})
	}

	if len(mutations) == 0 {
		return nil
	}

	// 2. Phase 1: Prewrite
	preReq := &kvrpcpb.PrewriteRequest{
		Mutations:    mutations,
		PrimaryLock:  t.primary,
		StartVersion: t.startTS,
		LockTtl:      3000,
	}

	preResp, err := t.client.KvPrewrite(ctx, preReq)
	if err != nil {
		return err
	}
	if len(preResp.Errors) > 0 {
		return nil
	}

	// 3. Phase 2: Commit
	// get CommitTS
	commitTS := uint64(time.Now().UnixNano())

	var keys [][]byte
	for _, m := range mutations {
		keys = append(keys, m.Key)
	}

	commitReq := &kvrpcpb.CommitRequest{
		StartVersion:  t.startTS,
		CommitVersion: commitTS,
		Keys:          keys,
	}

	commitResp, err := t.client.KvCommit(ctx, commitReq)
	if err != nil {
		return err
	}
	if commitResp.Error != nil {
		// Commit
		return nil
	}

	return nil
}

func (t *TinyKVTxn) Rollback() error {
	t.puts = make(map[string][]byte)
	t.deletes = make(map[string]bool)
	return nil
}
