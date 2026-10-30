package story

import "testing"

func TestValidatePatchAcceptsReasonablePatch(t *testing.T) {
	st := NewState()
	p := StatePatch{
		AddAssumptions: []string{"真相未被揭露"},
		Kill:           []string{"父亲"},
		SetFacts:       map[string]string{"地点": "荣国府"},
		Learn:          map[string][]string{"宝玉": {"黛玉已死"}},
	}
	if vs := ValidatePatch(p, st); HasHardViolation(vs) {
		t.Fatalf("valid patch rejected: %+v", vs)
	}
}

func TestValidatePatchRejectsOversized(t *testing.T) {
	cases := []StatePatch{
		{AddAssumptions: []string{"a", "b", "c", "d", "e"}},
		{Kill: []string{"a", "b", "c", "d"}},
		{SetFacts: map[string]string{
			"1": "a", "2": "b", "3": "c", "4": "d",
			"5": "e", "6": "f", "7": "g", "8": "h", "9": "i",
		}},
	}
	for i, p := range cases {
		if vs := ValidatePatch(p, NewState()); !HasHardViolation(vs) {
			t.Fatalf("case %d not rejected: %+v", i, vs)
		}
	}
}

func TestValidatePatchRejectsEmptyEntries(t *testing.T) {
	cases := []StatePatch{
		{AddAssumptions: []string{"  "}},
		{Kill: []string{""}},
		{SetFacts: map[string]string{"": "x"}},
		{SetItems: map[string]string{"": "x"}},
		{Learn: map[string][]string{"宝玉": {""}}},
		{Learn: map[string][]string{"  ": {"x"}}},
	}
	for i, p := range cases {
		if vs := ValidatePatch(p, NewState()); !HasHardViolation(vs) {
			t.Fatalf("case %d not rejected: %+v", i, vs)
		}
	}
}

func TestValidatePatchFlagsDuplicateDeathAsSoft(t *testing.T) {
	st := NewState()
	st.Apply(StatePatch{Kill: []string{"父亲"}})

	vs := ValidatePatch(StatePatch{Kill: []string{"父亲"}}, st)
	if HasHardViolation(vs) {
		t.Fatalf("duplicate death must not be hard: %+v", vs)
	}
	found := false
	for _, v := range vs {
		if v.Rule == "重复死亡" && v.Severity == SeveritySoft {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a soft duplicate flag: %+v", vs)
	}
}
