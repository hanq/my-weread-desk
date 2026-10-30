package weread

import "testing"

func TestNormalizeVID(t *testing.T) {
	cases := map[string]string{
		"58328018":      "58328018",
		" 58328018 ":    "58328018",
		"5.8328018e+07": "58328018",
		"58328018.0":    "58328018",
		"\"58328018\"":  "58328018",
		"":              "",
		"not-a-number":  "not-a-number",
		"1.2345e+08":    "123450000",
		"1234567890123": "1234567890123",
	}
	for in, want := range cases {
		if got := NormalizeVID(in); got != want {
			t.Errorf("NormalizeVID(%q) = %q, want %q", in, got, want)
		}
	}
}
