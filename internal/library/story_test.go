package library

import (
	"testing"

	"my-weread-desk/internal/story"
)

func TestStoryJobResumes(t *testing.T) {
	s := newStore(t)
	keys := []string{"1", "2", "3"}

	job, err := s.InitStoryJob("b", StoryJobSceneExtract, keys)
	if err != nil {
		t.Fatalf("InitStoryJob: %v", err)
	}
	if job.Total != 3 || job.Done != 0 {
		t.Fatalf("initial job = %+v", job)
	}

	if err := s.MarkStoryItem("b", StoryJobSceneExtract, "1", "done", ""); err != nil {
		t.Fatalf("MarkStoryItem: %v", err)
	}

	// Re-initialising must preserve finished chapters: that is what makes a
	// crash resumable instead of re-billing every chapter.
	job, err = s.InitStoryJob("b", StoryJobSceneExtract, keys)
	if err != nil {
		t.Fatalf("InitStoryJob 2: %v", err)
	}
	if job.Done != 1 {
		t.Fatalf("resume lost progress: %+v", job)
	}

	pending, err := s.PendingStoryItems("b", StoryJobSceneExtract)
	if err != nil {
		t.Fatalf("PendingStoryItems: %v", err)
	}
	if len(pending) != 2 {
		t.Fatalf("pending = %v", pending)
	}

	// A failed item is retried on the next pass rather than left as a hole.
	if err := s.MarkStoryItem("b", StoryJobSceneExtract, "2", "failed", "boom"); err != nil {
		t.Fatalf("MarkStoryItem failed: %v", err)
	}
	pending, _ = s.PendingStoryItems("b", StoryJobSceneExtract)
	if len(pending) != 2 {
		t.Fatalf("failed item not retried: %v", pending)
	}

	if err := s.SetStoryJobStatus("b", StoryJobSceneExtract, "done", ""); err != nil {
		t.Fatalf("SetStoryJobStatus: %v", err)
	}
	row, err := s.StoryJob("b", StoryJobSceneExtract)
	if err != nil {
		t.Fatalf("StoryJob: %v", err)
	}
	if row.Status != "done" || row.Done != 1 || row.Failed != 1 {
		t.Fatalf("counters = %+v", row)
	}
}

func TestStorySceneRoundTrip(t *testing.T) {
	s := newStore(t)
	scenes := []story.Scene{
		{BookID: "b", Seq: 0, ChapterUID: 1, ChapterIdx: 1, ChapterTitle: "一",
			BlockStart: 0, BlockEnd: 5, Summary: "甲", Participants: []string{"A"}, Source: "llm"},
		{BookID: "b", Seq: 1, ChapterUID: 2, ChapterIdx: 2, ChapterTitle: "二",
			BlockStart: 0, BlockEnd: 3, Summary: "乙", Participants: []string{}, Source: "chapter"},
	}
	if err := s.ReplaceStoryScenes("b", scenes); err != nil {
		t.Fatalf("ReplaceStoryScenes: %v", err)
	}

	got, err := s.StoryScenes("b")
	if err != nil {
		t.Fatalf("StoryScenes: %v", err)
	}
	if len(got) != 2 || got[1].ChapterUID != 2 {
		t.Fatalf("scenes = %+v", got)
	}
	if len(got[0].Participants) != 1 || got[0].Participants[0] != "A" {
		t.Fatalf("participants lost: %+v", got[0].Participants)
	}
	if got[1].Participants == nil {
		t.Fatal("participants must decode to an empty slice, not nil")
	}

	// Upsert replaces the row for the same seq instead of appending.
	scenes[0].Summary = "改写"
	if err := s.UpsertStoryScene(scenes[0]); err != nil {
		t.Fatalf("UpsertStoryScene: %v", err)
	}
	got, _ = s.StoryScenes("b")
	if got[0].Summary != "改写" {
		t.Fatalf("upsert did not replace: %+v", got[0])
	}
	if n, _ := s.SceneCount("b"); n != 2 {
		t.Fatalf("duplicate rows: %d", n)
	}

	if err := s.DropStoryGraph("b"); err != nil {
		t.Fatalf("DropStoryGraph: %v", err)
	}
	if got, _ := s.StoryScenes("b"); len(got) != 0 {
		t.Fatalf("scenes survived drop: %d", len(got))
	}
	if _, err := s.StoryJob("b", StoryJobSceneExtract); err == nil {
		t.Fatal("job row survived drop")
	}
}

func TestStoryJobUnknown(t *testing.T) {
	s := newStore(t)
	if _, err := s.StoryJob("missing", StoryJobSceneExtract); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
	if pending, err := s.PendingStoryItems("missing", StoryJobSceneExtract); err != nil || len(pending) != 0 {
		t.Fatalf("pending = %v, err = %v", pending, err)
	}
}
