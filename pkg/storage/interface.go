package storage

import (
	"context"

	"github.com/pingcap-incubator/tinykv/proto/pkg/kvrpcpb"
)

// TxnClient the factory of the txn
type TxnClient interface {
	Begin() (Transaction, error)
	Update(func(txn Transaction) error) error //
	Close() error
}

// Transaction interface for a txn: it is a txn for distributed txn
type Transaction interface {
	Get(key []byte) ([]byte, error)
	Put(key, val []byte) error
	Delete(key []byte) error
	Commit(ctx context.Context) error
	Rollback(ctx context.Context, mutations []*kvrpcpb.Mutation) error
	Scan(prefix []byte) ([]KVPair, error)
}

type KVPair struct {
	Key   []byte
	Value []byte
}

// VectorStore 向量检索能力
type VectorStore interface {
	// Insert 写入一条带向量的记忆
	Insert(ctx context.Context, entry VectorEntry) error
	// Search 相似度搜索，返回 top-k
	Search(ctx context.Context, query []float32, topK int, filter MetadataFilter) ([]SearchResult, error)
	// Delete 删除指定记忆
	Delete(ctx context.Context, id string) error
}

// GraphStore 图查询能力
type GraphStore interface {
	// AddEdge 建立关系: subject --predicate--> object
	AddEdge(ctx context.Context, edge Edge) error
	// GetNeighbors 查询某实体的关系
	GetNeighbors(ctx context.Context, entity string, direction EdgeDirection) ([]Edge, error)
	// FindPath 两实体之间的路径
	FindPath(ctx context.Context, from, to string, maxDepth int) ([][]Edge, error)
	// RemoveEdge 删除关系
	RemoveEdge(ctx context.Context, from, predicate, to string) error
}

// ==========================================
// Layer 2: 数据模型（三种记忆的结构体）
// ==========================================

type VectorEntry struct {
	ID        string
	Embedding []float32
	Content   string            // 原始文本
	Metadata  map[string]string // 时间戳、来源、类型等
}

type SearchResult struct {
	Entry VectorEntry
	Score float32 // 相似度
}

type MetadataFilter map[string]string

type EdgeDirection int

const (
	EdgeOutgoing EdgeDirection = iota
	EdgeIncoming
	EdgeBoth
)

type Edge struct {
	Subject   string
	Predicate string
	Object    string
	Weight    float64
	Metadata  map[string]string
}

// ==========================================
// Layer 3: 组合——Agent 实际使用的接口
// ==========================================

// MemoryStore 组合接口，Agent 只依赖这一个
type MemoryStore interface {
	// 基础设施
	Close() error

	// 三种能力，nil 表示该后端未配置
	KV() TxnClient
	Vector() VectorStore
	Graph() GraphStore
}
