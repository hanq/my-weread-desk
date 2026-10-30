package story

import (
	"errors"
	"strings"
	"testing"
)

func newMemory(t *testing.T) *Memory {
	t.Helper()
	m := NewMemory(BookRef{Title: "红楼梦", Author: "曹雪芹", EditionHash: "sha256:abc"})
	if err := m.Validate(); err != nil {
		t.Fatalf("NewMemory should be valid: %v", err)
	}
	return m
}

func TestMemoryRoundTrip(t *testing.T) {
	m := newMemory(t)
	m.Banned = []string{"复活", "穿越"}
	m.Mode = "free"
	m.Gravity = 0.3
	m.Engine = EngineZone{
		SatisfiedAnchors: []string{"anchor-1"},
		Divergence:       0.25,
		UpdatedAt:        1700000000,
		Summary:          "黛玉在第三幕主动表态",
	}

	raw, err := m.Render()
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	parsed, err := ParseMemory(raw)
	if err != nil {
		t.Fatalf("ParseMemory: %v", err)
	}

	if parsed.Book.Title != "红楼梦" || parsed.Book.EditionHash != "sha256:abc" {
		t.Fatalf("book lost: %+v", parsed.Book)
	}
	if parsed.Mode != "free" || parsed.Gravity != 0.3 {
		t.Fatalf("front matter lost: mode=%s gravity=%v", parsed.Mode, parsed.Gravity)
	}
	if len(parsed.Banned) != 2 || parsed.Banned[1] != "穿越" {
		t.Fatalf("banned lost: %+v", parsed.Banned)
	}
	if parsed.Engine.Divergence != 0.25 || len(parsed.Engine.SatisfiedAnchors) != 1 {
		t.Fatalf("engine zone lost: %+v", parsed.Engine)
	}
	if len(parsed.HardRules) != len(m.HardRules) {
		t.Fatalf("hard rules lost: %d vs %d", len(parsed.HardRules), len(m.HardRules))
	}
}

func TestMemoryDetectsEngineTamper(t *testing.T) {
	m := newMemory(t)
	m.Engine = EngineZone{Divergence: 0.25}
	raw, err := m.Render()
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	// A value edit must be treated as tampering: the engine zone is derived
	// state, and a hand-edited divergence would silently weaken the pull-back.
	tampered := strings.Replace(raw, "divergence: 0.2500", "divergence: 0.9000", 1)
	if tampered == raw {
		t.Fatal("test fixture did not contain the divergence line")
	}
	if _, err := ParseMemory(tampered); !errors.Is(err, ErrTampered) {
		t.Fatalf("expected ErrTampered, got %v", err)
	}

	// Whitespace-only edits must still parse: the hash covers canonical values,
	// not formatting.
	reformatted := strings.Replace(raw, "divergence: 0.2500", "divergence:   0.2500", 1)
	if _, err := ParseMemory(reformatted); err != nil {
		t.Fatalf("whitespace edit should be tolerated: %v", err)
	}
}

func TestMemoryRejectsDirectRuleMutation(t *testing.T) {
	m := newMemory(t)
	// Mutating the parsed slice cannot change the USER block, so failing loudly
	// is better than writing a file that drops the rule.
	m.UserRules = append(m.UserRules, "我希望黛玉更主动")
	if _, err := m.Render(); !errors.Is(err, ErrUserBodyDrift) {
		t.Fatalf("expected ErrUserBodyDrift, got %v", err)
	}
}

func TestMemorySetUserBody(t *testing.T) {
	m := newMemory(t)
	body := "## 强制规则\n- 不得改写已发生事件的结果\n\n## 用户规则\n- 黛玉更主动\n- 不要写诗词\n"
	if err := m.SetUserBody(body); err != nil {
		t.Fatalf("SetUserBody: %v", err)
	}
	if len(m.UserRules) != 2 {
		t.Fatalf("rules not parsed: %+v", m.UserRules)
	}

	raw, err := m.Render()
	if err != nil {
		t.Fatalf("Render after SetUserBody: %v", err)
	}
	parsed, err := ParseMemory(raw)
	if err != nil {
		t.Fatalf("ParseMemory: %v", err)
	}
	if len(parsed.UserRules) != 2 || parsed.UserRules[0] != "黛玉更主动" {
		t.Fatalf("user rules lost: %+v", parsed.UserRules)
	}
}

func TestParseRejectsUnknownFrontMatterKey(t *testing.T) {
	m := newMemory(t)
	raw, err := m.Render()
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	bad := strings.Replace(raw, "mode: balance", "unknownKey: 1\nmode: balance", 1)
	if _, err := ParseMemory(bad); err == nil {
		t.Fatal("unknown front-matter key must be rejected")
	}
}

func TestParseRejectsMissingUserBlock(t *testing.T) {
	m := newMemory(t)
	raw, _ := m.Render()
	bad := strings.Replace(raw, userBegin, "<!-- gone -->", 1)
	if _, err := ParseMemory(bad); err == nil {
		t.Fatal("missing USER block must be rejected")
	}
}

func TestValidateSceneBanned(t *testing.T) {
	m := newMemory(t)
	m.Banned = []string{"穿越"}
	vs := ValidateScene("主角在雨里突然穿越回了现代。", m, NewState())
	if !HasHardViolation(vs) {
		t.Fatalf("banned term should be hard: %+v", vs)
	}
}

func TestValidateSceneLength(t *testing.T) {
	m := newMemory(t)
	vs := ValidateScene("太短。", m, nil)
	found := false
	for _, v := range vs {
		if v.Rule == "篇幅过短" && v.Severity == SeveritySoft {
			found = true
		}
	}
	if !found {
		t.Fatalf("short scene should warn: %+v", vs)
	}
	if HasHardViolation(vs) {
		t.Fatalf("length must never be hard: %+v", vs)
	}
}

func TestValidateScenePOV(t *testing.T) {
	m := newMemory(t)
	m.POV = "first"
	vs := ValidateScene("他推开门，屋里没有人，雨声盖过了一切。", m, nil)
	if !HasHardViolation(vs) {
		t.Fatalf("first person without 我 should be hard: %+v", vs)
	}
}

func TestValidateSceneDeadCharacter(t *testing.T) {
	m := newMemory(t)
	st := NewState()
	st.Apply(StatePatch{Kill: []string{"黛玉"}})

	vs := ValidateScene("黛玉竟然推门走了进来。", m, st)
	if HasHardViolation(vs) {
		t.Fatalf("a mention of the dead is only suspicious: %+v", vs)
	}
	if len(vs) == 0 {
		t.Fatal("mentioning a dead character should warn")
	}

	vs = ValidateScene("黛玉竟然复活了，推门走了进来。", m, st)
	if !HasHardViolation(vs) {
		t.Fatalf("revival must be hard: %+v", vs)
	}
}

func TestStateApplyIsIdempotent(t *testing.T) {
	st := NewState()
	patch := StatePatch{
		AddAssumptions: []string{"真相未被揭露"},
		Kill:           []string{"父亲"},
		Learn:          map[string][]string{"宝玉": {"黛玉已死"}},
		SetFacts:       map[string]string{"地点": "荣国府"},
	}
	st.Apply(patch)
	st.Apply(patch)

	if len(st.Assumptions) != 1 {
		t.Fatalf("assumptions duplicated: %+v", st.Assumptions)
	}
	if len(st.Dead) != 1 || !st.IsDead("父亲") {
		t.Fatalf("dead ledger wrong: %+v", st.Dead)
	}
	if !st.KnowsFact("宝玉", "黛玉已死") {
		t.Fatalf("knows ledger wrong: %+v", st.Knows)
	}
	if st.Facts["地点"] != "荣国府" {
		t.Fatalf("facts ledger wrong: %+v", st.Facts)
	}
}

func TestStateCloneIsDeep(t *testing.T) {
	st := NewState()
	st.Apply(StatePatch{Kill: []string{"父亲"}})
	clone := st.Clone()
	clone.Apply(StatePatch{Kill: []string{"宝玉"}})

	if st.IsDead("宝玉") {
		t.Fatal("clone leaked a death into the original")
	}
	if !clone.IsDead("宝玉") {
		t.Fatal("clone did not record its own change")
	}
}
