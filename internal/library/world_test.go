package library

import (
	"testing"

	"my-weread-desk/internal/story"
)

func TestWorldSheetRoundTrip(t *testing.T) {
	s := newStore(t)
	sheet := story.WorldSheet{
		Stats: []story.WorldStat{
			{Key: "月钱", Label: "月钱", Kind: story.StatResource,
				Min: 0, Max: 10, Initial: 2, Decay: 1, EvidenceSeq: 3},
			{Key: "体面", Label: "体面", Kind: story.StatReputation,
				Min: 0, Max: 100, Initial: 70},
		},
		Items: []story.WorldItem{
			{ID: "jade", Name: "通灵宝玉", Kind: "token", Owner: "贾宝玉",
				FirstSeq: 3, Effects: "表明身份", LimitPerBranch: 1},
			{ID: "letter", Name: "书信", LimitPerBranch: 2},
		},
	}
	if err := s.ReplaceWorldSheet("b", sheet); err != nil {
		t.Fatalf("ReplaceWorldSheet: %v", err)
	}

	got, err := s.WorldSheet("b")
	if err != nil {
		t.Fatalf("WorldSheet: %v", err)
	}
	if len(got.Stats) != 2 || len(got.Items) != 2 {
		t.Fatalf("sheet = %+v", got)
	}

	var monthly story.WorldStat
	for _, def := range got.Stats {
		if def.Key == "月钱" {
			monthly = def
		}
	}
	if monthly.Max != 10 || monthly.Decay != 1 || monthly.EvidenceSeq != 3 {
		t.Fatalf("stat lost: %+v", monthly)
	}
	byID := map[string]story.WorldItem{}
	for _, item := range got.Items {
		byID[item.ID] = item
	}
	if byID["jade"].Name != "通灵宝玉" || byID["letter"].LimitPerBranch != 2 {
		t.Fatalf("items = %+v", got.Items)
	}

	stats, items, err := s.WorldSheetCounts("b")
	if err != nil {
		t.Fatalf("WorldSheetCounts: %v", err)
	}
	if stats != 2 || items != 2 {
		t.Fatalf("counts = %d/%d", stats, items)
	}
}

func TestWorldSheetNormalises(t *testing.T) {
	s := newStore(t)
	if err := s.UpsertWorldStat("b", story.WorldStat{
		Key: "bad", Min: 5, Max: 1, Initial: 999,
	}); err != nil {
		t.Fatalf("UpsertWorldStat: %v", err)
	}
	if err := s.UpsertWorldItem("b", story.WorldItem{Name: "书信"}); err != nil {
		t.Fatalf("UpsertWorldItem: %v", err)
	}

	got, err := s.WorldSheet("b")
	if err != nil {
		t.Fatalf("WorldSheet: %v", err)
	}
	def := got.Stats[0]
	if def.Max <= def.Min {
		t.Fatalf("max not repaired: %+v", def)
	}
	if def.Initial > def.Max || def.Initial < def.Min {
		t.Fatalf("initial not clamped: %+v", def)
	}
	if def.Kind != story.StatResource || def.Source != "derived" {
		t.Fatalf("defaults missing: %+v", def)
	}
	if len(got.Items) != 1 || got.Items[0].ID == "" {
		t.Fatalf("item id not generated: %+v", got.Items)
	}
}

func TestDeleteWorldRows(t *testing.T) {
	s := newStore(t)
	if err := s.UpsertWorldStat("b", story.WorldStat{Key: "月钱"}); err != nil {
		t.Fatalf("UpsertWorldStat: %v", err)
	}
	if err := s.DeleteWorldStat("b", "月钱"); err != nil {
		t.Fatalf("DeleteWorldStat: %v", err)
	}
	if err := s.DeleteWorldStat("b", "月钱"); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}

	if err := s.UpsertWorldItem("b", story.WorldItem{Name: "玉"}); err != nil {
		t.Fatalf("UpsertWorldItem: %v", err)
	}
	sheet, _ := s.WorldSheet("b")
	if err := s.DeleteWorldItem("b", sheet.Items[0].ID); err != nil {
		t.Fatalf("DeleteWorldItem: %v", err)
	}
	if err := s.DeleteWorldItem("b", "missing"); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestWorldSheetMissingIsEmpty(t *testing.T) {
	s := newStore(t)
	got, err := s.WorldSheet("missing")
	if err != nil {
		t.Fatalf("WorldSheet: %v", err)
	}
	if got.Stats == nil || got.Items == nil {
		t.Fatal("missing sheet must decode to empty slices, not nil")
	}
	if len(got.Stats) != 0 || len(got.Items) != 0 {
		t.Fatalf("unexpected content: %+v", got)
	}
}
