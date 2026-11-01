package app

import (
	"path/filepath"
	"strings"
	"testing"

	"my-weread-desk/internal/library"
	"my-weread-desk/internal/rag"
	"my-weread-desk/internal/story"
	"my-weread-desk/internal/weread"
)

func newStatusTestApp(t *testing.T) (*App, *library.Store) {
	t.Helper()
	store, err := library.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return &App{store: store}, store
}

func TestAIStatusCapabilities(t *testing.T) {
	a, store := newStatusTestApp(t)
	if err := store.UpsertBook(weread.Book{ID: "b1", Title: "三体", Category: "小说"}); err != nil {
		t.Fatalf("book: %v", err)
	}

	status, err := a.AIStatus("b1")
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if status.Kind != string(library.BookKindFiction) {
		t.Fatalf("kind = %q", status.Kind)
	}
	if status.Text.Status != AIStageNone || status.Scenes.Status != AIStageNone || status.World.Status != AIStageNone {
		t.Fatalf("unexpected initial stages: %+v", status)
	}
	if status.Capabilities.Ask || status.Capabilities.Immerse || status.Capabilities.Rewrite {
		t.Fatalf("nothing should be ready yet: %+v", status.Capabilities)
	}
	if err := a.requireTextIndex("b1"); err == nil || !strings.Contains(err.Error(), "文本索引") {
		t.Fatalf("requireTextIndex before index = %v", err)
	}

	chunks := []rag.Chunk{{
		BookID: "b1", Seq: 0, ChapterUID: 1, ChapterIdx: 1, ChapterTitle: "第一章",
		BlockStart: 0, BlockEnd: 0, CharCount: 12, Text: "三体世界",
	}}
	if err := store.ReplaceRAGChunks("b1", chunks); err != nil {
		t.Fatalf("rag chunks: %v", err)
	}
	status, err = a.AIStatus("b1")
	if err != nil {
		t.Fatalf("status after text: %v", err)
	}
	if status.Text.Status != AIStageReady {
		t.Fatalf("text stage = %+v", status.Text)
	}
	if !status.Capabilities.Ask || status.Capabilities.Immerse || status.Capabilities.Rewrite {
		t.Fatalf("capabilities after text = %+v", status.Capabilities)
	}
	if err := a.requireTextIndex("b1"); err != nil {
		t.Fatalf("requireTextIndex: %v", err)
	}

	if err := store.UpsertStoryScene(story.Scene{
		BookID: "b1", Seq: 0, ChapterUID: 1, ChapterIdx: 1, ChapterTitle: "第一章",
		BlockStart: 0, BlockEnd: 0, Summary: "叶文洁收到回信", Source: "chapter",
	}); err != nil {
		t.Fatalf("scene: %v", err)
	}
	status, err = a.AIStatus("b1")
	if err != nil {
		t.Fatalf("status after scenes: %v", err)
	}
	if status.Scenes.Status != AIStageReady {
		t.Fatalf("scene stage = %+v", status.Scenes)
	}
	if !status.Capabilities.Immerse || status.Capabilities.Rewrite {
		t.Fatalf("capabilities after scenes = %+v", status.Capabilities)
	}
	if err := a.requireScenes("b1"); err != nil {
		t.Fatalf("requireScenes: %v", err)
	}

	sheet := story.WorldSheet{
		BookID: "b1",
		Stats: []story.WorldStat{{
			Key: "money", Label: "钱", Kind: story.StatResource,
			Min: 0, Max: 100, Initial: 10,
		}},
	}
	if err := store.ReplaceWorldSheet("b1", sheet); err != nil {
		t.Fatalf("world sheet: %v", err)
	}
	status, err = a.AIStatus("b1")
	if err != nil {
		t.Fatalf("status after world: %v", err)
	}
	if status.World.Status != AIStageReady {
		t.Fatalf("world stage = %+v", status.World)
	}
	if !status.Capabilities.Rewrite {
		t.Fatalf("rewrite should be ready: %+v", status.Capabilities)
	}
	if err := a.requireWorld("b1"); err != nil {
		t.Fatalf("requireWorld: %v", err)
	}
}

func TestFictionGateAndUserOverride(t *testing.T) {
	a, store := newStatusTestApp(t)
	if err := store.UpsertBook(weread.Book{ID: "t1", Title: "高等数学", Category: "教材"}); err != nil {
		t.Fatalf("book: %v", err)
	}
	if err := a.requireFiction("t1"); err == nil || !strings.Contains(err.Error(), "仅支持虚构叙事") {
		t.Fatalf("requireFiction = %v", err)
	}

	if _, err := a.SetBookKind("t1", string(library.BookKindFiction)); err != nil {
		t.Fatalf("SetBookKind: %v", err)
	}
	if err := a.requireFiction("t1"); err != nil {
		t.Fatalf("requireFiction after override: %v", err)
	}

	status, err := a.AIStatus("t1")
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if status.Kind != string(library.BookKindFiction) || status.KindSource != "user" {
		t.Fatalf("override not persisted: %+v", status)
	}

	if _, err := a.SetBookKind("t1", "novel"); err == nil {
		t.Fatal("unknown kind should be rejected")
	}
}
