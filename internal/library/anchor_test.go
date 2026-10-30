package library

import (
	"testing"

	"my-weread-desk/internal/story"
)

func TestAnchorRoundTrip(t *testing.T) {
	s := newStore(t)
	anchors := []story.CanonAnchor{
		{ID: "a1", BookID: "b", SceneSeq: 12, DeadlineSeq: 14, Kind: story.AnchorReveal,
			Summary: "身世被揭穿", Weight: 0.9, Source: "manual"},
		{ID: "a2", BookID: "b", SceneSeq: 3, DeadlineSeq: 5, Kind: story.AnchorClimax,
			Summary: "两人决裂", Weight: 0.8, Source: "llm"},
	}
	for _, a := range anchors {
		if err := s.UpsertAnchor(a); err != nil {
			t.Fatalf("UpsertAnchor: %v", err)
		}
	}

	got, err := s.Anchors("b")
	if err != nil {
		t.Fatalf("Anchors: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("anchors = %+v", got)
	}
	// Ordered by urgency, not insertion order.
	if got[0].ID != "a2" || got[1].ID != "a1" {
		t.Fatalf("order wrong: %+v", got)
	}
	if got[1].Summary != "身世被揭穿" {
		t.Fatalf("summary lost: %+v", got[1])
	}

	one, err := s.Anchor("b", "a1")
	if err != nil {
		t.Fatalf("Anchor: %v", err)
	}
	if one.Kind != story.AnchorReveal {
		t.Fatalf("kind = %q", one.Kind)
	}

	if err := s.DeleteAnchor("b", "a1"); err != nil {
		t.Fatalf("DeleteAnchor: %v", err)
	}
	if n, _ := s.AnchorCount("b"); n != 1 {
		t.Fatalf("count after delete = %d", n)
	}
	if _, err := s.Anchor("b", "a1"); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestAnchorNormalises(t *testing.T) {
	s := newStore(t)
	// A deadline before the canon scene would make the anchor unsatisfiable by
	// its own route, so it is widened. Bad weight and kind get defaults.
	if err := s.UpsertAnchor(story.CanonAnchor{
		ID: "a", BookID: "b", SceneSeq: 10, DeadlineSeq: 4, Weight: 5,
	}); err != nil {
		t.Fatalf("UpsertAnchor: %v", err)
	}
	a, err := s.Anchor("b", "a")
	if err != nil {
		t.Fatalf("Anchor: %v", err)
	}
	if a.DeadlineSeq != 10 {
		t.Fatalf("deadline = %d, want 10", a.DeadlineSeq)
	}
	if a.Weight != 0.8 {
		t.Fatalf("weight = %v, want 0.8", a.Weight)
	}
	if a.Kind != story.AnchorBeat {
		t.Fatalf("kind = %q", a.Kind)
	}
	if a.Source != "manual" {
		t.Fatalf("source = %q", a.Source)
	}
}

func TestReplaceAnchors(t *testing.T) {
	s := newStore(t)
	if err := s.ReplaceAnchors("b", []story.CanonAnchor{
		{ID: "old", BookID: "b", SceneSeq: 1, DeadlineSeq: 2, Summary: "旧"},
	}); err != nil {
		t.Fatalf("ReplaceAnchors: %v", err)
	}
	if err := s.ReplaceAnchors("b", []story.CanonAnchor{
		{ID: "new", BookID: "b", SceneSeq: 5, DeadlineSeq: 8, Summary: "新"},
	}); err != nil {
		t.Fatalf("ReplaceAnchors 2: %v", err)
	}
	got, _ := s.Anchors("b")
	if len(got) != 1 || got[0].ID != "new" {
		t.Fatalf("replace did not clear the old set: %+v", got)
	}
}

func TestAnchorsAreScopedPerBook(t *testing.T) {
	s := newStore(t)
	if err := s.UpsertAnchor(story.CanonAnchor{ID: "a", BookID: "b1", DeadlineSeq: 5, Summary: "一"}); err != nil {
		t.Fatalf("UpsertAnchor: %v", err)
	}
	if err := s.UpsertAnchor(story.CanonAnchor{ID: "a", BookID: "b2", DeadlineSeq: 5, Summary: "二"}); err != nil {
		t.Fatalf("UpsertAnchor: %v", err)
	}
	one, _ := s.Anchors("b1")
	two, _ := s.Anchors("b2")
	if len(one) != 1 || one[0].Summary != "一" {
		t.Fatalf("b1 = %+v", one)
	}
	if len(two) != 1 || two[0].Summary != "二" {
		t.Fatalf("b2 = %+v", two)
	}
}

func TestDeleteMissingAnchor(t *testing.T) {
	s := newStore(t)
	if err := s.DeleteAnchor("b", "nope"); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
