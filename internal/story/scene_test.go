package story

import (
	"strings"
	"testing"

	"my-weread-desk/internal/weread"
)

func TestChapterSceneCoversChapter(t *testing.T) {
	blocks := []weread.Block{
		{Kind: "heading", Text: "第一章"},
		{Kind: "image", Src: "/media/x.png"},
		{Kind: "para", Text: "他推开门，雨声一下子涌了进来。"},
		{Kind: "divider"},
		{Kind: "para", Text: "屋里没有人。"},
	}
	ch := weread.Chapter{UID: 42, Index: 1, Title: "第一章"}
	sc := ChapterScene("book", ch, blocks)

	if sc.ChapterUID != 42 || sc.ChapterIdx != 1 || sc.ChapterTitle != "第一章" {
		t.Fatalf("chapter metadata lost: %+v", sc)
	}
	if sc.BlockStart != 0 || sc.BlockEnd != len(blocks)-1 {
		t.Fatalf("block span = %d..%d", sc.BlockStart, sc.BlockEnd)
	}
	if !strings.Contains(sc.Summary, "雨声") {
		t.Fatalf("summary should come from the body: %q", sc.Summary)
	}
	if sc.Source != "chapter" {
		t.Fatalf("source = %q", sc.Source)
	}
	if sc.Participants == nil {
		t.Fatal("participants must be non-nil so JSON stays an array")
	}
}

func TestChapterSceneHandlesEmptyBlocks(t *testing.T) {
	sc := ChapterScene("b", weread.Chapter{UID: 1}, nil)
	if sc.BlockEnd != 0 {
		t.Fatalf("empty chapter block end = %d", sc.BlockEnd)
	}
	if sc.Summary != "" {
		t.Fatalf("summary = %q", sc.Summary)
	}
}

func TestFirstRunesDoesNotSplitUTF8(t *testing.T) {
	if got := firstRunes("一二三四五", 3); got != "一二三…" {
		t.Fatalf("firstRunes = %q", got)
	}
	if got := firstRunes("一二", 5); got != "一二" {
		t.Fatalf("short text changed: %q", got)
	}
}

func TestParseSceneFields(t *testing.T) {
	raw := "```json\n{\"summary\":\"黛玉进府\",\"location\":\"荣国府\",\"participants\":[\"黛玉\",\"宝玉\",\"黛玉\"]}\n```"
	f, err := ParseSceneFields(raw)
	if err != nil {
		t.Fatalf("ParseSceneFields: %v", err)
	}
	if f.Summary != "黛玉进府" || f.Location != "荣国府" {
		t.Fatalf("fields lost: %+v", f)
	}
	if len(f.Participants) != 2 {
		t.Fatalf("duplicates not removed: %+v", f.Participants)
	}
}

func TestParseSceneFieldsRejectsUnusableReply(t *testing.T) {
	if _, err := ParseSceneFields("{}"); err == nil {
		t.Fatal("empty object must be rejected")
	}
	if _, err := ParseSceneFields("抱歉，我无法分析这段文本。"); err == nil {
		t.Fatal("prose must be rejected")
	}
	if _, err := ParseSceneFields(""); err == nil {
		t.Fatal("empty reply must be rejected")
	}
}

func TestApplySceneFieldsKeepsDeterministicValues(t *testing.T) {
	sc := Scene{Summary: "原文摘要", Source: "chapter"}
	out := ApplySceneFields(sc, SceneFields{Location: "荣国府"})

	if out.Summary != "原文摘要" {
		t.Fatalf("empty model summary must not clear the original: %q", out.Summary)
	}
	if out.Location != "荣国府" || out.Source != "llm" {
		t.Fatalf("merge failed: %+v", out)
	}
}
