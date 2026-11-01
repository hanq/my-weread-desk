package library

import (
	"path/filepath"
	"testing"
)

func TestBookAIRoundTrip(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer store.Close()

	if err := store.UpsertBookAI("b1", string(BookKindFiction), "auto"); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	ai, err := store.BookAI("b1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if ai.Kind != string(BookKindFiction) || ai.KindSource != "auto" {
		t.Fatalf("ai = %+v", ai)
	}

	// A user correction replaces an automatic guess and must keep its source.
	if err := store.UpsertBookAI("b1", string(BookKindTextbook), "user"); err != nil {
		t.Fatalf("correct: %v", err)
	}
	ai, err = store.BookAI("b1")
	if err != nil {
		t.Fatalf("get corrected: %v", err)
	}
	if ai.Kind != string(BookKindTextbook) || ai.KindSource != "user" {
		t.Fatalf("corrected ai = %+v", ai)
	}

	if _, err := store.BookAI("missing"); err != ErrNotFound {
		t.Fatalf("missing error = %v", err)
	}
}

func TestBookAIMap(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer store.Close()

	if err := store.UpsertBookAI("b1", string(BookKindFiction), "auto"); err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertBookAI("b2", string(BookKindTextbook), "user"); err != nil {
		t.Fatal(err)
	}

	got, err := store.BookAIMap([]string{"b1", "b2", "b3", "b1"})
	if err != nil {
		t.Fatalf("map: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("map len = %d, want 2", len(got))
	}
	if got["b1"].Kind != string(BookKindFiction) {
		t.Fatalf("b1 = %+v", got["b1"])
	}
	if got["b2"].KindSource != "user" {
		t.Fatalf("b2 = %+v", got["b2"])
	}
	if _, ok := got["b3"]; ok {
		t.Fatal("missing book should not be in the map")
	}
}
