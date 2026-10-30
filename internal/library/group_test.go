package library

import (
	"database/sql"
	"path/filepath"
	"testing"

	"my-weread-desk/internal/weread"
)

func TestGroupsRoundTrip(t *testing.T) {
	s := newStore(t)
	books := []weread.Book{
		{ID: "a", Title: "甲", GroupID: "g1", GroupName: "小说"},
		{ID: "b", Title: "乙", GroupID: "g1", GroupName: "小说"},
		{ID: "c", Title: "丙"},
		{ID: "d", Title: "丁", IsAudio: true, TrackCount: 12},
	}
	if err := s.ReplaceShelf(books); err != nil {
		t.Fatalf("ReplaceShelf: %v", err)
	}
	if err := s.ReplaceGroups([]weread.ShelfGroup{{ID: "g1", Name: "小说", Source: "cloud"}}); err != nil {
		t.Fatalf("ReplaceGroups: %v", err)
	}

	groups, err := s.Groups()
	if err != nil {
		t.Fatalf("Groups: %v", err)
	}
	if len(groups) != 1 {
		t.Fatalf("groups = %+v", groups)
	}
	if groups[0].Name != "小说" || len(groups[0].BookIDs) != 2 {
		t.Fatalf("membership lost: %+v", groups[0])
	}

	got, err := s.Book("d")
	if err != nil {
		t.Fatalf("Book: %v", err)
	}
	if !got.IsAudio || got.TrackCount != 12 {
		t.Fatalf("audio fields lost: %+v", got)
	}
}

func TestLocalGroupSurvivesShelfSync(t *testing.T) {
	s := newStore(t)
	if err := s.ReplaceShelf([]weread.Book{{ID: "a", Title: "甲", GroupID: "g1", GroupName: "小说"}}); err != nil {
		t.Fatalf("ReplaceShelf: %v", err)
	}
	if err := s.ReplaceGroups([]weread.ShelfGroup{{ID: "g1", Name: "小说", Source: "cloud"}}); err != nil {
		t.Fatalf("ReplaceGroups: %v", err)
	}

	if err := s.UpsertGroup(weread.ShelfGroup{ID: "local-1", Name: "稍后读", Source: "local"}); err != nil {
		t.Fatalf("UpsertGroup: %v", err)
	}
	if err := s.SetBookGroup([]string{"a"}, "local-1", "稍后读"); err != nil {
		t.Fatalf("SetBookGroup: %v", err)
	}

	// The account still reports the book in g1. A sync must not move it back:
	// the local assignment is the newer intent and is marked as such.
	if err := s.ReplaceShelf([]weread.Book{{ID: "a", Title: "甲", GroupID: "g1", GroupName: "小说"}}); err != nil {
		t.Fatalf("ReplaceShelf 2: %v", err)
	}
	book, err := s.Book("a")
	if err != nil {
		t.Fatalf("Book: %v", err)
	}
	if book.GroupID != "local-1" || book.GroupName != "稍后读" {
		t.Fatalf("local assignment was overwritten: %+v", book)
	}

	// Cloud folders are mirrored, so a second sync keeps both the cloud folder
	// and the local one.
	if err := s.ReplaceGroups([]weread.ShelfGroup{{ID: "g1", Name: "小说", Source: "cloud"}}); err != nil {
		t.Fatalf("ReplaceGroups 2: %v", err)
	}
	groups, _ := s.Groups()
	if len(groups) != 2 {
		t.Fatalf("expected cloud + local folders, got %+v", groups)
	}
}

func TestDeleteGroupClearsMembership(t *testing.T) {
	s := newStore(t)
	if err := s.ReplaceShelf([]weread.Book{
		{ID: "a", Title: "甲", GroupID: "g1", GroupName: "小说"},
		{ID: "b", Title: "乙", GroupID: "g1", GroupName: "小说"},
	}); err != nil {
		t.Fatalf("ReplaceShelf: %v", err)
	}
	if err := s.ReplaceGroups([]weread.ShelfGroup{{ID: "g1", Name: "小说", Source: "cloud"}}); err != nil {
		t.Fatalf("ReplaceGroups: %v", err)
	}
	if err := s.DeleteGroup("g1"); err != nil {
		t.Fatalf("DeleteGroup: %v", err)
	}
	for _, id := range []string{"a", "b"} {
		book, err := s.Book(id)
		if err != nil {
			t.Fatalf("Book(%s): %v", id, err)
		}
		if book.GroupID != "" || book.GroupName != "" {
			t.Fatalf("book %s still grouped: %+v", id, book)
		}
	}
	if groups, _ := s.Groups(); len(groups) != 0 {
		t.Fatalf("folder row survived: %+v", groups)
	}
}

func TestRenameGroupUpdatesBooks(t *testing.T) {
	s := newStore(t)
	if err := s.ReplaceShelf([]weread.Book{{ID: "a", Title: "甲", GroupID: "g1", GroupName: "小说"}}); err != nil {
		t.Fatalf("ReplaceShelf: %v", err)
	}
	if err := s.ReplaceGroups([]weread.ShelfGroup{{ID: "g1", Name: "小说", Source: "cloud"}}); err != nil {
		t.Fatalf("ReplaceGroups: %v", err)
	}
	if err := s.RenameGroup("g1", "长篇小说"); err != nil {
		t.Fatalf("RenameGroup: %v", err)
	}
	book, _ := s.Book("a")
	if book.GroupName != "长篇小说" {
		t.Fatalf("denormalised name stale: %+v", book)
	}
}

// TestMigrationAddsGroupColumns opens a database written by the previous schema.
// SQLite has no ADD COLUMN IF NOT EXISTS, so this exercises the PRAGMA path that
// keeps existing installs upgradable.
func TestMigrationAddsGroupColumns(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old.db")
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open raw: %v", err)
	}
	_, err = raw.Exec(`CREATE TABLE book (
		id TEXT PRIMARY KEY,
		title TEXT NOT NULL DEFAULT '',
		author TEXT NOT NULL DEFAULT '',
		cover TEXT NOT NULL DEFAULT '',
		category TEXT NOT NULL DEFAULT '',
		intro TEXT NOT NULL DEFAULT '',
		rating REAL NOT NULL DEFAULT 0,
		rating_count INTEGER NOT NULL DEFAULT 0,
		word_count INTEGER NOT NULL DEFAULT 0,
		finished INTEGER NOT NULL DEFAULT 0,
		secret INTEGER NOT NULL DEFAULT 0,
		is_mp INTEGER NOT NULL DEFAULT 0,
		last_read INTEGER NOT NULL DEFAULT 0,
		update_at INTEGER NOT NULL DEFAULT 0,
		shelf_order INTEGER NOT NULL DEFAULT 0,
		in_shelf INTEGER NOT NULL DEFAULT 0,
		format TEXT NOT NULL DEFAULT '',
		updated_at INTEGER NOT NULL DEFAULT 0
	)`)
	if err != nil {
		t.Fatalf("create v4 table: %v", err)
	}
	if err := raw.Close(); err != nil {
		t.Fatalf("close raw: %v", err)
	}

	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open after v4 schema: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	// The migrated table must accept the new columns and keep working.
	if err := s.ReplaceShelf([]weread.Book{{ID: "a", Title: "甲", GroupID: "g1", GroupName: "小说"}}); err != nil {
		t.Fatalf("ReplaceShelf after migration: %v", err)
	}
	book, err := s.Book("a")
	if err != nil {
		t.Fatalf("Book: %v", err)
	}
	if book.GroupID != "g1" {
		t.Fatalf("group column not usable: %+v", book)
	}
}
