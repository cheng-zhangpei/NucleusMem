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
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	tinykv_client "NucleusMem/pkg/storage/tinykv-client"
)

const SeqKey = "memory_seq"

type MemoryRecord struct {
	ID        string            `json:"id"`
	Content   string            `json:"content"`
	Embedding []float32         `json:"embedding,omitempty"`
	Timestamp int64             `json:"timestamp"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

type MemoryRegion struct {
	MemSpaceID      uint64
	mu              sync.RWMutex
	records         map[string]*MemoryRecord
	KvClient        *tinykv_client.MemClient
	embeddingClient *client.EmbeddingServerClient
	neq             uint64 //Serial Number Maintenance Memory Record Key Increment
}

func NewMemoryRegion(kvClient *tinykv_client.MemClient,
	memSpaceID uint64,
	embeddingClient *client.EmbeddingServerClient) *MemoryRegion {
	mr := &MemoryRegion{
		records:         make(map[string]*MemoryRecord), // memory cache here
		KvClient:        kvClient,
		MemSpaceID:      memSpaceID,
		embeddingClient: embeddingClient,
	}
	seq, _ := mr.loadNextSeq()
	mr.neq = seq
	return mr
}
func (mr *MemoryRegion) Write(agentId uint64, content string) error {
	if content == "" {
		return fmt.Errorf("content cannot be empty")
	}

	mr.mu.Lock()
	seq := mr.neq
	mr.neq++
	err := mr.saveNextSeq(mr.neq) // persist immediately

	mr.mu.Unlock()

	key := mr.GenerateKey(agentId, seq)

	embedding, err := mr.embeddingClient.EmbedSingle(content, 0)
	if err != nil {
		return fmt.Errorf("embedding failed: %w", err)
	}

	record := &MemoryRecord{
		ID:        key,
		Content:   content,
		Embedding: embedding,
		Timestamp: time.Now().Unix(),
	}
	return mr.Add(record)
}

// Add writes a memory record to both cache and TinyKV (transactionally)
func (mr *MemoryRegion) Add(record *MemoryRecord) error {
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
func (mr *MemoryRegion) GetAll() ([]*MemoryRecord, error) {
	prefix := configs.GetScanPrefix(configs.ZoneMemory, mr.MemSpaceID)
	var records []*MemoryRecord

	err := mr.KvClient.Update(func(txn storage.Transaction) error {
		kvPairs, err := txn.Scan(prefix)
		if err != nil {
			return err
		}
		for _, pair := range kvPairs {
			var record MemoryRecord
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

func (mr *MemoryRegion) ScanByAgent(agentID uint64) ([]*MemoryRecord, error) {
	prefix := []byte(fmt.Sprintf("memory/%d/", agentID))
	rawPrefix := configs.EncodeKey(configs.ZoneMemory, mr.MemSpaceID, prefix)
	var records []*MemoryRecord
	err := mr.KvClient.Update(func(txn storage.Transaction) error {
		kvPairs, err := txn.Scan(rawPrefix)
		if err != nil {
			return err
		}
		for _, pair := range kvPairs {
			var record MemoryRecord
			if err := json.Unmarshal(pair.Value, &record); err != nil {
				continue
			}
			records = append(records, &record)
		}
		return nil
	})
	return records, err
}

// Search returns top-n most similar memory contents based on semantic similarity
func (mr *MemoryRegion) Search(query string, n int) ([]string, error) {
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
func (mr *MemoryRegion) GetBatch(startSeq, count uint64) ([]*MemoryRecord, error) {
	if count == 0 {
		return nil, nil
	}

	var records []*MemoryRecord
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
				var record MemoryRecord
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
