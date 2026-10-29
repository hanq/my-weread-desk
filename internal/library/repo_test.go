package library

import (
	"path/filepath"
	"testing"

	"my-weread-desk/internal/weread"
)

func newStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "library.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestShelfRoundTrip(t *testing.T) {
	s := newStore(t)
	books := []weread.Book{
		{ID: "a", Title: "甲", Author: "作者甲", LastRead: 100, WordCount: 10},
		{ID: "b", Title: "乙", Finished: true, Secret: true, IsMP: false},
	}
	if err := s.ReplaceShelf(books); err != nil {
		t.Fatalf("ReplaceShelf: %v", err)
	}
	got, err := s.Books()
	if err != nil {
		t.Fatalf("Books: %v", err)
	}
	if len(got) != 2 || got[0].ID != "a" || !got[1].Finished || !got[1].Secret {
		t.Fatalf("books = %+v", got)
	}

	// A second sync with one book removed must drop it from the shelf view but
	// keep the cached content addressable by id.
	if err := s.ReplaceShelf(books[:1]); err != nil {
		t.Fatalf("ReplaceShelf 2: %v", err)
	}
	visible, _ := s.Books()
	if len(visible) != 1 {
		t.Fatalf("visible = %+v", visible)
	}
	if _, err := s.Book("b"); err != nil {
		t.Errorf("removed book should still be cached: %v", err)
	}
}

func TestChapterAndContentCache(t *testing.T) {
	s := newStore(t)
	chapters := []weread.Chapter{
		{UID: 1, Index: 1, Title: "一", WordCount: 100},
		{UID: 2, Index: 2, Title: "二", Tar: "https://res/1.tar", Format: "epub"},
	}
	if err := s.ReplaceChapters("b", chapters); err != nil {
		t.Fatalf("ReplaceChapters: %v", err)
	}
	got, err := s.Chapters("b")
	if err != nil || len(got) != 2 {
		t.Fatalf("Chapters: %v %+v", err, got)
	}
	if got[1].Tar != "https://res/1.tar" || got[1].Format != "epub" {
		t.Errorf("tar/format lost: %+v", got[1])
	}

	content := &weread.ChapterContent{
		BookID: "b", ChapterUID: 2, Title: "二", Format: "epub", FetchedAt: 42,
		Blocks: []weread.Block{{Kind: "para", Text: "正文"}},
	}
	if err := s.SaveChapterContent(content); err != nil {
		t.Fatalf("SaveChapterContent: %v", err)
	}
	loaded, err := s.ChapterContent("b", 2)
	if err != nil {
		t.Fatalf("ChapterContent: %v", err)
	}
	if len(loaded.Blocks) != 1 || loaded.Blocks[0].Text != "正文" || loaded.FetchedAt != 42 {
		t.Fatalf("content = %+v", loaded)
	}

	cached, err := s.CachedChapterUIDs("b")
	if err != nil || !cached[2] || cached[1] {
		t.Fatalf("cached = %+v err=%v", cached, err)
	}

	if err := s.DropChapterContent("b"); err != nil {
		t.Fatalf("DropChapterContent: %v", err)
	}
	if _, err := s.ChapterContent("b", 2); err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestProgressDirtyFlag(t *testing.T) {
	s := newStore(t)
	if err := s.SaveProgress(weread.Progress{BookID: "b", ChapterUID: 1, Offset: 10, Percent: 5}, true); err != nil {
		t.Fatalf("SaveProgress: %v", err)
	}
	dirty, err := s.DirtyProgress()
	if err != nil || len(dirty) != 1 || dirty[0].Offset != 10 {
		t.Fatalf("dirty = %+v err=%v", dirty, err)
	}
	// A clean save removes it from the dirty set.
	if err := s.SaveProgress(weread.Progress{BookID: "b", ChapterUID: 1, Offset: 12, Percent: 6}, false); err != nil {
		t.Fatalf("SaveProgress 2: %v", err)
	}
	dirty, _ = s.DirtyProgress()
	if len(dirty) != 0 {
		t.Fatalf("dirty after clean = %+v", dirty)
	}
	p, err := s.Progress("b")
	if err != nil || p.Offset != 12 {
		t.Fatalf("progress = %+v err=%v", p, err)
	}
}

func TestMediaRoundTrip(t *testing.T) {
	s := newStore(t)
	if err := s.SaveMedia("b", "epub_1_2.png", "image/png", []byte("\x89PNG")); err != nil {
		t.Fatalf("SaveMedia: %v", err)
	}
	data, mime, err := s.Media("b", "epub_1_2.png")
	if err != nil || string(data) != "\x89PNG" || mime != "image/png" {
		t.Fatalf("Media = %q %q %v", data, mime, err)
	}
	if !s.HasMedia("b", "epub_1_2.png") || s.HasMedia("b", "missing") {
		t.Error("HasMedia mismatch")
	}
}

func TestMPArticleCache(t *testing.T) {
	s := newStore(t)
	article := weread.MPArticle{ReviewID: "r1", Account: "MP_WXS_1", Title: "标题", CreateTime: 7}
	if err := s.SaveMPArticle(article, []weread.Block{{Kind: "para", Text: "内容"}}); err != nil {
		t.Fatalf("SaveMPArticle: %v", err)
	}
	got, blocks, err := s.MPArticle("r1")
	if err != nil || got.Title != "标题" || len(blocks) != 1 {
		t.Fatalf("MPArticle = %+v %+v %v", got, blocks, err)
	}
}
