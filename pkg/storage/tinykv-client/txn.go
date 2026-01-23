package tinykv_client

import (
	"bytes"
	"context"
	"fmt"
	"github.com/pingcap-incubator/tinykv/proto/pkg/kvrpcpb"
	"github.com/pingcap-incubator/tinykv/proto/pkg/tinykvpb"
	"time"
)

// todo(Cheng) In cross-cluster scenarios, the Client serves as the decision-maker for transaction coordination.
// todo(Cheng) I will mock the the part of PD in client, the test env focus on single node

// TinyKVTxn the txn of the tinyKV
type TinyKVTxn struct {
	client        tinykvpb.TinyKvClient
	startTS       uint64
	puts          map[string][]byte
	deletes       map[string]bool
	primary       []byte
	clusterClient *ClusterClient
}

func NewTinyKVTxn(client tinykvpb.TinyKvClient, startTS uint64, pdAddr string) *TinyKVTxn {
	// init the cluster router
	clusterClient, err := NewClusterClient(pdAddr)
	if err != nil {
		panic(err)
	}
	return &TinyKVTxn{
		client:        client,
		startTS:       startTS,
		puts:          make(map[string][]byte),
		deletes:       make(map[string]bool),
		clusterClient: clusterClient,
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
	// construct mutations
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

	// --- Phase 1: Prewrite (send all keys) ---
	preReq := &kvrpcpb.PrewriteRequest{
		Mutations:    mutations,
		PrimaryLock:  t.primary,
		StartVersion: t.startTS,
		LockTtl:      3000,
	}

	preResp, err := t.client.KvPrewrite(ctx, preReq)
	// process Prewrite response
	if err := t.handlePreWriteResponse(preResp, err); err != nil {
		err := t.Rollback(ctx, mutations)
		return err
	}

	// --- Phase 2: Commit ---
	commitTS := t.getCommitTS()
	// 2.1 Commit Primary
	// only commit Primary Key
	commitPrimaryReq := &kvrpcpb.CommitRequest{
		StartVersion:  t.startTS,
		CommitVersion: commitTS,
		Keys:          [][]byte{t.primary},
	}

	primaryResp, err := t.client.KvCommit(ctx, commitPrimaryReq)
	// process Primary Commit response
	isRetry, err := t.handleCommitResponse(primaryResp, err)
	if err != nil {
		return err
	}
	if isRetry {
		t.retryCommit(commitPrimaryReq)
	}
	// 2.2 Commit Secondaries (Async / Sync)
	// now the transaction is committed successfully
	var secondaryKeys [][]byte
	for _, m := range mutations {
		if !bytes.Equal(m.Key, t.primary) {
			secondaryKeys = append(secondaryKeys, m.Key)
		}
	}

	if len(secondaryKeys) > 0 {
		commitSecondariesReq := &kvrpcpb.CommitRequest{
			StartVersion:  t.startTS,
			CommitVersion: commitTS,
			Keys:          secondaryKeys,
		}
		// send Secondary Commit (async send secondary key)
		go func() {
			secResp, _ := t.client.KvCommit(context.Background(), commitSecondariesReq)
			isRetry, err := t.handleCommitResponse(secResp, nil)
			if err != nil {
				return
			}
			if isRetry {
				t.retryCommit(commitSecondariesReq)
			}
		}()
	}

	return nil
}

// handlePreWriteResponse check if Prewrite success
func (t *TinyKVTxn) handlePreWriteResponse(resp *kvrpcpb.PrewriteResponse, err error) error {
	if err != nil {
		return err
	}

	if len(resp.Errors) > 0 {
		return fmt.Errorf("prewrite failed: %v", resp.Errors[0])
	}
	return nil
}

// handleCommitResponse check if the Commit success
func (t *TinyKVTxn) handleCommitResponse(resp *kvrpcpb.CommitResponse, err error) (bool, error) {
	//  network error,if retry?
	if err != nil {
		// todo(Cheng): network error,if the client should resend the request?
		return false, err
	}
	if resp.RegionError != nil {
		// When encountering Region errors (e.g., NotLeader, RegionNotFound, EpochNotMatch)
		// The client should refresh the Region Cache and resend the request.
		// This returns true (retry required)
		// Since we haven't implemented Region Cache at this layer yet, we can only propagate the error upward for now.
		return true, fmt.Errorf("region error: %v", resp.RegionError)
	}
	if resp.Error != nil {
		// if retryable, here we should resend the txn logic
		if resp.Error.Conflict != nil {
			// Found a commit record later than my StartTS.
			// This conflict cannot be resolved by retrying (because my StartTS is already outdated).
			// Must abort and restart with a new StartTS.
			return false, fmt.Errorf("write conflict: %v", resp.Error.Conflict)
		}
		// B. lock conflict (Locked)
		if resp.Error.Locked != nil {
			// Detected that the Key is locked by another process.
			// Strategy: Backoff (wait a moment) -> ResolveLock (if timed out) -> Retry.
			// Here we simplify: directly attempt ResolveLock, then inform the upper layer to “please retry”.
			// Execute ResolveLock logic (check primary key status -> clean up lock)
			isSolved := t.resolveLocks(resp.Error.Locked)
			if isSolved {
				// the lock is solved, sending the txn again
				return true, nil
			} else {
				// the lock is still stuck there,what we can do is just wait
				return false, fmt.Errorf("key is locked: %v", resp.Error.Locked)
			}
		}
		return false, fmt.Errorf("key error: %v", resp.Error)

	}
	return false, nil
}

func (t *TinyKVTxn) Rollback(ctx context.Context, mutations []*kvrpcpb.Mutation) error {
	var keys [][]byte
	for _, m := range mutations {
		keys = append(keys, m.Key)
	}

	req := &kvrpcpb.BatchRollbackRequest{
		StartVersion: t.startTS,
		Keys:         keys,
	}
	// trigger roll back
	_, err := t.client.KvBatchRollback(ctx, req)

	t.puts = make(map[string][]byte)
	t.deletes = make(map[string]bool)

	return err
}
func (t *TinyKVTxn) retryPreWrite(req *kvrpcpb.PrewriteRequest) error {
	return nil
}
func (t *TinyKVTxn) retryCommit(req *kvrpcpb.CommitRequest) error {
	return nil
}

// getCommitTS mock the TSO service to get committedTs
func (t *TinyKVTxn) getCommitTS() uint64 {
	// todo(Cheng) owing to the scale of the MVP,We do not support the TSO to get the time
	return uint64(time.Now().UnixNano())
}

func (t *TinyKVTxn) resolveLocks(lock *kvrpcpb.LockInfo) bool {
	checkReq := &kvrpcpb.CheckTxnStatusRequest{
		PrimaryKey: lock.PrimaryLock,
		LockTs:     lock.LockVersion,
		CurrentTs:  t.startTS,
	}
	// todo(cheng) check the key range and comfirm
	checkResp, err := t.client.KvCheckTxnStatus(context.TODO(), checkReq)
	if err != nil {
		return false
	}
	// the transaction have been committed
	if checkResp.CommitVersion > 0 {
		// help to commit the transaction
		// todo(cheng) error handle
		t.client.KvResolveLock(context.TODO(), &kvrpcpb.ResolveLockRequest{
			StartVersion:  lock.LockVersion,
			CommitVersion: checkResp.CommitVersion,
		})
		return true
		// if the lock is
	} else if checkResp.Action == kvrpcpb.Action_TTLExpireRollback ||
		checkResp.Action == kvrpcpb.Action_LockNotExistRollback {
		// todo(cheng) error handle
		t.client.KvResolveLock(context.TODO(), &kvrpcpb.ResolveLockRequest{
			StartVersion:  lock.LockVersion,
			CommitVersion: 0,
		})
		return true
	}
	return false
}
