package library

import (
	"errors"
	"testing"

	"my-weread-desk/internal/rag"
)

func TestRAGIndexRoundTrip(t *testing.T) {
	s := newStore(t)
	chunks := []rag.Chunk{
		{BookID: "b", Seq: 0, ChapterUID: 1, ChapterIdx: 1, ChapterTitle: "一",
			BlockStart: 0, BlockEnd: 2, CharCount: 20, Text: "林黛玉初进荣国府。"},
		{BookID: "b", Seq: 1, ChapterUID: 1, ChapterIdx: 1, ChapterTitle: "一",
			BlockStart: 3, BlockEnd: 5, CharCount: 18, Text: "她步步留心，时时在意。"},
	}
	if err := s.ReplaceRAGChunks("b", chunks); err != nil {
		t.Fatalf("ReplaceRAGChunks: %v", err)
	}

	meta, err := s.RAGIndex("b")
	if err != nil {
		t.Fatalf("RAGIndex: %v", err)
	}
	if meta.Status != "ready" || meta.ChunkCount != 2 || meta.CharCount != 38 {
		t.Fatalf("meta = %+v", meta)
	}
	if meta.ChunkerVersion != RAGChunkerVersion {
		t.Fatalf("chunker version = %d", meta.ChunkerVersion)
	}

	got, err := s.RAGChunks("b")
	if err != nil {
		t.Fatalf("RAGChunks: %v", err)
	}
	if len(got) != 2 || got[1].BlockStart != 3 || got[0].Text == "" {
		t.Fatalf("chunks = %+v", got)
	}

	// Re-indexing replaces rather than appends.
	if err := s.ReplaceRAGChunks("b", chunks[:1]); err != nil {
		t.Fatalf("ReplaceRAGChunks 2: %v", err)
	}
	if got, _ := s.RAGChunks("b"); len(got) != 1 {
		t.Fatalf("stale chunks survived: %d", len(got))
	}
}

func TestDropRAGIndex(t *testing.T) {
	s := newStore(t)
	if err := s.ReplaceRAGChunks("b", []rag.Chunk{{BookID: "b", Seq: 0, Text: "正文"}}); err != nil {
		t.Fatalf("ReplaceRAGChunks: %v", err)
	}
	if err := s.DropRAGIndex("b"); err != nil {
		t.Fatalf("DropRAGIndex: %v", err)
	}
	if _, err := s.RAGIndex("b"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
	if got, _ := s.RAGChunks("b"); len(got) != 0 {
		t.Fatalf("chunks survived the drop: %d", len(got))
	}
}

func TestSetRAGStatusKeepsFailureVisible(t *testing.T) {
	s := newStore(t)
	if err := s.SetRAGStatus("b", "failed", "没有缓存章节"); err != nil {
		t.Fatalf("SetRAGStatus: %v", err)
	}
	meta, err := s.RAGIndex("b")
	if err != nil {
		t.Fatalf("RAGIndex: %v", err)
	}
	if meta.Status != "failed" || meta.Error != "没有缓存章节" {
		t.Fatalf("meta = %+v", meta)
	}

	// A successful index clears the previous error.
	if err := s.ReplaceRAGChunks("b", []rag.Chunk{{BookID: "b", Seq: 0, Text: "正文"}}); err != nil {
		t.Fatalf("ReplaceRAGChunks: %v", err)
	}
	meta, _ = s.RAGIndex("b")
	if meta.Status != "ready" || meta.Error != "" {
		t.Fatalf("status not cleared: %+v", meta)
	}
}

func TestIndexedBooksFiltersReady(t *testing.T) {
	s := newStore(t)
	if err := s.ReplaceRAGChunks("ready", []rag.Chunk{{BookID: "ready", Seq: 0, Text: "正文"}}); err != nil {
		t.Fatalf("ReplaceRAGChunks: %v", err)
	}
	if err := s.SetRAGStatus("failed", "failed", "boom"); err != nil {
		t.Fatalf("SetRAGStatus: %v", err)
	}

	got, err := s.IndexedBooks([]string{"ready", "failed", "missing"})
	if err != nil {
		t.Fatalf("IndexedBooks: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected only the ready book, got %+v", got)
	}
	if _, ok := got["ready"]; !ok {
		t.Fatalf("ready book missing: %+v", got)
	}
}
