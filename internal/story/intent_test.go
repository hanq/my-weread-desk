package story

import (
	"strings"
	"testing"
)

func TestPrecheckIntentCatchesBannedWords(t *testing.T) {
	m := NewMemory(BookRef{Title: "测试"})
	m.Banned = []string{"穿越"}
	vs := PrecheckIntent("让他穿越回现代", m, NewState())
	if len(vs) == 0 || !HasHardViolation(vs) {
		t.Fatalf("banned word not caught: %+v", vs)
	}
}

func TestPrecheckIntentCatchesRevival(t *testing.T) {
	m := NewMemory(BookRef{Title: "测试"})
	st := NewState()
	st.Dead = []string{"父亲"}

	vs := PrecheckIntent("让父亲复活", m, st)
	if len(vs) == 0 || !HasHardViolation(vs) {
		t.Fatalf("revival not caught: %+v", vs)
	}
	if vs[0].Rule != "死者不可复活" {
		t.Fatalf("rule = %q", vs[0].Rule)
	}

	if vs := PrecheckIntent("让黛玉主动表白", m, st); len(vs) != 0 {
		t.Fatalf("valid intent flagged: %+v", vs)
	}
}

func TestAlternativesForRevival(t *testing.T) {
	alts := AlternativesFor([]Violation{{Rule: "死者不可复活", Evidence: "复活"}})
	if len(alts) < 2 {
		t.Fatalf("alternatives = %v", alts)
	}
	joined := strings.Join(alts, " ")
	if !strings.Contains(joined, "记忆") {
		t.Fatalf("expected a concrete alternative, got %v", alts)
	}
}

func TestBuildIntentPromptIncludesConstraints(t *testing.T) {
	m := NewMemory(BookRef{Title: "测试"})
	m.Banned = []string{"穿越"}
	st := NewState()
	st.Dead = []string{"父亲"}
	st.Assumptions = []string{"真相未揭露"}

	system, user := BuildIntentPrompt(m, st, "让父亲复活")
	both := system + user
	for _, want := range []string{"让父亲复活", "不得改写已发生事件的结果", "穿越", "父亲", "真相未揭露", "feasibility"} {
		if !strings.Contains(both, want) {
			t.Fatalf("prompt missing %q", want)
		}
	}
}

func TestParseIntent(t *testing.T) {
	raw := "```json\n" +
		`{"feasibility":"violates","reason":"死者不可复活","blockedBy":["死者不可复活"],"alternatives":["改为失忆","放弃"],"statePatch":{}}` +
		"\n```"
	got, err := ParseIntent(raw)
	if err != nil {
		t.Fatalf("ParseIntent: %v", err)
	}
	if got.Feasibility != IntentViolates || len(got.Alternatives) != 2 {
		t.Fatalf("intent = %+v", got)
	}
}

func TestParseIntentOKRequiresGoal(t *testing.T) {
	if _, err := ParseIntent(`{"feasibility":"ok","goal":"让黛玉先开口"}`); err != nil {
		t.Fatalf("valid intent rejected: %v", err)
	}
	if _, err := ParseIntent(`{"feasibility":"ok"}`); err == nil {
		t.Fatal("missing goal must be rejected")
	}
}

func TestParseIntentDefaultsUnknownFeasibility(t *testing.T) {
	got, err := ParseIntent(`{"feasibility":"maybe","goal":"让他离开"}`)
	if err != nil {
		t.Fatalf("ParseIntent: %v", err)
	}
	if got.Feasibility != IntentOK {
		t.Fatalf("feasibility = %q", got.Feasibility)
	}
}
