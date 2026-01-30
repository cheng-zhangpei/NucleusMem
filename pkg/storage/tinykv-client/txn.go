package tinykv_client

import (
	"NucleusMem/pkg/storage"
	"bytes"
	"context"
	"fmt"
	"github.com/pingcap-incubator/tinykv/proto/pkg/errorpb"
	"github.com/pingcap-incubator/tinykv/proto/pkg/kvrpcpb"
	"time"
)

// todo(Cheng) In cross-cluster scenarios, the Client serves as the decision-maker for transaction coordination.
// todo(Cheng) I will mock the the part of PD in client, the test env focus on single node

// TinyKVTxn the txn of the tinyKV
type TinyKVTxn struct {
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

func NewTinyKVTxn(startTS uint64, pdAddr string) *TinyKVTxn {
	// init the cluster router
	clusterClient, err := NewClusterClient(pdAddr)
	if err != nil {
		panic(err)
	}
	return &TinyKVTxn{
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
	regionInfo, addr, err := txn.clusterClient.regionCache.LocateRegion(context.Background(), key)
	if err != nil {
		return nil, err
	}
	req := &kvrpcpb.GetRequest{
		Key:     key,
		Version: txn.startTS,
	}
	// sendRequest
	respInterface, err := txn.clusterClient.SendRequest(context.Background(), addr, regionInfo, key, req)
	if err != nil {
		return nil, err
	}

	// handle get Response
	getResp := respInterface.(*kvrpcpb.GetResponse)
	if err := txn.handleGetResponse(context.Background(), getResp, nil); err != nil {
		return nil, err
	}

	return getResp.Value, nil
}

func (txn *TinyKVTxn) Scan(prefix []byte) ([]storage.KVPair, error) {
	ctx := context.Background()
	// 1. Construct the scan request
	req := &kvrpcpb.ScanRequest{
		StartKey: prefix,
		Limit:    10000,
		Version:  txn.startTS,
	}

	// 2. Locate the region containing the prefix (use prefix as the routing key)
	// todo(cheng) the method here only support debug mode, we need to write another location method to handle here
	// todo(cheng) I just use the first region to be the target

	txn.clusterClient.regionCache.mu.RLock()
	var regionInfo *RegionInfo
	var addr string
	if len(txn.clusterClient.regionCache.regions) > 0 {
		regionInfo = txn.clusterClient.regionCache.regions[0]
		storeID := regionInfo.Region.Peers[0].StoreId
		addr = txn.clusterClient.regionCache.storeAddrs[storeID]
	}
	txn.clusterClient.regionCache.mu.RUnlock()
	if regionInfo == nil {
		regionInfo, addr, _ = txn.clusterClient.regionCache.LocateRegion(ctx, prefix)
	}

	// 3. Send the request
	// SendRequest returns an interface{}, so we must type-assert it
	respInterface, err := txn.clusterClient.SendRequest(ctx, addr, regionInfo, prefix, req)
	if err != nil {
		return nil, err
	}

	resp := respInterface.(*kvrpcpb.ScanResponse)

	// 4. Process the response
	var pairs []storage.KVPair
	for _, p := range resp.Pairs {
		// If there's a key error (e.g., locked), handle it appropriately
		// For simplicity, skip this KV pair if an error exists (or you could return an error)
		if p.Error != nil {
			// Advanced: if p.Error.Locked != nil, you should attempt to resolve the lock.
			// For now, in a basic Scan implementation, we just skip the problematic entry.
			continue
		}

		// Stop scanning once we encounter a key that doesn't match the prefix
		// (since keys are sorted, all subsequent keys will also not match)
		if !bytes.HasPrefix(p.Key, prefix) {
			break
		}

		pairs = append(pairs, storage.KVPair{Key: p.Key, Value: p.Value})
	}

	return pairs, nil
}

// Commit execute the 2PC process
// remember, 2PC means actually 3 phase:
// 1、preWrite all the key
// 2、commit primary key —— means the transaction is committed
// 3、commit all secondary key —— this step allow error occur(async)
func (t *TinyKVTxn) Commit(ctx context.Context) error {
	// 1. Construct mutations from pending puts and deletes
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

	// 2. Group mutations by region
	groupMutations, err := t.GroupMutations(ctx, mutations)
	if err != nil {
		return err
	}

	// --- Phase 1: Prewrite (must succeed for all regions) ---
	for _, group := range groupMutations {
		if len(group.Mutations) == 0 {
			continue
		}

		// [Critical fix] Get the leader's store ID and address
		leaderStoreID := group.RegionInfo.Leader.StoreId
		addr, err := t.clusterClient.GetStoreAddr(ctx, leaderStoreID)
		if err != nil {
			t.Rollback(ctx, mutations)
			return err
		}

		preReq := &kvrpcpb.PrewriteRequest{
			Mutations:    group.Mutations,
			PrimaryLock:  t.primary,
			StartVersion: t.startTS,
			LockTtl:      3000,
		}

		respInterface, err := t.clusterClient.SendRequest(ctx, addr, group.RegionInfo, group.Mutations[0].Key, preReq)
		if err != nil {
			t.Rollback(ctx, mutations)
			return err
		}

		preResp := respInterface.(*kvrpcpb.PrewriteResponse)
		if err := t.handlePreWriteResponse(ctx, preResp, nil); err != nil {
			t.Rollback(ctx, mutations)
			return err
		}
	}

	// --- Phase 2: Commit Primary (point of no return) ---
	commitTS := t.getCommitTS()

	// Relocate the primary key's region (since groupMutations is unordered)
	primaryRegionInfo, _, err := t.clusterClient.regionCache.LocateRegion(ctx, t.primary)
	if err != nil {
		return err
	}

	// Get the store address where the primary key resides
	primaryAddr, err := t.clusterClient.GetStoreAddr(ctx, primaryRegionInfo.Leader.StoreId)
	if err != nil {
		return err
	}

	commitPrimaryReq := &kvrpcpb.CommitRequest{
		StartVersion:  t.startTS,
		CommitVersion: commitTS,
		Keys:          [][]byte{t.primary},
	}

	respInterface, err := t.clusterClient.SendRequest(ctx, primaryAddr, primaryRegionInfo, t.primary, commitPrimaryReq)
	if err != nil {
		return err
	}

	primaryResp := respInterface.(*kvrpcpb.CommitResponse)
	if err := t.handleCommitResponse(ctx, primaryResp, nil); err != nil {
		// Primary commit failed — the transaction is aborted
		return err
	}

	// --- Phase 3: Commit Secondaries (asynchronous, best-effort) ---
	go func() {
		for _, group := range groupMutations {
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

			// Get the store address for the secondary keys
			secAddr, err := t.clusterClient.GetStoreAddr(context.Background(), group.RegionInfo.Leader.StoreId)
			if err != nil {
				// Skip on error; cleanup will be handled later by resolve-lock mechanisms
				continue
			}

			respInterface, err := t.clusterClient.SendRequest(context.Background(), secAddr, group.RegionInfo, secondaryKeys[0], commitSecondariesReq)
			if err == nil {
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
		return &RetryableError{Msg: "network jitter"}
	}
	if resp.RegionError != nil {
		return t.handleRegionErr(resp.RegionError, nil)
	}

	if resp.Error != nil && resp.Error.Locked != nil {
		isSolved, resolveErr := t.resolveLocks(ctx, resp.Error.Locked.Key, resp.Error.Locked)
		if resolveErr != nil {
			return resolveErr
		}
		if isSolved {
			return &RetryableError{Msg: "lock is solved"}
		}
		return fmt.Errorf("key is locked: %v", resp.Error.Locked)
	}
	return nil
}

// handlePreWriteResponse check if PreWrite success
func (t *TinyKVTxn) handlePreWriteResponse(ctx context.Context, resp *kvrpcpb.PrewriteResponse, err error) error {
	if err != nil {
		return &RetryableError{Msg: "network jitter"}
	}
	if resp.RegionError != nil {
		return t.handleRegionErr(resp.RegionError, t.primary)
	}

	if len(resp.Errors) > 0 {
		for _, keyErr := range resp.Errors {
			if keyErr.Conflict != nil {
				return &RetryableError{Msg: "write conflict"}
			}
			if keyErr.Locked != nil {
				// 【修正】从 Locked 错误里拿到 Key
				isSolved, err := t.resolveLocks(ctx, keyErr.Locked.Key, keyErr.Locked)
				if err != nil {
					return err
				}
				if isSolved {
					return &RetryableError{Msg: "lock resolved"}
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
		return &RetryableError{Msg: "network jitter"}
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
			return &RetryableError{Msg: "write conflict"}
		}
		// B. lock conflict (Locked)
		if resp.Error.Locked != nil {
			// Detected that the Key is locked by another process.
			// Strategy: Backoff (wait a moment) -> ResolveLock (if timed out) -> Retry.
			// Here we simplify: directly attempt ResolveLock, then inform the upper layer to “please retry”.
			// Execute ResolveLock logic (check primary key status -> clean up lock)
			isSolved, err := t.resolveLocks(ctx, resp.Error.Locked.Key, resp.Error.Locked)
			if err != nil {
				return err
			}
			if isSolved {
				return nil // Commit 阶段如果锁解开了，通常不需要重试整个事务，或者重试 Commit
				// 但为了简单，这里返回 nil，让外层决定（或者你也返回 RetryableError）
			}
			return fmt.Errorf("key is locked: %v", resp.Error.Locked)
		}
		return fmt.Errorf("key error: %v", resp.Error)

	}
	return nil
}

func (t *TinyKVTxn) resolveLocks(ctx context.Context, key []byte, lock *kvrpcpb.LockInfo) (bool, error) {
	// 1. 查询 Primary Key 状态 (CheckTxnStatus)
	// 这个请求必须发给 Primary Key 所在的 Region
	checkReq := &kvrpcpb.CheckTxnStatusRequest{
		PrimaryKey: lock.PrimaryLock,
		LockTs:     lock.LockVersion,
		CurrentTs:  getCommitTS(),
	}

	primaryRegionInfo, primaryAddr, err := t.clusterClient.regionCache.LocateRegion(ctx, lock.PrimaryLock)
	if err != nil {
		return false, err
	}

	respInterface, err := t.clusterClient.SendRequest(ctx, primaryAddr, primaryRegionInfo, lock.PrimaryLock, checkReq)
	if err != nil {
		return false, err
	}
	checkResp := respInterface.(*kvrpcpb.CheckTxnStatusResponse)

	// 2. 根据状态决定如何 Resolve
	var resolveReq *kvrpcpb.ResolveLockRequest

	if checkResp.CommitVersion > 0 {
		// 已提交 -> 帮它提交
		resolveReq = &kvrpcpb.ResolveLockRequest{
			StartVersion:  lock.LockVersion,
			CommitVersion: checkResp.CommitVersion,
		}
	} else if checkResp.Action == kvrpcpb.Action_TTLExpireRollback ||
		checkResp.Action == kvrpcpb.Action_LockNotExistRollback {
		// 已回滚 -> 帮它回滚
		resolveReq = &kvrpcpb.ResolveLockRequest{
			StartVersion:  lock.LockVersion,
			CommitVersion: 0,
		}
	} else {
		// 锁还在且没过期 -> 等待
		return false, nil
	}

	// 3. 发送 ResolveLock 请求
	// 这个请求必须发给【当前被锁住的 Key】所在的 Region
	currentRegionInfo, currentAddr, err := t.clusterClient.regionCache.LocateRegion(ctx, key)
	if err != nil {
		return false, err
	}

	_, err = t.clusterClient.SendRequest(ctx, currentAddr, currentRegionInfo, key, resolveReq)
	if err != nil {
		return false, err
	}

	return true, nil
}

func (t *TinyKVTxn) handleRegionErr(re *errorpb.Error, key []byte) error {
	if re.NotLeader != nil {
		if re.NotLeader.Leader != nil {
			t.clusterClient.regionCache.UpdateLeader(re.NotLeader.RegionId, re.NotLeader.Leader)
		} else {
			t.clusterClient.regionCache.InvalidateCache(key)
		}
	}
	return &RetryableError{Msg: "region error"}
}

// GetStoreAddr 获取 Store 地址 (对外暴露)
func (c *ClusterClient) GetStoreAddr(ctx context.Context, storeID uint64) (string, error) {
	return c.regionCache.getStoreAddr(ctx, storeID)
}
