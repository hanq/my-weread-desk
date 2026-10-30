package rag

import (
	"sort"
	"sync"
)

// ChunkStore loads persisted chunks for one book.
type ChunkStore interface {
	RAGChunks(bookID string) ([]Chunk, error)
}

// Retriever keeps one BM25 index per book in memory.
//
// A book's index is immutable once built, so the cache only needs invalidation
// when the book is re-indexed. Building costs a few milliseconds for a normal
// book; the cache exists to avoid paying it on every question, not to bound
// memory.
type Retriever struct {
	store ChunkStore

	mu    sync.RWMutex
	cache map[string]*Index
}

// NewRetriever wires a retriever to a chunk store.
func NewRetriever(store ChunkStore) *Retriever {
	return &Retriever{store: store, cache: map[string]*Index{}}
}

// Index returns the book's index, building and caching it on first use.
func (r *Retriever) Index(bookID string) (*Index, error) {
	r.mu.RLock()
	idx, ok := r.cache[bookID]
	r.mu.RUnlock()
	if ok {
		return idx, nil
	}

	chunks, err := r.store.RAGChunks(bookID)
	if err != nil {
		return nil, err
	}
	built := Build(chunks)

	r.mu.Lock()
	r.cache[bookID] = built
	r.mu.Unlock()
	return built, nil
}

// Invalidate drops a cached index after a re-index or a cache clear.
func (r *Retriever) Invalidate(bookID string) {
	r.mu.Lock()
	delete(r.cache, bookID)
	r.mu.Unlock()
}

// Search queries one or more books and merges the rankings.
//
// Scores from different books use the same BM25 parameters, so they are
// comparable directly. The tie-break is the book id, which keeps a multi-book
// result deterministic instead of depending on map iteration.
func (r *Retriever) Search(bookIDs []string, query string, limit int) ([]Hit, error) {
	var all []Hit
	seen := map[string]bool{}
	for _, id := range bookIDs {
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		idx, err := r.Index(id)
		if err != nil {
			return nil, err
		}
		all = append(all, idx.Search(query, limit)...)
	}
	sort.SliceStable(all, func(a, b int) bool {
		if all[a].Score != all[b].Score {
			return all[a].Score > all[b].Score
		}
		return all[a].Chunk.BookID < all[b].Chunk.BookID
	})
	if limit > 0 && len(all) > limit {
		all = all[:limit]
	}
	return all, nil
}
