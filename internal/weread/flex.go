package weread

import (
	"bytes"
	"encoding/json"
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
