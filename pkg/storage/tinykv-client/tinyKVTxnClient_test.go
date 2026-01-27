package tinykv_client

import (
	"NucleusMem/pkg/storage"
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
)

const pdAddr = "127.0.0.1:2379"

// createClient is a helper function to quickly create a MemClient for testing.
// If the cluster is unavailable, the test will be skipped.
func createClient(t *testing.T) *MemClient {
	client, err := NewMemClient(pdAddr)
	if err != nil {
		t.Skipf("Skipping test: cluster not available (%v)", err)
	}
	return client
}

// TestClient_Basic verifies basic Put/Get/Delete operations.
func TestClient_Basic(t *testing.T) {
	client := createClient(t)

	key := []byte("test_basic_key")
	val := []byte("hello_world")

	// 1. Put
	err := client.Update(func(txn storage.Transaction) error {
		return txn.Put(key, val)
	})
	assert.NoError(t, err)

	// 2. Get
	var got []byte
	err = client.Update(func(txn storage.Transaction) error {
		got, err = txn.Get(key)
		return err
	})
	assert.NoError(t, err)
	assert.Equal(t, val, got)

	// 3. Delete
	err = client.Update(func(txn storage.Transaction) error {
		return txn.Delete(key)
	})
	assert.NoError(t, err)

	// 4. Get after delete should return nil
	err = client.Update(func(txn storage.Transaction) error {
		got, err = txn.Get(key)
		return err
	})
	assert.NoError(t, err)
	assert.Nil(t, got)
}

// TestClient_Atomicity verifies transaction atomicity: either all operations succeed or none do.
func TestClient_Atomicity(t *testing.T) {
	client := createClient(t)

	k1 := []byte("txn_k1")
	k2 := []byte("txn_k2")

	// Set initial state
	_ = client.Update(func(txn storage.Transaction) error {
		txn.Put(k1, []byte("v1"))
		txn.Put(k2, []byte("v2"))
		return nil
	})

	// Simulate a transaction that fails (should roll back)
	err := client.Update(func(txn storage.Transaction) error {
		txn.Put(k1, []byte("v1_new"))
		// Intentionally return an error to trigger rollback
		return fmt.Errorf("business error")
	})
	assert.Error(t, err)

	// Verify that data remains unchanged
	var v1, v2 []byte
	_ = client.Update(func(txn storage.Transaction) error {
		v1, _ = txn.Get(k1)
		v2, _ = txn.Get(k2)
		return nil
	})
	assert.Equal(t, []byte("v1"), v1)
	assert.Equal(t, []byte("v2"), v2)
}

// TestClient_Scan verifies range scanning by prefix.
// Note: This assumes the Transaction interface includes a Scan method.
func TestClient_Scan(t *testing.T) {
	client := createClient(t)

	prefix := []byte("scan_")
	// Write 5 keys with the same prefix
	err := client.Update(func(txn storage.Transaction) error {
		for i := 0; i < 5; i++ {
			key := []byte(fmt.Sprintf("scan_%d", i))
			txn.Put(key, key)
		}
		return nil
	})
	assert.NoError(t, err)

	// Perform scan
	var pairs []storage.KVPair
	err = client.Update(func(txn storage.Transaction) error {
		pairs, err = txn.Scan(prefix) // Requires Scan() in the Transaction interface
		return err
	})
	assert.NoError(t, err)
	assert.Len(t, pairs, 5)
}
