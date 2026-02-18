// pkg/runtime/memspace/region/summary_region.go
package memspace_region

import (
	"NucleusMem/pkg/configs"
	"NucleusMem/pkg/storage"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	tinykv_client "NucleusMem/pkg/storage/tinykv-client"
)

const SummarySeqKey = "summary_seq"

type SummaryRecord struct {
	ID        string   `json:"id"`
	Content   string   `json:"content"`    // compressed summary text
	SourceIDs []string `json:"source_ids"` // original memory IDs
	Timestamp int64    `json:"timestamp"`
	// Metadata removed per your preference
}

type SummaryRegion struct {
	memSpaceID uint64
	mu         sync.RWMutex
	summaries  map[string]*SummaryRecord
	kvClient   *tinykv_client.MemClient
	seq        uint64 // sequence number for generating unique keys
}

func NewSummaryRegion(kvClient *tinykv_client.MemClient, memSpaceID uint64) *SummaryRegion {
	sr := &SummaryRegion{
		memSpaceID: memSpaceID,
		summaries:  make(map[string]*SummaryRecord),
		kvClient:   kvClient,
	}

	// Load sequence number from dedicated key (O(1))
	if seq, err := sr.loadSummarySeq(); err == nil {
		sr.seq = seq
	} else {
		sr.seq = 1 // fallback
	}
	return sr
}

// Add persists a summary record to storage and cache
func (sr *SummaryRegion) Add(summary *SummaryRecord) error {
	if summary.Timestamp == 0 {
		summary.Timestamp = time.Now().Unix()
	}

	data, err := json.Marshal(summary)
	if err != nil {
		return err
	}

	rawKey := configs.EncodeKey(configs.ZoneSummary, sr.memSpaceID, []byte(summary.ID))
	return sr.kvClient.Update(func(txn storage.Transaction) error {
		return txn.Put(rawKey, data)
	})
}

// Get retrieves a summary by ID (from cache or storage)
func (sr *SummaryRegion) Get(id string) (*SummaryRecord, bool) {
	// Try cache
	sr.mu.RLock()
	if s, ok := sr.summaries[id]; ok {
		sr.mu.RUnlock()
		return s, true
	}
	sr.mu.RUnlock()

	// Load from storage
	rawKey := configs.EncodeKey(configs.ZoneSummary, sr.memSpaceID, []byte(id))
	var record SummaryRecord
	err := sr.kvClient.Update(func(txn storage.Transaction) error {
		data, err := txn.Get(rawKey)
		if err != nil {
			return err
		}
		return json.Unmarshal(data, &record)
	})
	if err != nil {
		return nil, false
	}

	// Cache it
	sr.mu.Lock()
	sr.summaries[id] = &record
	sr.mu.Unlock()
	return &record, true
}

// GetAll loads all summaries for this MemSpace
func (sr *SummaryRegion) GetAll() ([]*SummaryRecord, error) {
	prefix := configs.GetScanPrefix(configs.ZoneSummary, sr.memSpaceID)
	var records []*SummaryRecord
	err := sr.kvClient.Update(func(txn storage.Transaction) error {
		kvPairs, err := txn.Scan(prefix)
		if err != nil {
			return err
		}
		for _, pair := range kvPairs {
			var record SummaryRecord
			if err := json.Unmarshal(pair.Value, &record); err != nil {
				continue
			}
			records = append(records, &record)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Update cache
	sr.mu.Lock()
	for _, rec := range records {
		sr.summaries[rec.ID] = rec
	}
	sr.mu.Unlock()

	return records, nil
}

// GetBefore returns all summaries with timestamp < given timestamp
func (sr *SummaryRegion) GetBefore(timestamp int64) ([]*SummaryRecord, error) {
	all, err := sr.GetAll()
	if err != nil {
		return nil, err
	}

	var result []*SummaryRecord
	for _, s := range all {
		if s.Timestamp < timestamp {
			result = append(result, s)
		}
	}
	return result, nil
}

// GenerateSummary creates a compressed summary from a list of memory contents
// For now, return a placeholder.
func (sr *SummaryRegion) GenerateSummary(contents []string, sourceIDs []string) (*SummaryRecord, error) {
	if len(contents) == 0 {
		return nil, fmt.Errorf("no content to summarize")
	}
	// Placeholder summary
	const maxLen = 500
	// todo(cheng): Integrate small LLM for actual summarization
	summaryText := ""
	for _, c := range contents {
		if len(summaryText)+len(c) > maxLen {
			break
		}
		summaryText += c + " "
	}
	// Atomically get and increment sequence number
	sr.mu.Lock()
	sr.seq++
	key := fmt.Sprintf("summary/%d", sr.seq)
	err := sr.saveSummarySeq(sr.seq) // persist immediately
	sr.mu.Unlock()

	if err != nil {
		return nil, fmt.Errorf("failed to save summary sequence: %w", err)
	}

	return &SummaryRecord{
		ID:        key,
		Content:   summaryText,
		SourceIDs: sourceIDs,
		Timestamp: time.Now().Unix(),
	}, nil
}

// loadSummarySeq loads the next sequence number for summaries
func (sr *SummaryRegion) loadSummarySeq() (uint64, error) {
	rawKey := configs.EncodeKey(configs.ZoneSummary, sr.memSpaceID, []byte(SummarySeqKey))
	var seq uint64 = 1

	err := sr.kvClient.Update(func(txn storage.Transaction) error {
		data, err := txn.Get(rawKey)
		if err != nil {
			return nil // key not found → use default 1
		}
		seq = binary.LittleEndian.Uint64(data)
		return nil
	})
	return seq, err
}

// saveSummarySeq saves the current sequence number
func (sr *SummaryRegion) saveSummarySeq(seq uint64) error {
	rawKey := configs.EncodeKey(configs.ZoneSummary, sr.memSpaceID, []byte(SummarySeqKey))
	data := make([]byte, 8)
	binary.LittleEndian.PutUint64(data, seq)
	return sr.kvClient.Update(func(txn storage.Transaction) error {
		return txn.Put(rawKey, data)
	})
}
