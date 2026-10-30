package story

import (
	"strings"
	"testing"
)

func TestParseAnchorCheck(t *testing.T) {
	cases := []struct {
		raw  string
		want bool
	}{
		{"yes", true},
		{"Yes.", true},
		{"是", true},
		{"是的，发生了", true},
		{"no", false},
		{"No", false},
		{"不是", false},
		{"没有发生", false},
		{"否", false},
		{"可能吧", false},
	}
	for _, c := range cases {
		got, err := ParseAnchorCheck(c.raw)
		if err != nil {
			t.Fatalf("%q: %v", c.raw, err)
		}
		if got != c.want {
			t.Fatalf("%q = %v, want %v", c.raw, got, c.want)
		}
	}
	if _, err := ParseAnchorCheck("   "); err == nil {
		t.Fatal("empty reply must be an error")
	}
}

func TestBuildAnchorCheckPrompt(t *testing.T) {
	system, user := BuildAnchorCheckPrompt(
		CanonAnchor{Summary: "宝玉摔玉"},
		"他摘下通灵宝玉，狠狠摔在地上。",
	)
	if !strings.Contains(system, "yes") {
		t.Fatalf("system = %q", system)
	}
	for _, want := range []string{"宝玉摔玉", "通灵宝玉"} {
		if !strings.Contains(user, want) {
			t.Fatalf("prompt missing %q", want)
		}
	}
}
