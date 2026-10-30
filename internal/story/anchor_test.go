package story

import (
	"math"
	"testing"
)

func approx(a, b float64) bool { return math.Abs(a-b) < 1e-6 }

func TestGravityIgnoresDistantAnchors(t *testing.T) {
	anchors := []CanonAnchor{{ID: "a", SceneSeq: 30, DeadlineSeq: 30, Weight: 0.8}}
	if g := Gravity(10, anchors, nil); g != 0 {
		t.Fatalf("a distant anchor must not pull, got %v", g)
	}
}

func TestGravityRampsUpBeforeDeadline(t *testing.T) {
	anchors := []CanonAnchor{{ID: "a", SceneSeq: 10, DeadlineSeq: 10, Weight: 0.8}}
	cases := []struct {
		seq  int
		want float64
	}{
		{5, 0},          // due in 5 → outside the ramp window
		{6, 0.35 * 0.8}, // due in 4
		{8, 0.7 * 0.8},  // due in 2
		{10, 1.0 * 0.8}, // due now
		{14, 1.0 * 0.8}, // overdue caps at full weight
	}
	for _, c := range cases {
		if got := Gravity(c.seq, anchors, nil); !approx(got, c.want) {
			t.Fatalf("seq %d: gravity = %v, want %v", c.seq, got, c.want)
		}
	}
}

func TestGravitySkipsSatisfiedAnchors(t *testing.T) {
	anchors := []CanonAnchor{{ID: "a", SceneSeq: 10, DeadlineSeq: 10, Weight: 1}}
	if g := Gravity(10, anchors, Satisfaction{"a": true}); g != 0 {
		t.Fatalf("satisfied anchor still pulls: %v", g)
	}
}

func TestDivergenceCountsOverdueAnchors(t *testing.T) {
	anchors := []CanonAnchor{
		{ID: "past", DeadlineSeq: 5},
		{ID: "future", DeadlineSeq: 50},
	}
	// One overdue and unsatisfied → coverage 1 → 0.45.
	if got := Divergence(10, anchors, nil, 0, 0); !approx(got, 0.45) {
		t.Fatalf("divergence = %v, want 0.45", got)
	}
	// Satisfying it removes the coverage term.
	sat := Satisfaction{"past": true}
	if got := Divergence(10, anchors, sat, 0, 0); !approx(got, 0) {
		t.Fatalf("divergence = %v, want 0", got)
	}
	// Nothing due yet means no coverage penalty either.
	if got := Divergence(1, anchors, nil, 0, 0); !approx(got, 0) {
		t.Fatalf("divergence before any deadline = %v", got)
	}
}

func TestDivergenceWeightsOtherLedgers(t *testing.T) {
	got := Divergence(0, nil, nil, 1, 1)
	if !approx(got, 0.55) {
		t.Fatalf("relation+guns divergence = %v, want 0.55", got)
	}
}

func TestPlanPressurePicksTheEarliestDeadline(t *testing.T) {
	anchors := []CanonAnchor{
		{ID: "later", DeadlineSeq: 20, Weight: 0.8, Summary: "later"},
		{ID: "sooner", DeadlineSeq: 11, Weight: 0.8, Summary: "sooner"},
	}
	p := PlanPressure(10, anchors, nil, 0)
	if p == nil {
		t.Fatal("expected a pressure plan")
	}
	if p.AnchorID != "sooner" {
		t.Fatalf("picked %s, want sooner", p.AnchorID)
	}
	if p.DueIn != 1 {
		t.Fatalf("dueIn = %d", p.DueIn)
	}
}

func TestPlanPressureReturnsNilWhenNothingPending(t *testing.T) {
	anchors := []CanonAnchor{{ID: "a", DeadlineSeq: 10, Weight: 1}}
	if p := PlanPressure(10, anchors, Satisfaction{"a": true}, 0); p != nil {
		t.Fatalf("expected nil, got %+v", p)
	}
}

func TestMeansLadderEscalates(t *testing.T) {
	cases := []struct {
		level float64
		want  string
	}{
		{0.1, "environment"},
		{0.4, "third_party"},
		{0.6, "info_leak"},
		{0.8, "resource"},
		{1.0, "motivation"},
	}
	for _, c := range cases {
		if got := meansFor(c.level); got != c.want {
			t.Fatalf("level %v: means = %q, want %q", c.level, got, c.want)
		}
	}
}

func TestParseAnchors(t *testing.T) {
	raw := "```json\n[" +
		"{\"sceneSeq\":3,\"kind\":\"reveal\",\"summary\":\"身世被揭穿\",\"weight\":0.9}," +
		"{\"sceneSeq\":7,\"summary\":\"两人决裂\",\"weight\":2}," +
		"{\"sceneSeq\":-1,\"summary\":\"无效\"}," +
		"{\"sceneSeq\":9,\"summary\":\"   \"}" +
		"]\n```"
	got, err := ParseAnchors(raw)
	if err != nil {
		t.Fatalf("ParseAnchors: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 usable anchors, got %d: %+v", len(got), got)
	}
	if got[0].Kind != AnchorReveal || got[0].Weight != 0.9 {
		t.Fatalf("first anchor = %+v", got[0])
	}
	// Missing kind defaults to beat and an out-of-range weight is clamped.
	if got[1].Kind != AnchorBeat || got[1].Weight != 0.8 {
		t.Fatalf("second anchor not normalised: %+v", got[1])
	}
}

func TestParseAnchorsRejectsNoise(t *testing.T) {
	if _, err := ParseAnchors("抱歉，我无法分析。"); err == nil {
		t.Fatal("prose must be rejected")
	}
	if _, err := ParseAnchors("[]"); err == nil {
		t.Fatal("empty array must be rejected")
	}
}
