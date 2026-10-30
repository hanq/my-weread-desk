package story

import (
	"strings"
	"testing"
)

func TestBuildSceneGenPromptIncludesConstitution(t *testing.T) {
	m := NewMemory(BookRef{Title: "红楼梦"})
	if err := m.AddUserRule("黛玉更主动"); err != nil {
		t.Fatalf("AddUserRule: %v", err)
	}
	st := NewState()
	st.Assumptions = []string{"真相未被揭露"}
	st.Dead = []string{"贾敏"}

	system, user := BuildSceneGenPrompt(SceneGenInput{
		BookTitle:  "红楼梦",
		CanonTitle: "第三回",
		CanonText:  "黛玉初进荣国府。",
		Memory:     m,
		State:      st,
		Pressure:   &Pressure{Summary: "宝玉摔玉", Level: 0.7, Means: "environment"},
		Intent:     "让黛玉先开口",
		Previous:   []string{"上一幕她沉默不语。"},
		Violations: []Violation{{Rule: "视角：第一人称", Evidence: "全文没有我"}},
	})

	both := system + user
	for _, want := range []string{
		"红楼梦",
		"不得改写已发生事件的结果", // hard rule from NewMemory
		"黛玉更主动",        // user rule
		"真相未被揭露",       // assumption
		"贾敏",           // dead character ban
		"宝玉摔玉",         // anchor pressure
		"环境事件",         // means hint
		"让黛玉先开口",       // free-form intent
		"上一幕她沉默不语",     // continuity
		"视角：第一人称",      // violation fed back
	} {
		if !strings.Contains(both, want) {
			t.Fatalf("prompt missing %q\n---\n%s", want, both)
		}
	}
	if !strings.Contains(system, "只输出正文") {
		t.Fatal("system prompt must forbid meta output")
	}
}

func TestBuildSceneGenPromptWithoutMemory(t *testing.T) {
	system, user := BuildSceneGenPrompt(SceneGenInput{BookTitle: "无名书"})
	if !strings.Contains(system, "无名书") {
		t.Fatalf("system = %q", system)
	}
	if !strings.Contains(user, "请写出本幕") {
		t.Fatalf("user = %q", user)
	}
}

func TestMeansHintCoversTheLadder(t *testing.T) {
	cases := map[string]string{
		"environment": "环境",
		"third_party": "第三方",
		"info_leak":   "秘密",
		"resource":    "资源",
		"motivation":  "决定",
	}
	for means, want := range cases {
		if hint := MeansHint(means); !strings.Contains(hint, want) {
			t.Fatalf("%s hint = %q", means, hint)
		}
	}
	if MeansHint("unknown") != "" {
		t.Fatal("unknown means should add no hint")
	}
}

func TestEstimateRejoin(t *testing.T) {
	cases := []struct {
		fork, max, span, want int
	}{
		{10, 20, 4, 14},
		{18, 20, 4, 20},
		{1, 20, 0, 5},
		{30, 20, 4, 30},
	}
	for _, c := range cases {
		if got := EstimateRejoin(c.fork, c.max, c.span); got != c.want {
			t.Fatalf("EstimateRejoin(%d,%d,%d) = %d, want %d",
				c.fork, c.max, c.span, got, c.want)
		}
	}
}

func TestBuildBranchSummaryPromptListsScenes(t *testing.T) {
	system, user := BuildBranchSummaryPrompt("红楼梦", []BranchScene{
		{Seq: 0, Summary: "黛玉进府"},
		{Seq: 1, Summary: "宝玉摔玉"},
	})
	if !strings.Contains(system, "150 字") {
		t.Fatalf("system = %q", system)
	}
	for _, want := range []string{"红楼梦", "黛玉进府", "宝玉摔玉"} {
		if !strings.Contains(user, want) {
			t.Fatalf("summary prompt missing %q", want)
		}
	}
}
