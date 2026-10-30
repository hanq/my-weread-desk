package story

import (
	"strings"
	"testing"
)

func TestParseChoices(t *testing.T) {
	raw := "```json\n[" +
		`{"text":"说出真相"},` +
		`{"text":"保持沉默"},` +
		`{"text":"说出真相"},` +
		`{"text":"   "}` +
		"]\n```"
	got, err := ParseChoices(raw)
	if err != nil {
		t.Fatalf("ParseChoices: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 unique choices, got %d: %+v", len(got), got)
	}
	if got[0].ID != "c1" || got[1].ID != "c2" {
		t.Fatalf("ids not assigned: %+v", got)
	}
	if got[0].Text != "说出真相" {
		t.Fatalf("text = %q", got[0].Text)
	}
}

func TestParseChoicesCapsAtThree(t *testing.T) {
	raw := `[{"text":"a"},{"text":"b"},{"text":"c"},{"text":"d"}]`
	got, err := ParseChoices(raw)
	if err != nil {
		t.Fatalf("ParseChoices: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3, got %d", len(got))
	}
}

func TestParseChoicesRejectsUnusableReply(t *testing.T) {
	if _, err := ParseChoices("没有选项"); err == nil {
		t.Fatal("prose must be rejected")
	}
	if _, err := ParseChoices("[]"); err == nil {
		t.Fatal("empty array must be rejected")
	}
}

func TestBuildChoicesPrompt(t *testing.T) {
	system, user := BuildChoicesPrompt("红楼梦", "他推门走了出去，雨声涌了进来。")
	if !strings.Contains(system, "JSON") {
		t.Fatalf("system = %q", system)
	}
	for _, want := range []string{"红楼梦", "他推门走了出去", "2 到 3 个", "不要写结果"} {
		if !strings.Contains(user, want) {
			t.Fatalf("prompt missing %q", want)
		}
	}
}
