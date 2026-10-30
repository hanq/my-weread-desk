package rag

import "testing"

type fakeStore struct {
	chunks map[string][]Chunk
	calls  int
}

func (f *fakeStore) RAGChunks(bookID string) ([]Chunk, error) {
	f.calls++
	return f.chunks[bookID], nil
}

func TestRetrieverBuildsOncePerBook(t *testing.T) {
	store := &fakeStore{chunks: map[string][]Chunk{
		"a": {{BookID: "a", Seq: 0, Text: "林黛玉初进荣国府"}},
	}}
	r := NewRetriever(store)

	if _, err := r.Index("a"); err != nil {
		t.Fatalf("Index: %v", err)
	}
	if _, err := r.Index("a"); err != nil {
		t.Fatalf("Index: %v", err)
	}
	if store.calls != 1 {
		t.Fatalf("index should be cached, store calls = %d", store.calls)
	}

	r.Invalidate("a")
	if _, err := r.Index("a"); err != nil {
		t.Fatalf("Index after invalidate: %v", err)
	}
	if store.calls != 2 {
		t.Fatalf("invalidate should force a rebuild, store calls = %d", store.calls)
	}
}

func TestRetrieverSearchMergesBooks(t *testing.T) {
	store := &fakeStore{chunks: map[string][]Chunk{
		"a": {{BookID: "a", Seq: 0, Text: "林黛玉初进荣国府，步步留心。"}},
		"b": {{BookID: "b", Seq: 0, Text: "宋江怒杀阎婆惜，投奔梁山。"}},
	}}
	r := NewRetriever(store)

	hits, err := r.Search([]string{"a", "b"}, "林黛玉", 5)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(hits) != 1 {
		t.Fatalf("expected 1 hit across books, got %d", len(hits))
	}
	if hits[0].Chunk.BookID != "a" {
		t.Fatalf("wrong book: %s", hits[0].Chunk.BookID)
	}

	// Duplicate ids must not double the work or the results.
	hits, err = r.Search([]string{"a", "a"}, "林黛玉", 5)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(hits) != 1 {
		t.Fatalf("duplicate book id produced %d hits", len(hits))
	}
}

func TestRetrieverHonoursLimit(t *testing.T) {
	store := &fakeStore{chunks: map[string][]Chunk{
		"a": {
			{BookID: "a", Seq: 0, Text: "雨下得很大"},
			{BookID: "a", Seq: 1, Text: "雨一直没有停"},
			{BookID: "a", Seq: 2, Text: "雨声盖过了说话"},
		},
	}}
	r := NewRetriever(store)
	hits, err := r.Search([]string{"a"}, "雨", 2)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(hits) != 2 {
		t.Fatalf("expected 2 hits, got %d", len(hits))
	}
}
