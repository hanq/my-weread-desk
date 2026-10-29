package protocol

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestEVectors(t *testing.T) {
	var golden struct {
		E map[string]string `json:"e"`
	}
	readJSON(t, "golden_hash_vectors.json", &golden)
	for in, want := range golden.E {
		if got := E(in); got != want {
			t.Errorf("E(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSignVectors(t *testing.T) {
	var golden struct {
		Sign map[string]string `json:"sign"`
	}
	readJSON(t, "golden_hash_vectors.json", &golden)
	for in, want := range golden.Sign {
		if got := Sign(in); got != want {
			t.Errorf("Sign(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestWebAppID(t *testing.T) {
	var golden struct {
		AppID struct {
			UA    string `json:"ua"`
			Value string `json:"value"`
		} `json:"appid"`
	}
	readJSON(t, "golden_hash_vectors.json", &golden)
	if got := WebAppID(golden.AppID.UA); got != golden.AppID.Value {
		t.Errorf("WebAppID() = %q, want %q", got, golden.AppID.Value)
	}
}

func TestEscapeMatchesEncodeURIComponent(t *testing.T) {
	cases := map[string]string{
		"abc-_.~":    "abc-_.~",
		"中文":         "%E4%B8%AD%E6%96%87",
		"a b":        "a%20b",
		"a&b=c":      "a%26b%3Dc",
		"<p>":        "%3Cp%3E",
		"false":      "false",
		"0":          "0",
		"100%":       "100%25",
		"emoji😀":     "emoji%F0%9F%98%80",
		`back\slash`: "back%5Cslash",
	}
	for in, want := range cases {
		if got := Escape(in); got != want {
			t.Errorf("Escape(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSortedQueryExcludesSignature(t *testing.T) {
	params := map[string]any{
		"b": "x", "a": 1, "prevChapter": false, "s": "should-be-ignored",
	}
	want := "a=1&b=x&prevChapter=false"
	if got := SortedQuery(params); got != want {
		t.Fatalf("SortedQuery() = %q, want %q", got, want)
	}
}

// TestDecodePayloadMatchesReference compares DecodePayload against the Python
// reference implementation for a corpus of synthetic payloads. The golden file
// was produced by scripts/reference_decode.py from the fetched reference.
func TestDecodePayloadMatchesReference(t *testing.T) {
	var cases []struct {
		In  string `json:"in"`
		Out string `json:"out"`
	}
	readJSON(t, "golden_shard_cases.json", &cases)
	if len(cases) == 0 {
		t.Fatal("no golden cases")
	}
	for _, c := range cases {
		if got := DecodePayload(c.In); got != c.Out {
			t.Errorf("DecodePayload(%q)\n got %q\nwant %q", c.In, got, c.Out)
		}
	}
}

func TestDecodeShardsVerifiesChecksum(t *testing.T) {
	body := "abc"
	good := MD5Hex(body)[:32] + body
	// MD5 hex is 32 chars already.
	if _, err := DecodeShards(good, "", ""); err != nil {
		t.Fatalf("expected checksum to pass, got %v", err)
	}
	bad := "00000000000000000000000000000000" + body
	if _, err := DecodeShards(bad, "", ""); err == nil {
		t.Fatal("expected checksum error")
	}
}

func TestTruncateRunes(t *testing.T) {
	if got := TruncateRunes("第一章风起云涌的时候", 3); got != "第一章" {
		t.Errorf("TruncateRunes = %q", got)
	}
}

func TestReadPayloadShape(t *testing.T) {
	p := ReadPayload(ReadReport{
		BookID: "43208843", ChapterUID: 44, ChapterIndex: 2, ChapterOffset: 1543,
		Progress: 12, Summary: "这是一段用于校验的摘要文本，超过二十个字应当被截断处理",
		PSVTS: "abc", Token: "tok",
		ElapsedSeconds: 30,
	})
	required := []string{"appId", "b", "c", "ci", "co", "sm", "pr", "rt", "ts", "rn", "sg", "ct", "ps", "pc", "s"}
	for _, k := range required {
		if _, ok := p[k]; !ok {
			t.Errorf("missing field %q", k)
		}
	}
	if p["b"].(string) != E("43208843") {
		t.Errorf("b = %v", p["b"])
	}
	if n := len([]rune(p["sm"].(string))); n > 20 {
		t.Errorf("sm has %d runes", n)
	}
	if p["s"] != Sign(SortedQuery(p)) {
		t.Error("s is not a valid signature of the payload")
	}
}

func TestContentParamsStyleShardUsesSt1(t *testing.T) {
	plain := ContentParams("43208843", "44", "ps", false, 1)
	if plain["st"] != 0 {
		t.Errorf("content shard st = %v, want 0", plain["st"])
	}
	style := ContentParams("43208843", "44", "ps", true, 1)
	if style["st"] != 1 {
		t.Errorf("style shard st = %v, want 1", style["st"])
	}
	if style["s"] != Sign(SortedQuery(style)) {
		t.Error("signature mismatch")
	}
	if style["prevChapter"] != false {
		t.Error("prevChapter must encode as boolean false")
	}
}

func readJSON(t *testing.T, name string, dst any) {
	t.Helper()
	path := filepath.Join("testdata", name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if err := json.Unmarshal(data, dst); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
}
