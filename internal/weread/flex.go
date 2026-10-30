package weread

import (
	"bytes"
	"encoding/json"
	"math"
	"sort"
	"strconv"
	"strings"
)

// FlexString accepts a JSON string, number or bool.
type FlexString string

func (f *FlexString) UnmarshalJSON(raw []byte) error {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || string(raw) == "null" {
		*f = ""
		return nil
	}
	if raw[0] == '"' {
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return err
		}
		*f = FlexString(s)
		return nil
	}
	*f = FlexString(strings.Trim(string(raw), `"`))
	return nil
}

func (f FlexString) String() string { return string(f) }

// NormalizeVID repairs a user id that went through float64 formatting. JSON
// numbers decoded into any become float64, and fmt.Sprint then renders large
// ids as "5.8328018e+07". Sending that as the vid header makes the E-Ink API
// answer -2010 用户不存在. Stored credentials from older builds are repaired
// with the same helper.
func NormalizeVID(raw string) string {
	v := strings.TrimSpace(raw)
	v = strings.Trim(v, `"`)
	if v == "" {
		return ""
	}
	if !strings.ContainsAny(v, "eE.") {
		return v
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil || math.IsNaN(f) || math.IsInf(f, 0) || f <= 0 || f != math.Trunc(f) {
		return v
	}
	return strconv.FormatFloat(f, 'f', 0, 64)
}

// Int64 parses the value as an integer, tolerating float spellings.
func (f FlexString) Int64() int64 {
	v, err := strconv.ParseInt(string(f), 10, 64)
	if err == nil {
		return v
	}
	fl, err := strconv.ParseFloat(string(f), 64)
	if err != nil {
		return 0
	}
	return int64(fl)
}

// FlexBool accepts true/false, 0/1 and "0"/"1".
type FlexBool bool

func (f *FlexBool) UnmarshalJSON(raw []byte) error {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || string(raw) == "null" {
		*f = false
		return nil
	}
	switch raw[0] {
	case 't':
		*f = true
	case 'f':
		*f = false
	case '"':
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return err
		}
		*f = FlexBool(s == "1" || strings.EqualFold(s, "true"))
	default:
		var n float64
		if err := json.Unmarshal(raw, &n); err != nil {
			return err
		}
		*f = n != 0
	}
	return nil
}

func (f FlexBool) Bool() bool { return bool(f) }

// FlexInt accepts numbers and numeric strings.
type FlexInt int64

func (f *FlexInt) UnmarshalJSON(raw []byte) error {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || string(raw) == "null" {
		*f = 0
		return nil
	}
	if raw[0] == '"' {
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return err
		}
		*f = FlexInt(FlexString(s).Int64())
		return nil
	}
	var fl float64
	if err := json.Unmarshal(raw, &fl); err != nil {
		return err
	}
	*f = FlexInt(int64(fl))
	return nil
}

func (f FlexInt) Int64() int64 { return int64(f) }

// FlexFloat accepts numbers and numeric strings.
type FlexFloat float64

func (f *FlexFloat) UnmarshalJSON(raw []byte) error {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || string(raw) == "null" {
		*f = 0
		return nil
	}
	if raw[0] == '"' {
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return err
		}
		fl, _ := strconv.ParseFloat(s, 64)
		*f = FlexFloat(fl)
		return nil
	}
	var fl float64
	if err := json.Unmarshal(raw, &fl); err != nil {
		return err
	}
	*f = FlexFloat(fl)
	return nil
}

func (f FlexFloat) Float64() float64 { return float64(f) }

// FlexRank accepts a number, a numeric string, or an object carrying the rank.
// Some Web /readdata/detail responses wrap the value as
// {"rank":12,"total":100}; a strict int field would reject the whole stats
// payload with a JSON type error.
type FlexRank int64

func (f *FlexRank) UnmarshalJSON(raw []byte) error {
	*f = FlexRank(parseRank(raw, 0))
	return nil
}

func (f FlexRank) Int64() int64 { return int64(f) }

func parseRank(raw []byte, depth int) int64 {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || string(raw) == "null" || depth > 4 {
		return 0
	}
	switch raw[0] {
	case '"':
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return 0
		}
		if n := FlexString(s).Int64(); n != 0 {
			return n
		}
		return leadingInt(s)
	case '{':
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(raw, &obj); err != nil {
			return 0
		}
		for _, key := range []string{"rank", "rankValue", "value", "number", "num", "position", "place", "no", "index"} {
			if v, ok := obj[key]; ok {
				if n := parseRank(v, depth+1); n != 0 {
					return n
				}
			}
		}
		return 0
	case '[':
		var list []json.RawMessage
		if err := json.Unmarshal(raw, &list); err != nil || len(list) == 0 {
			return 0
		}
		return parseRank(list[0], depth+1)
	default:
		var n float64
		if err := json.Unmarshal(raw, &n); err == nil {
			return int64(n)
		}
		return 0
	}
}

// leadingInt extracts the first integer from labels such as "第 12 名".
func leadingInt(s string) int64 {
	start := -1
	for i, r := range s {
		if r >= '0' && r <= '9' {
			if start < 0 {
				start = i
			}
			continue
		}
		if start >= 0 {
			n, _ := strconv.ParseInt(s[start:i], 10, 64)
			return n
		}
	}
	if start >= 0 {
		n, _ := strconv.ParseInt(s[start:], 10, 64)
		return n
	}
	return 0
}

// IsMP reports whether the book id belongs to a WeChat official account.
func IsMP(bookID string) bool { return strings.HasPrefix(bookID, "MP_WXS_") }

// FirstString returns the first non-empty value.
func FirstString(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// FlexSeries is a numeric series that WeRead spells either as an array of
// numbers or as an object keyed by the bucket start. The object form is
// returned in ascending key order so a chart stays chronological; keys that are
// not numeric fall back to lexicographic order.
//
// /readdata/detail uses both spellings: readTimes is an object for every mode,
// while dailyReadTimes is an object only for the annual period. A strict slice
// field rejects the whole payload with a JSON type error.
type FlexSeries []FlexInt

func (f *FlexSeries) UnmarshalJSON(raw []byte) error {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || string(raw) == "null" {
		*f = nil
		return nil
	}
	switch raw[0] {
	case '[':
		var list []FlexInt
		if err := json.Unmarshal(raw, &list); err != nil {
			return err
		}
		*f = list
		return nil
	case '{':
		var obj map[string]FlexInt
		if err := json.Unmarshal(raw, &obj); err != nil {
			return err
		}
		keys := make([]string, 0, len(obj))
		for key := range obj {
			keys = append(keys, key)
		}
		sort.Slice(keys, func(i, j int) bool {
			a, aerr := strconv.ParseInt(keys[i], 10, 64)
			b, berr := strconv.ParseInt(keys[j], 10, 64)
			if aerr == nil && berr == nil {
				return a < b
			}
			return keys[i] < keys[j]
		})
		out := make([]FlexInt, 0, len(keys))
		for _, key := range keys {
			out = append(out, obj[key])
		}
		*f = out
		return nil
	default:
		var n FlexInt
		if err := json.Unmarshal(raw, &n); err != nil {
			return err
		}
		*f = []FlexInt{n}
		return nil
	}
}

// Int64s returns the series as plain integers, or nil when it is empty.
func (f FlexSeries) Int64s() []int64 {
	if len(f) == 0 {
		return nil
	}
	out := make([]int64, len(f))
	for i, v := range f {
		out[i] = v.Int64()
	}
	return out
}
