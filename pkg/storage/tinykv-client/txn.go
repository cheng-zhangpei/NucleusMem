package tinykv_client

import (
	"NucleusMem/pkg/storage"
	"bytes"
	"context"
	"fmt"
	"github.com/pingcap-incubator/tinykv/proto/pkg/errorpb"
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

type GroupedMutations struct {
	RegionInfo *RegionInfo
	Mutations  []*kvrpcpb.Mutation
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
	// todo (cheng) now the chose of the primary key is simple,how can the client decide the primary key better?
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
	// todo(cheng) need retry and refresh region cache
	_, addr, err := txn.clusterClient.regionCache.LocateRegion(context.Background(), key)
	if err != nil {
		return nil, err
	}
	client, err := txn.clusterClient.getConn(addr)
	if err != nil {
		return nil, err
	}

	req := &kvrpcpb.GetRequest{Key: key, Version: txn.startTS}

	resp, err := client.KvGet(context.Background(), req)

	if err != nil {
		return nil, err
	}
	return resp.Value, nil
}

// Commit execute the 2PC process
// remember, 2PC means actually 3 phase:
// 1、preWrite all the key
// 2、commit primary key —— means the transaction is committed
// 3、commit all secondary key —— this step allow error occur(async)

func (t *TinyKVTxn) Commit(ctx context.Context) error {
	// 1. Construct Mutations (all operations)
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

	// 2. Group by Region (Batch Grouping)
	groupMutations, err := t.GroupMutations(ctx, mutations)
	if err != nil {
		return err
	}

	// --- Phase 1: Prewrite (Must Succeed for All) ---
	for regionId, group := range groupMutations {
		if len(group.Mutations) == 0 {
			continue
		}

		preReq := &kvrpcpb.PrewriteRequest{
			Mutations:    group.Mutations,
			PrimaryLock:  t.primary,
			StartVersion: t.startTS,
			LockTtl:      3000,
		}

		addr := t.clusterClient.regionCache.storeAddrs[regionId]
		// Use the first key in the group as the routing key
		respInterface, err := t.clusterClient.SendRequest(ctx, addr, group.RegionInfo, group.Mutations[0].Key, preReq)
		if err != nil {
			// Network-level failure; roll back the entire transaction
			t.Rollback(ctx, mutations)
			return err
		}

		preResp := respInterface.(*kvrpcpb.PrewriteResponse)
		// Handle business-level errors (e.g., Locked/Conflict)
		if err := t.handlePreWriteResponse(ctx, preResp, nil); err != nil {
			// Prewrite failed; roll back the entire transaction
			t.Rollback(ctx, mutations)
			return err
		}
	}

	// --- Phase 2: Commit Primary (Point of No Return) ---
	commitTS := t.getCommitTS()

	// Locate the region containing the primary key
	// Note: We must re-locate the primary key since groupMutations is an unordered map
	primaryRegionInfo, addr, err := t.clusterClient.regionCache.LocateRegion(ctx, t.primary)
	if err != nil {
		return err // Failure here doesn't require rollback; let locks expire automatically
	}

	commitPrimaryReq := &kvrpcpb.CommitRequest{
		StartVersion:  t.startTS,
		CommitVersion: commitTS,
		Keys:          [][]byte{t.primary},
	}

	respInterface, err := t.clusterClient.SendRequest(ctx, addr, primaryRegionInfo, t.primary, commitPrimaryReq)
	// If network fails during primary commit, we cannot determine success/failure.
	// Simply return error and let client retry or check status later.
	if err != nil {
		return err
	}

	primaryResp := respInterface.(*kvrpcpb.CommitResponse)
	if err := t.handleCommitResponse(ctx, primaryResp, nil); err != nil {
		// Primary commit failed at business level (e.g., lock missing); transaction fails
		return err
	}

	// --- Phase 3: Commit Secondaries (Async Best-Effort) ---
	// Primary has succeeded, so the transaction is guaranteed to succeed.
	// Even if secondary commits fail, it's acceptable.
	go func() {
		for regionId, group := range groupMutations {
			// Extract secondary keys from this group (exclude primary)
			var secondaryKeys [][]byte
			for _, m := range group.Mutations {
				if !bytes.Equal(m.Key, t.primary) {
					secondaryKeys = append(secondaryKeys, m.Key)
				}
			}
			if len(secondaryKeys) == 0 {
				continue
			}

			commitSecondariesReq := &kvrpcpb.CommitRequest{
				StartVersion:  t.startTS,
				CommitVersion: commitTS,
				Keys:          secondaryKeys,
			}

			addr := t.clusterClient.regionCache.storeAddrs[regionId]
			// Send asynchronously; ignore errors
			respInterface, err := t.clusterClient.SendRequest(context.Background(), addr, group.RegionInfo, secondaryKeys[0], commitSecondariesReq)
			if err == nil {
				// Only used to handle possible RegionError and update cache; no need to handle Locked errors
				t.handleCommitResponse(context.Background(), respInterface.(*kvrpcpb.CommitResponse), nil)
			}
		}
	}()

	return nil
}

func (t *TinyKVTxn) Rollback(ctx context.Context, mutations []*kvrpcpb.Mutation) error {
	var keys [][]byte

	groupMutations, err := t.GroupMutations(ctx, mutations)
	if err != nil {
		return err
	}
	for regionId, RegionKeyPair := range groupMutations {
		for _, m := range RegionKeyPair.Mutations {
			keys = append(keys, m.Key)
		}
		req := &kvrpcpb.BatchRollbackRequest{
			StartVersion: t.startTS,
			Keys:         keys,
		}
		addr := t.clusterClient.regionCache.storeAddrs[regionId]
		RegionInfo := RegionKeyPair.RegionInfo
		// trigger roll back
		if len(RegionKeyPair.Mutations) == 0 {
			continue
		}
		_, err := t.clusterClient.SendRequest(ctx, addr, RegionInfo, RegionKeyPair.Mutations[0].Key, req)
		if err != nil {
			return err
		}
		t.puts = make(map[string][]byte)
		t.deletes = make(map[string]bool)

	}
	t.puts = make(map[string][]byte)
	t.deletes = make(map[string]bool)
	return err
}

// getCommitTS mock the TSO service to get committedTs
func (t *TinyKVTxn) getCommitTS() uint64 {
	// todo(Cheng) owing to the scale of the MVP,We do not support the TSO to get the time
	return uint64(time.Now().UnixNano())
}

// GroupMutations according to the  key group mutations
// return: map[RegionID] => {RegionInfo, []*Mutation}
func (t *TinyKVTxn) GroupMutations(ctx context.Context, mutations []*kvrpcpb.Mutation) (map[uint64]*GroupedMutations, error) {
	groups := make(map[uint64]*GroupedMutations)

	for _, m := range mutations {
		// 1. get the region of the key
		region, _, err := t.clusterClient.regionCache.LocateRegion(ctx, m.Key)
		if err != nil {
			return nil, err
		}

		id := region.Region.Id
		if _, ok := groups[id]; !ok {
			groups[id] = &GroupedMutations{
				RegionInfo: region,
				Mutations:  make([]*kvrpcpb.Mutation, 0),
			}
		}
		groups[id].Mutations = append(groups[id].Mutations, m)
	}
	return groups, nil
}

// ---------------------------handle response----------------------------
func (t *TinyKVTxn) handleGetResponse(ctx context.Context, resp *kvrpcpb.GetResponse, err error) error {
	if err != nil {
		return &storage.RetryableError{Msg: "network jitter"}
	}
	if resp.RegionError != nil {
		return t.handleRegionErr(resp.RegionError, nil)
	}

	if resp.Error != nil && resp.Error.Locked != nil {
		isSolved, resolveErr := t.resolveLocks(ctx, resp.Error.Locked)
		if resolveErr != nil {
			return resolveErr
		}
		if isSolved {
			return &storage.RetryableError{Msg: "lock is solved"}
		}
		return fmt.Errorf("key is locked: %v", resp.Error.Locked)
	}
	return nil
}

// handlePreWriteResponse check if PreWrite success
func (t *TinyKVTxn) handlePreWriteResponse(ctx context.Context, resp *kvrpcpb.PrewriteResponse, err error) error {
	if err != nil {
		return &storage.RetryableError{Msg: "network jitter"}
	}
	if resp.RegionError != nil {
		return t.handleRegionErr(resp.RegionError, t.primary)
	}

	if len(resp.Errors) > 0 {
		for _, keyErr := range resp.Errors {
			if keyErr.Conflict != nil {
				return fmt.Errorf("write conflict: %v", keyErr.Conflict)
			}
			if keyErr.Locked != nil {
				isSolved, resolveErr := t.resolveLocks(ctx, keyErr.Locked)
				if resolveErr != nil {
					return resolveErr
				}
				if isSolved {
					return &storage.RetryableError{Msg: "lock is solved"}
				}
				return fmt.Errorf("key is locked: %v", keyErr.Locked)
			}
			return fmt.Errorf("key error: %v", keyErr)
		}
	}
	return nil
}

// handleCommitResponse check if the Commit success
func (t *TinyKVTxn) handleCommitResponse(ctx context.Context, resp *kvrpcpb.CommitResponse, err error) error {
	//  network error,if retry? Network jitter
	if err != nil {
		return &storage.RetryableError{Msg: "network jitter"}
	}
	if resp.RegionError != nil {
		// When encountering Region errors (e.g., NotLeader, RegionNotFound, EpochNotMatch)
		// The client should refresh the Region Cache and resend the request.
		// This returns true (retry required)
		// Since we haven't implemented Region Cache at this layer yet, we can only propagate the error upward for now.
		re := resp.RegionError
		key := t.primary
		return t.handleRegionErr(re, key)

	}
	if resp.Error != nil {
		// if retryable, here we should resend the txn logic
		if resp.Error.Conflict != nil {
			// Found a commit record later than my StartTS.
			// This conflict cannot be resolved by retrying (because my StartTS is already outdated).
			// Must abort and restart with a new StartTS.
			return &storage.RetryableError{Msg: "write conflict"}
		}
		// B. lock conflict (Locked)
		if resp.Error.Locked != nil {
			// Detected that the Key is locked by another process.
			// Strategy: Backoff (wait a moment) -> ResolveLock (if timed out) -> Retry.
			// Here we simplify: directly attempt ResolveLock, then inform the upper layer to “please retry”.
			// Execute ResolveLock logic (check primary key status -> clean up lock)
			isSolved, _ := t.resolveLocks(ctx, resp.Error.Locked)
			if isSolved {
				// the lock is solved, sending the txn again
				return &storage.RetryableError{Msg: "lock is solved"}

			} else {
				// the lock is still stuck there,what we can do is just wait
				return fmt.Errorf("key is locked: %v", resp.Error.Locked)
			}
		}
		return fmt.Errorf("key error: %v", resp.Error)

	}
	return nil
}

func (t *TinyKVTxn) resolveLocks(ctx context.Context, lock *kvrpcpb.LockInfo) (bool, error) {
	checkReq := &kvrpcpb.CheckTxnStatusRequest{
		PrimaryKey: lock.PrimaryLock,
		LockTs:     lock.LockVersion,
		CurrentTs:  getCommitTS(),
	}
	// todo(cheng) check the key range and confirm
	regionInfo, addr, err := t.clusterClient.regionCache.LocateRegion(ctx, lock.PrimaryLock)
	if err != nil {
		return false, err
	}
	// 2. get conn by addr
	client, err := t.clusterClient.getConn(addr)
	if err != nil {
		return false, err
	}
	resp, err := t.clusterClient.SendRequest(ctx, addr, regionInfo, lock.PrimaryLock, checkReq)
	if err != nil {
		return false, err
	}
	checkResp := resp.(*kvrpcpb.CheckTxnStatusResponse)
	// the transaction have been committed
	if checkResp.CommitVersion > 0 {
		// help to commit the transaction
		_, keyErr := client.KvResolveLock(context.TODO(), &kvrpcpb.ResolveLockRequest{
			StartVersion:  lock.LockVersion,
			CommitVersion: checkResp.CommitVersion,
		})
		// todo(cheng) error handle
		if keyErr != nil {
			return false, err
		}
		return true, nil
		// if the lock is expired or not exist
	} else if checkResp.Action == kvrpcpb.Action_TTLExpireRollback ||
		checkResp.Action == kvrpcpb.Action_LockNotExistRollback {
		_, keyErr := client.KvResolveLock(context.TODO(), &kvrpcpb.ResolveLockRequest{
			StartVersion:  lock.LockVersion,
			CommitVersion: 0,
		})
		// todo(cheng) error handle
		if keyErr != nil {
			return false, err
		}
		return true, nil
	}
	return false, nil
}
func (t *TinyKVTxn) handleRegionErr(re *errorpb.Error, key []byte) error {
	if re.NotLeader != nil {
		if re.NotLeader.Leader != nil {
			t.clusterClient.regionCache.UpdateLeader(re.NotLeader.RegionId, re.NotLeader.Leader)
		} else {
			t.clusterClient.regionCache.InvalidateCache(key)
		}
	}
	return &storage.RetryableError{Msg: "region error"}
}
