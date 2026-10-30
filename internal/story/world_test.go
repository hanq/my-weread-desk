package story

import (
	"strings"
	"testing"
)

func sampleSheet() *WorldSheet {
	return &WorldSheet{
		Stats: []WorldStat{
			{Key: "月钱", Label: "月钱", Kind: StatResource, Min: 0, Max: 10, Initial: 2, Decay: 1},
			{Key: "体面", Label: "体面", Kind: StatReputation, Min: 0, Max: 100, Initial: 70},
		},
		Items: []WorldItem{
			{ID: "jade", Name: "通灵宝玉", LimitPerBranch: 1},
			{ID: "letter", Name: "书信", LimitPerBranch: 2},
		},
	}
}

func TestApplyWorldClampsStats(t *testing.T) {
	st := NewState()
	sheet := sampleSheet()
	sheet.InitState(st)

	vs := ApplyWorld(sheet, st, StatePatch{StatDelta: map[string]float64{
		"月钱": 100, "体面": -1000, "不存在": 5,
	}})
	if HasHardViolation(vs) {
		t.Fatalf("stat overflow must not be hard: %+v", vs)
	}
	if st.Stats["月钱"] != 10 {
		t.Fatalf("月钱 = %v, want 10", st.Stats["月钱"])
	}
	if st.Stats["体面"] != 0 {
		t.Fatalf("体面 = %v, want 0", st.Stats["体面"])
	}
	found := false
	for _, v := range vs {
		if v.Rule == "未知属性" && v.Severity == SeveritySoft {
			found = true
		}
	}
	if !found {
		t.Fatalf("unknown stat should warn: %+v", vs)
	}
}

func TestApplyWorldUnknownItemIsAtomic(t *testing.T) {
	st := NewState()
	sheet := sampleSheet()
	sheet.InitState(st)
	before := st.Stats["月钱"]

	vs := ApplyWorld(sheet, st, StatePatch{
		StatDelta: map[string]float64{"月钱": 5},
		ItemDelta: map[string]int{"ghost": 1},
	})
	if !HasHardViolation(vs) {
		t.Fatalf("unknown item must be hard: %+v", vs)
	}
	if st.Stats["月钱"] != before {
		t.Fatalf("stats were applied despite the item failure: %v", st.Stats["月钱"])
	}
}

func TestApplyWorldItemLimits(t *testing.T) {
	st := NewState()
	sheet := sampleSheet()
	sheet.InitState(st)

	if vs := ApplyWorld(sheet, st, StatePatch{ItemDelta: map[string]int{"jade": 2}}); !HasHardViolation(vs) {
		t.Fatalf("exceeding per-branch limit must be hard: %+v", vs)
	}
	if vs := ApplyWorld(sheet, st, StatePatch{ItemDelta: map[string]int{"letter": 2}}); HasHardViolation(vs) {
		t.Fatalf("within limit should pass: %+v", vs)
	}
	if st.Inventory["letter"] != 2 {
		t.Fatalf("letter = %d", st.Inventory["letter"])
	}
	if vs := ApplyWorld(sheet, st, StatePatch{ItemDelta: map[string]int{"letter": 1}}); !HasHardViolation(vs) {
		t.Fatalf("exceeding limit after adding must be hard: %+v", vs)
	}
}

func TestApplyWorldRejectsNegativeBalance(t *testing.T) {
	st := NewState()
	sheet := sampleSheet()
	sheet.InitState(st)

	vs := ApplyWorld(sheet, st, StatePatch{ItemDelta: map[string]int{"letter": -1}})
	if !HasHardViolation(vs) {
		t.Fatalf("negative balance must be hard: %+v", vs)
	}
}

func TestApplyWorldEnforcesSlotCap(t *testing.T) {
	st := NewState()
	sheet := &WorldSheet{}
	for i := 0; i < MaxInventorySlots+1; i++ {
		sheet.Items = append(sheet.Items, WorldItem{
			ID: idAt(i), Name: nameAt(i), LimitPerBranch: 1,
		})
	}
	for i := 0; i < MaxInventorySlots; i++ {
		if vs := ApplyWorld(sheet, st, StatePatch{ItemDelta: map[string]int{idAt(i): 1}}); HasHardViolation(vs) {
			t.Fatalf("slot %d should fit: %+v", i, vs)
		}
	}
	vs := ApplyWorld(sheet, st, StatePatch{ItemDelta: map[string]int{idAt(MaxInventorySlots): 1}})
	if !HasHardViolation(vs) {
		t.Fatalf("slot cap not enforced: %+v", vs)
	}
}

func idAt(i int) string   { return "item-" + itoa(i) }
func nameAt(i int) string { return "物件" + itoa(i) }

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [8]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

func TestTickDecaysAndClamps(t *testing.T) {
	st := NewState()
	sheet := sampleSheet()
	sheet.InitState(st)

	sheet.Tick(st)
	if st.Stats["月钱"] != 1 {
		t.Fatalf("月钱 after one tick = %v", st.Stats["月钱"])
	}
	sheet.Tick(st)
	sheet.Tick(st)
	if st.Stats["月钱"] != 0 {
		t.Fatalf("decay must clamp at min, got %v", st.Stats["月钱"])
	}
}

func TestInitStateSeedsInitialValues(t *testing.T) {
	st := NewState()
	st.Stats["月钱"] = 9
	sampleSheet().InitState(st)
	if st.Stats["月钱"] != 9 {
		t.Fatalf("existing value overwritten: %v", st.Stats["月钱"])
	}
	if st.Stats["体面"] != 70 {
		t.Fatalf("initial not seeded: %v", st.Stats["体面"])
	}
}

func TestParseWorldFieldsCaps(t *testing.T) {
	stats := make([]string, 0, 8)
	for i := 0; i < 8; i++ {
		stats = append(stats, `{"key":"s`+itoa(i)+`","label":"S","min":0,"max":10,"initial":1}`)
	}
	raw := `{"stats":[` + strings.Join(stats, ",") + `],"items":[{"name":"玉"},{"name":"玉"},{"name":"书信"}]}`
	got, err := ParseWorldFields(raw)
	if err != nil {
		t.Fatalf("ParseWorldFields: %v", err)
	}
	if len(got.Stats) != MaxWorldStats {
		t.Fatalf("stats = %d, want %d", len(got.Stats), MaxWorldStats)
	}
	if len(got.Items) != 2 {
		t.Fatalf("duplicate items not removed: %+v", got.Items)
	}
}

func TestParseWorldFieldsRejectsEmpty(t *testing.T) {
	if _, err := ParseWorldFields(`{"stats":[],"items":[]}`); err == nil {
		t.Fatal("empty sheet must be rejected")
	}
	if _, err := ParseWorldFields("没有世界表"); err == nil {
		t.Fatal("prose must be rejected")
	}
}

func TestBuildWorldPromptConstrainsDesign(t *testing.T) {
	system, user := BuildWorldPrompt("红楼梦", []Scene{{Seq: 0, Summary: "黛玉进府"}})
	if !strings.Contains(system, "JSON") {
		t.Fatalf("system = %q", system)
	}
	for _, want := range []string{"红楼梦", "黛玉进府", "3 到 6 项", "不要设计等级"} {
		if !strings.Contains(user, want) {
			t.Fatalf("prompt missing %q", want)
		}
	}
}
