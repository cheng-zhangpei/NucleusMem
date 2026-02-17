// pkg/memspace/memspace.go
package memspace

import (
	"NucleusMem/pkg/client"
	"NucleusMem/pkg/runtime/memspace/region"
	"NucleusMem/pkg/storage/tinykv-client"
	"context"
	"github.com/pingcap-incubator/tinykv/log"
	"github.com/pkg/errors"
	"strconv"
	"sync"
	"time"
)

type MemSpaceType string

const (
	MemSpaceTypePrivate MemSpaceType = "private"
	MemSpaceTypePublic  MemSpaceType = "public"
)

type MemSpace struct {
	ID          string
	Type        MemSpaceType
	Description string
	OwnerID     uint64 // only meaningful for private
	summaryCnt  uint64

	// New fields for background worker
	summaryThreshold  uint64 // trigger threshold (e.g., 10 memories)
	workerCtx         context.Context
	workerCancel      context.CancelFunc
	workerWG          sync.WaitGroup
	lastSummarizedSeq uint64
	chatServer        *client.ChatServerClient
	MemoryRegion      *memspace_region.MemoryRegion
	CommRegion        *memspace_region.CommRegion
	SummaryRegion     *memspace_region.SummaryRegion
}

// NewMemSpace creates a new MemSpace instance
func NewMemSpace(
	id string,
	memspaceType MemSpaceType,
	description string,
	ownerID uint64,
	summaryCnt uint64,
	summaryThreshold uint64, // ← 新增阈值参数
	pdAddr string,
	embeddingClientAddr string,
	lightModelAddr string,
) (*MemSpace, error) {
	if id == "" {
		return nil, errors.New("memspace ID required")
	}
	if memspaceType != MemSpaceTypePrivate && memspaceType != MemSpaceTypePublic {
		return nil, errors.New("invalid memspace type")
	}
	if summaryCnt == 0 {
		summaryCnt = 5 // default batch size
	}
	if summaryThreshold == 0 {
		summaryThreshold = 10 // default threshold
	}

	kvClient, err := tinykv_client.NewMemClient(pdAddr)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create TinyKV client")
	}
	idUint64, err := strconv.ParseUint(id, 10, 64)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse memspace ID")
	}
	serverClient := client.NewEmbeddingServerClient(embeddingClientAddr)
	chatServerClient := client.NewChatServerClient(lightModelAddr)
	ctx, cancel := context.WithCancel(context.Background())

	ms := &MemSpace{
		ID:               id,
		Type:             memspaceType,
		Description:      description,
		OwnerID:          ownerID,
		summaryCnt:       summaryCnt,
		summaryThreshold: summaryThreshold,
		workerCtx:        ctx,
		workerCancel:     cancel,
		chatServer:       chatServerClient,
		MemoryRegion:     memspace_region.NewMemoryRegion(kvClient, idUint64, serverClient),
		CommRegion:       memspace_region.NewCommRegion(kvClient),
		SummaryRegion:    memspace_region.NewSummaryRegion(kvClient, idUint64),
	}

	// Start background summary worker
	ms.startSummaryWorker()

	return ms, nil
}

func (m *MemSpace) WriteMemory(memory string, agentId uint64) error {
	if memory == "" {
		return errors.New("memory content cannot be empty")
	}
	return m.MemoryRegion.Write(agentId, memory)
}

// startSummaryWorker launches a background goroutine for auto-summarization
func (m *MemSpace) startSummaryWorker() {
	m.workerWG.Add(1)
	go func() {
		defer m.workerWG.Done()
		ticker := time.NewTicker(5 * time.Second) // check every 5s
		defer ticker.Stop()
		for {
			select {
			case <-m.workerCtx.Done():
				log.Infof("Summary worker stopped for MemSpace %s", m.ID)
				return
			case <-ticker.C:
				m.trySummarize()
			}
		}
	}()
}
func (m *MemSpace) trySummarize() {
	// Step 1: Check total count via a lightweight method (optional)
	// For now, just try to fetch next batch
	count, err := m.MemoryRegion.Count()
	if err != nil || count < m.summaryThreshold {
		log.Infof("the count:%d is not satisfied with the threshold:%d", count, m.summaryThreshold)
		return
	}
	// Now fetch a batch to summarize
	batch, err := m.MemoryRegion.GetBatch(m.lastSummarizedSeq+1, m.summaryCnt)
	if err != nil {
		log.Warnf("Failed to get memory batch: %v", err)
		return
	}
	// Step 2: Generate summary
	var contents []string
	var sourceIDs []string
	var maxSeq uint64
	for _, rec := range batch {
		contents = append(contents, rec.Content)
		sourceIDs = append(sourceIDs, rec.ID)
		seq := memspace_region.ParseSeqFromKey(rec.ID)
		if seq > maxSeq {
			maxSeq = seq
		}
	}
	summaryRec, err := m.SummaryRegion.GenerateSummary(contents, sourceIDs)
	if err != nil {
		log.Errorf("Summary generation failed: %v", err)
		return
	}
	if err := m.SummaryRegion.Add(summaryRec); err != nil {
		log.Errorf("Failed to save summary: %v", err)
		return
	}
	// Step 3: Update last processed sequence
	m.lastSummarizedSeq = maxSeq
	log.Infof("the summary record has been added to the summary region,lastIndex :%d", m.lastSummarizedSeq)
}
func (m *MemSpace) Stop() {
	m.workerCancel()
	m.workerWG.Wait()
}
