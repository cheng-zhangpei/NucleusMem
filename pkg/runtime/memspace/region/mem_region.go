// pkg/runtime/memspace/region/memory_region.go
//todoList:
/*
1。Remove records cache (if you don't use precise Get)
2. Add pagination/filtering for Search (e.g., Search(agentID, query, n))
3. Replace GetAll() with ANN indexing in the future
//todo Persistence or scanning—it's a trade-off.
*/

package memspace_region

import (
	"NucleusMem/pkg/client"
	"NucleusMem/pkg/configs"
	"NucleusMem/pkg/storage"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"github.com/pingcap-incubator/tinykv/log"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const SeqKey = "memory_seq"

type MemoryRegion struct {
	MemSpaceID      uint64
	mu              sync.RWMutex
	records         map[string]*configs.MemoryRecord
	KvClient        storage.TxnClient
	embeddingClient *client.EmbeddingServerClient
	neq             uint64 //Serial Number Maintenance Memory Record Key Increment
}

func NewMemoryRegion(kvClient storage.TxnClient,
	memSpaceID uint64,
	embeddingClient *client.EmbeddingServerClient) *MemoryRegion {
	mr := &MemoryRegion{
		records:         make(map[string]*configs.MemoryRecord), // memory cache here
		KvClient:        kvClient,
		MemSpaceID:      memSpaceID,
		embeddingClient: embeddingClient,
	}
	seq, _ := mr.loadNextSeq()
	mr.neq = seq
	return mr
}
func (mr *MemoryRegion) Write(agentId uint64, content string) error {
	mr.mu.Lock()
	seq := mr.neq
	mr.neq++
	err := mr.saveNextSeq(mr.neq) // persist immediately
	if err != nil {
		log.Warn("save next seq failed", zap.Error(err))
	}
	mr.mu.Unlock()
	key := mr.GenerateKey(agentId, seq)
	// 这个位置我们需要去区分现在的agent是否有开embedding
	var embedding []float32 = nil
	if mr.embeddingClient != nil {
		embedding, err = mr.embeddingClient.EmbedSingle(content, 0)
		if err != nil {
			return fmt.Errorf("embedding failed: %w", err)
		}
	}

	record := &configs.MemoryRecord{
		ID:        key,
		Content:   content,
		Embedding: embedding,
		Timestamp: time.Now().Unix(),
	}
	return mr.Add(record)
}

// Add writes a memory record to both cache and TinyKV (transactionally)
func (mr *MemoryRegion) Add(record *configs.MemoryRecord) error {
	if record.Timestamp == 0 {
		record.Timestamp = time.Now().Unix()
	}
	data, err := json.Marshal(record)
	if err != nil {
		return err
	}

	rawKey := configs.EncodeKey(configs.ZoneMemory, mr.MemSpaceID, []byte(record.ID))
	return mr.KvClient.Update(func(txn storage.Transaction) error {
		return txn.Put(rawKey, data)
	})
}

// GetAll scans all keys in Memory Zone for this MemSpace
func (mr *MemoryRegion) GetAll() ([]*configs.MemoryRecord, error) {
	prefix := configs.GetScanPrefix(configs.ZoneMemory, mr.MemSpaceID)
	var records []*configs.MemoryRecord

	err := mr.KvClient.Update(func(txn storage.Transaction) error {
		kvPairs, err := txn.Scan(prefix)
		if err != nil {
			return err
		}
		for _, pair := range kvPairs {
			var record configs.MemoryRecord
			if err := json.Unmarshal(pair.Value, &record); err != nil {
				continue // skip corrupted entries
			}
			records = append(records, &record)
		}
		return nil
	})

	if err != nil {
		return nil, err
	}
	// Update cache
	mr.mu.Lock()
	defer mr.mu.Unlock()
	for _, record := range records {
		mr.records[record.ID] = record
	}
	return records, nil
}
func (mr *MemoryRegion) GetAllWithKeys() ([]struct {
	Key    string
	Record *configs.MemoryRecord
}, error) {
	prefix := configs.GetScanPrefix(configs.ZoneMemory, mr.MemSpaceID)
	var results []struct {
		Key    string
		Record *configs.MemoryRecord
	}

	err := mr.KvClient.Update(func(txn storage.Transaction) error {
		kvPairs, err := txn.Scan(prefix)
		if err != nil {
			return err
		}
		for _, pair := range kvPairs {
			var record configs.MemoryRecord
			if err := json.Unmarshal(pair.Value, &record); err != nil {
				continue // skip corrupted
			}
			results = append(results, struct {
				Key    string
				Record *configs.MemoryRecord
			}{
				Key:    string(pair.Key),
				Record: &record,
			})
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	// Update cache by ID
	mr.mu.Lock()
	defer mr.mu.Unlock()
	for _, item := range results {
		mr.records[item.Record.ID] = item.Record
	}
	return results, nil
}
func (mr *MemoryRegion) ScanByAgent(agentID uint64) ([]*configs.MemoryRecord, error) {
	prefix := []byte(fmt.Sprintf("memory/%d/", agentID))
	rawPrefix := configs.EncodeKey(configs.ZoneMemory, mr.MemSpaceID, prefix)
	var records []*configs.MemoryRecord
	err := mr.KvClient.Update(func(txn storage.Transaction) error {
		kvPairs, err := txn.Scan(rawPrefix)
		if err != nil {
			return err
		}
		for _, pair := range kvPairs {
			var record configs.MemoryRecord
			if err := json.Unmarshal(pair.Value, &record); err != nil {
				continue
			}
			records = append(records, &record)
		}
		return nil
	})
	return records, err
}

// DeleteBatchByKeys deletes entries by their actual KV keys
func (mr *MemoryRegion) DeleteBatchByKeys(keys []string) error {
	return mr.KvClient.Update(func(txn storage.Transaction) error {
		for _, k := range keys {
			if err := txn.Delete([]byte(k)); err != nil {
				log.Warnf("Failed to delete key %s: %v", k, err)
			}
		}
		return nil
	})
}

// Search returns top-n most similar memory contents based on semantic similarity
func (mr *MemoryRegion) Search(query string, n int) ([]string, error) {
	if mr.embeddingClient == nil {
		log.Warnf("No embedding server, may be the search function did not start!")
		return nil, errors.New("embedding client not initialized or started in the config")
	}

	if query == "" {
		return nil, fmt.Errorf("query cannot be empty")
	}
	if n <= 0 {
		return nil, fmt.Errorf("n must be positive")
	}

	// Step 1: Get embedding for query
	queryEmbed, err := mr.embeddingClient.EmbedSingle(query, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to embed query: %w", err)
	}

	// Step 2: Load all records from storage
	// todo(cheng):This full-page loading is clearly undesirable, so batch loading or more reliable caching mechanisms are crucial
	records, err := mr.GetAll()
	if err != nil {
		return nil, fmt.Errorf("failed to load memories: %w", err)
	}

	if len(records) == 0 {
		return []string{}, nil
	}
	// Step 3: Compute similarity scores
	type scoredRecord struct {
		content string
		score   float32
	}
	scores := make([]scoredRecord, 0, len(records))
	for _, rec := range records {
		if len(rec.Embedding) == 0 {
			continue // skip unembedded records
		}
		score := cosineSimilarity(queryEmbed, rec.Embedding)
		scores = append(scores, scoredRecord{
			content: rec.Content,
			score:   score,
		})
	}
	// Step 4: Sort by score (descending)
	sort.Slice(scores, func(i, j int) bool {
		return scores[i].score > scores[j].score
	})
	// Step 5: Extract top-n contents
	result := make([]string, 0, n)
	for i := 0; i < n && i < len(scores); i++ {
		result = append(result, scores[i].content)
	}
	return result, nil
}

// dotProduct computes the dot product of two vectors (assumes same length)
func dotProduct(a, b []float32) float32 {
	if len(a) != len(b) {
		return 0
	}
	var sum float32
	for i := range a {
		sum += a[i] * b[i]
	}
	return sum
}

// GenerateKey generates a user-friendly key (not the storage key!)
func (mr *MemoryRegion) GenerateKey(agentID, neq uint64) string {
	return fmt.Sprintf("memory/%d/%d", agentID, neq)
}
func cosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}

	var dot, normA, normB float32
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (float32(math.Sqrt(float64(normA))) * float32(math.Sqrt(float64(normB))))
}

// loadNextSeq loads the next sequence number from storage
func (mr *MemoryRegion) loadNextSeq() (uint64, error) {
	rawKey := configs.EncodeKey(configs.ZoneMemory, mr.MemSpaceID, []byte(SeqKey))
	var seq uint64 = 1 // default if not exists
	err := mr.KvClient.Update(func(txn storage.Transaction) error {
		data, err := txn.Get(rawKey)
		if err != nil {
			// Key not found is OK — use default 1
			return nil
		}
		if len(data) == 0 {
			return nil
		}

		if len(data) < 8 {
			return fmt.Errorf("corrupted sequence key: got %d bytes, want 8", len(data))
		}
		seq = binary.LittleEndian.Uint64(data)
		return nil
	})
	return seq, err
}

// saveNextSeq saves the next sequence number to storage
func (mr *MemoryRegion) saveNextSeq(seq uint64) error {
	rawKey := configs.EncodeKey(configs.ZoneMemory, mr.MemSpaceID, []byte(SeqKey))
	data := make([]byte, 8)
	binary.LittleEndian.PutUint64(data, seq)

	return mr.KvClient.Update(func(txn storage.Transaction) error {
		return txn.Put(rawKey, data)
	})
}

// GetBatch retrieves memories with sequence numbers in [startSeq, startSeq + count)
func (mr *MemoryRegion) GetBatch(startSeq, count uint64) ([]*configs.MemoryRecord, error) {
	if count == 0 {
		return nil, nil
	}

	var records []*configs.MemoryRecord
	endSeq := startSeq + count

	// We'll scan and filter by parsing the key: "memory/{agent}/{seq}"
	// Since keys are lexicographically ordered, we can't use pure prefix,
	// but we can scan and break early when seq >= endSeq.

	prefix := configs.GetScanPrefix(configs.ZoneMemory, mr.MemSpaceID)
	err := mr.KvClient.Update(func(txn storage.Transaction) error {
		kvPairs, err := txn.Scan(prefix)
		if err != nil {
			return err
		}

		for _, pair := range kvPairs {
			// Decode user key from raw key
			_, _, userKey, err := configs.DecodeKey(pair.Key)
			if err != nil {
				continue
			}

			keyStr := string(userKey)
			if !strings.HasPrefix(keyStr, "memory/") {
				continue
			}

			parts := strings.Split(keyStr, "/")
			if len(parts) != 3 {
				continue
			}

			seq, err := strconv.ParseUint(parts[2], 10, 64)
			if err != nil {
				continue
			}

			// Only collect records in [startSeq, endSeq)
			if seq >= startSeq && seq < endSeq {
				var record configs.MemoryRecord
				if err := json.Unmarshal(pair.Value, &record); err == nil {
					records = append(records, &record)
				}
			}

			// Optional: break early if keys are sorted by seq
			// (not guaranteed unless you control agent ID ordering)
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	// Sort by sequence number to ensure order
	sort.Slice(records, func(i, j int) bool {
		seqI := configs.ParseMemSeqFromKey(records[i].ID)
		seqJ := configs.ParseMemSeqFromKey(records[j].ID)
		return seqI < seqJ
	})

	return records, nil
}

func (mr *MemoryRegion) Count() (uint64, error) {
	// Scan prefix and count keys (no value loading)
	prefix := configs.GetScanPrefix(configs.ZoneMemory, mr.MemSpaceID)
	var count uint64
	err := mr.KvClient.Update(func(txn storage.Transaction) error {
		kvPairs, err := txn.Scan(prefix)
		if err != nil {
			return err
		}
		count = uint64(len(kvPairs))
		return nil
	})
	return count, err
}
func (mr *MemoryRegion) DeleteBatch(ids []string) error {
	return mr.KvClient.Update(func(txn storage.Transaction) error {
		for _, id := range ids {
			rawKey := []byte(id) // 假设 ID 就是完整 key
			if err := txn.Delete(rawKey); err != nil {
				log.Warnf("Failed to delete key %s: %v", id, err)
				// 继续删除其他
			}
		}
		return nil
	})
}
