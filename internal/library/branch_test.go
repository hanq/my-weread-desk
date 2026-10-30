package library

import (
	"testing"

	"my-weread-desk/internal/story"
)

func TestBranchRoundTrip(t *testing.T) {
	s := newStore(t)
	b := story.Branch{
		ID: "br1", BookID: "b", ForkSeq: 3, RejoinSeq: 7,
		Title: "世界线", Status: story.BranchActive,
		State:    *story.NewState(),
		MemoryMD: "---\nschema: 1\n---\n",
	}
	b.State.Assumptions = []string{"真相未揭露"}
	b.State.Dead = []string{"父亲"}
	if err := s.UpsertBranch(b); err != nil {
		t.Fatalf("UpsertBranch: %v", err)
	}

	got, err := s.Branch("br1")
	if err != nil {
		t.Fatalf("Branch: %v", err)
	}
	if got.ForkSeq != 3 || got.RejoinSeq != 7 {
		t.Fatalf("branch = %+v", got)
	}
	if len(got.State.Assumptions) != 1 || got.State.Assumptions[0] != "真相未揭露" {
		t.Fatalf("state lost: %+v", got.State)
	}
	if !got.State.IsDead("父亲") {
		t.Fatalf("dead ledger lost: %+v", got.State.Dead)
	}
	if got.State.Facts == nil || got.State.Knows == nil {
		t.Fatal("state maps must be normalised after decode")
	}
	if got.CreatedAt == 0 {
		t.Fatal("created_at not set")
	}

	// Updating keeps created_at and applies status/summary.
	b.Status = story.BranchJoined
	b.Summary = "回到了原著"
	if err := s.UpsertBranch(b); err != nil {
		t.Fatalf("UpsertBranch 2: %v", err)
	}
	got, _ = s.Branch("br1")
	if got.Status != story.BranchJoined || got.Summary != "回到了原著" {
		t.Fatalf("update lost: %+v", got)
	}
	if n, _ := s.BranchCount("b"); n != 1 {
		t.Fatalf("branch count = %d", n)
	}
}

func TestBranchScenesAndDelete(t *testing.T) {
	s := newStore(t)
	scenes := []story.BranchScene{
		{BranchID: "br", Seq: 0, CanonSeq: 3, Text: "甲", Summary: "一",
			Verdicts: []story.Violation{{Rule: "测试", Severity: story.SeveritySoft}}},
		{BranchID: "br", Seq: 1, CanonSeq: 4, Text: "乙", Summary: "二"},
	}
	for _, sc := range scenes {
		if err := s.AppendBranchScene(sc); err != nil {
			t.Fatalf("AppendBranchScene: %v", err)
		}
	}

	got, err := s.BranchScenes("br")
	if err != nil {
		t.Fatalf("BranchScenes: %v", err)
	}
	if len(got) != 2 || got[1].CanonSeq != 4 {
		t.Fatalf("scenes = %+v", got)
	}
	if len(got[0].Verdicts) != 1 || got[0].Verdicts[0].Rule != "测试" {
		t.Fatalf("verdicts lost: %+v", got[0].Verdicts)
	}

	// Regenerating the same seq replaces rather than duplicates.
	scenes[0].Text = "改写"
	if err := s.AppendBranchScene(scenes[0]); err != nil {
		t.Fatalf("AppendBranchScene 2: %v", err)
	}
	got, _ = s.BranchScenes("br")
	if len(got) != 2 || got[0].Text != "改写" {
		t.Fatalf("upsert failed: %+v", got)
	}

	last, err := s.LastBranchScene("br")
	if err != nil || last.Seq != 1 {
		t.Fatalf("last scene = %+v, err = %v", last, err)
	}
	if n, _ := s.BranchSceneCount("br"); n != 2 {
		t.Fatalf("scene count = %d", n)
	}

	if err := s.DeleteBranch("br"); err != nil {
		t.Fatalf("DeleteBranch: %v", err)
	}
	if got, _ := s.BranchScenes("br"); len(got) != 0 {
		t.Fatalf("scenes survived delete: %d", len(got))
	}
	if _, err := s.Branch("br"); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestBranchMissing(t *testing.T) {
	s := newStore(t)
	if _, err := s.Branch("nope"); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
	if _, err := s.LastBranchScene("nope"); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
	if got, err := s.Branches("nope"); err != nil || len(got) != 0 {
		t.Fatalf("branches = %v, err = %v", got, err)
	}
}
