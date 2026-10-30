package story

import (
	"encoding/json"
	"strings"
	"testing"
)

func validMemory(t *testing.T, title, author string) string {
	t.Helper()
	m := NewMemory(BookRef{Title: title, Author: author})
	md, err := m.Render()
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	return md
}

func TestBookKeyNormalises(t *testing.T) {
	a := BookKey("红楼梦", "曹雪芹")
	b := BookKey("  红楼梦 ", " 曹雪芹")
	if a != b {
		t.Fatalf("normalisation failed: %q vs %q", a, b)
	}
	if a == BookKey("红楼梦", "高鹗") {
		t.Fatal("different authors must produce different keys")
	}
	if !strings.HasPrefix(a, "sha256:") {
		t.Fatalf("key format = %q", a)
	}
}

func TestBranchPackageRoundTrip(t *testing.T) {
	md := validMemory(t, "红楼梦", "曹雪芹")
	b := Branch{
		ID: "b1", BookID: "local", ForkSeq: 3, RejoinSeq: 7,
		Title: "世界线", Status: BranchActive, Summary: "摘要",
		State:    *NewState(),
		MemoryMD: md,
	}
	b.State.Assumptions = []string{"真相未揭露"}
	scenes := []BranchScene{
		{BranchID: "b1", Seq: 0, CanonSeq: 3, Text: "第一幕", Summary: "一",
			Choices: []Choice{{ID: "c1", Text: "走"}}},
		{BranchID: "b1", Seq: 1, CanonSeq: 4, Text: "第二幕", Summary: "二"},
	}

	pkg := BuildBranchPackage(b, scenes, PackageAuthor{Name: "reader", Note: "试读"})
	raw, err := json.Marshal(pkg)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	parsed, err := ParseBranchPackage(string(raw))
	if err != nil {
		t.Fatalf("ParseBranchPackage: %v", err)
	}
	if parsed.Schema != BranchPackageSchema {
		t.Fatalf("schema = %d", parsed.Schema)
	}
	if !parsed.MatchesBook("红楼梦", "曹雪芹") {
		t.Fatalf("book key mismatch: %q", parsed.BookKey)
	}
	if len(parsed.Scenes) != 2 || parsed.Scenes[0].Text != "第一幕" {
		t.Fatalf("scenes lost: %+v", parsed.Scenes)
	}
	if len(parsed.Scenes[0].Choices) != 1 || parsed.Scenes[0].Choices[0].Text != "走" {
		t.Fatalf("choices lost: %+v", parsed.Scenes[0].Choices)
	}
	if len(parsed.State.Assumptions) != 1 {
		t.Fatalf("state lost: %+v", parsed.State)
	}
	if parsed.Author.Note != "试读" {
		t.Fatalf("author note lost: %+v", parsed.Author)
	}
	if !parsed.MatchesBook("红楼梦", "曹雪芹") {
		t.Fatal("MatchesBook should accept the same book")
	}
	if parsed.MatchesBook("水浒传", "施耐庵") {
		t.Fatal("MatchesBook should reject another book")
	}
}

func TestParseBranchPackageRejectsWrongSchema(t *testing.T) {
	md := validMemory(t, "红楼梦", "曹雪芹")
	b := Branch{ID: "b", Title: "t", MemoryMD: md, State: *NewState()}
	raw, _ := json.Marshal(BuildBranchPackage(b, nil, PackageAuthor{}))
	bad := strings.Replace(string(raw), `"schema":1`, `"schema":99`, 1)
	if bad == string(raw) {
		t.Fatal("fixture did not contain the schema field")
	}
	if _, err := ParseBranchPackage(bad); err == nil {
		t.Fatal("unknown schema must be rejected")
	}
}

func TestParseBranchPackageRejectsOversizedScene(t *testing.T) {
	md := validMemory(t, "红楼梦", "曹雪芹")
	pkg := BranchPackage{
		Schema:   BranchPackageSchema,
		BookKey:  BookKey("红楼梦", "曹雪芹"),
		MemoryMD: md,
		State:    *NewState(),
		Scenes: []BranchScene{
			{Seq: 0, Text: strings.Repeat("字", maxPackageSceneRunes+1)},
		},
	}
	raw, _ := json.Marshal(pkg)
	if _, err := ParseBranchPackage(string(raw)); err == nil {
		t.Fatal("oversized scene must be rejected")
	}
}

func TestParseBranchPackageRejectsInvalidMemory(t *testing.T) {
	pkg := BranchPackage{
		Schema:   BranchPackageSchema,
		BookKey:  BookKey("红楼梦", "曹雪芹"),
		MemoryMD: "这不是宪法",
		State:    *NewState(),
		Scenes:   []BranchScene{{Seq: 0, Text: "正文"}},
	}
	raw, _ := json.Marshal(pkg)
	if _, err := ParseBranchPackage(string(raw)); err == nil {
		t.Fatal("invalid constitution must be rejected")
	}
}

func TestParseBranchPackageRejectsMissingKey(t *testing.T) {
	md := validMemory(t, "红楼梦", "曹雪芹")
	pkg := BranchPackage{
		Schema: BranchPackageSchema, MemoryMD: md, State: *NewState(),
		Scenes: []BranchScene{{Seq: 0, Text: "正文"}},
	}
	raw, _ := json.Marshal(pkg)
	if _, err := ParseBranchPackage(string(raw)); err == nil {
		t.Fatal("missing bookKey must be rejected")
	}
}

func TestParseBranchPackageRejectsEmptyAndHugePayload(t *testing.T) {
	if _, err := ParseBranchPackage("   "); err == nil {
		t.Fatal("empty payload must be rejected")
	}
	huge := strings.Repeat("x", maxPackageBytes+1)
	if _, err := ParseBranchPackage(huge); err == nil {
		t.Fatal("oversized payload must be rejected")
	}
}
