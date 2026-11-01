package app

import (
	"testing"

	"my-weread-desk/internal/library"
	"my-weread-desk/internal/rag"
	"my-weread-desk/internal/weread"
)

func TestSmartBooksOnlyIncludesPreparedBooks(t *testing.T) {
	a, store := newStatusTestApp(t)
	if err := store.UpsertBook(weread.Book{ID: "ready", Title: "三体", Category: "小说"}); err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertBook(weread.Book{ID: "empty", Title: "未准备", Category: "小说"}); err != nil {
		t.Fatal(err)
	}
	chunks := []rag.Chunk{{
		BookID: "ready", Seq: 0, ChapterUID: 1, ChapterIdx: 1, ChapterTitle: "第一章",
		BlockStart: 0, BlockEnd: 0, CharCount: 12, Text: "三体世界",
	}}
	if err := store.ReplaceRAGChunks("ready", chunks); err != nil {
		t.Fatal(err)
	}

	list, err := a.SmartBooks()
	if err != nil {
		t.Fatalf("SmartBooks: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("smart books = %d, want 1: %+v", len(list), list)
	}
	if list[0].Book.ID != "ready" {
		t.Fatalf("book = %s", list[0].Book.ID)
	}
	if list[0].Kind != string(library.BookKindFiction) {
		t.Fatalf("kind = %q", list[0].Kind)
	}
	if !list[0].Capabilities.Ask || list[0].Capabilities.Immerse || list[0].Capabilities.Rewrite {
		t.Fatalf("capabilities = %+v", list[0].Capabilities)
	}
}
