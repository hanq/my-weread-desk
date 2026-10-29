package protocol

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// ErrShardMD5 means a shard body did not match its leading MD5 checksum.
// This almost always means the session is not entitled to the chapter or the
// request signature is stale.
var ErrShardMD5 = errors.New("weread: shard checksum mismatch")

// DecodeShards decodes an EPUB chapter body from the e_0/e_1/e_3 shards.
func DecodeShards(e0, e1, e3 string) (string, error) {
	payload, err := checkedBody(e0)
	if err != nil {
		return "", err
	}
	mid, err := checkedBody(e1)
	if err != nil {
		return "", err
	}
	tail, err := checkedBody(e3)
	if err != nil {
		return "", err
	}
	return DecodePayload(payload + mid + tail), nil
}

// DecodeStyle decodes the e_2 stylesheet shard.
func DecodeStyle(e2 string) (string, error) {
	body, err := checkedBody(e2)
	if err != nil {
		return "", err
	}
	return DecodePayload(body), nil
}

// checkedBody verifies the 32-character MD5 prefix and returns the remainder.
func checkedBody(response string) (string, error) {
	if len(response) <= 32 {
		return "", nil
	}
	expected := response[:32]
	body := response[32:]
	if !strings.EqualFold(MD5Hex(body), expected) {
		return "", fmt.Errorf("%w: expected %s, got %s", ErrShardMD5, expected, strings.ToUpper(MD5Hex(body)))
	}
	return body, nil
}

// DecodePayload reverses WeRead's character swaps and base64-url encoding.
func DecodePayload(encoded string) string {
	if encoded == "" {
		return ""
	}
	if len(encoded) > 0 {
		encoded = encoded[1:]
	}
	reordered := reverseSwaps(encoded, swapPositions(encoded))

	var b64 strings.Builder
	for i := 0; i < len(reordered); i++ {
		c := reordered[i]
		switch {
		case c >= 'A' && c <= 'Z', c >= 'a' && c <= 'z', c >= '0' && c <= '9', c == '+', c == '/':
			b64.WriteByte(c)
		case c == '-':
			b64.WriteByte('+')
		case c == '_':
			b64.WriteByte('/')
		}
	}
	s := b64.String()
	if pad := len(s) % 4; pad != 0 {
		s += strings.Repeat("=", 4-pad)
	}
	raw, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		// Truncated or malformed payloads still yield readable prefixes.
		raw = decodeLoose(s)
	}
	return repairUTF8(raw)
}

// decodeLoose decodes as much of a std-base64 string as possible.
func decodeLoose(s string) []byte {
	out := make([]byte, 0, len(s)*3/4)
	var acc uint32
	var bits uint
	for i := 0; i < len(s); i++ {
		v, ok := b64Value(s[i])
		if !ok {
			continue
		}
		acc = acc<<6 | uint32(v)
		bits += 6
		if bits >= 8 {
			bits -= 8
			out = append(out, byte(acc>>bits))
		}
	}
	return out
}

func b64Value(c byte) (int, bool) {
	switch {
	case c >= 'A' && c <= 'Z':
		return int(c - 'A'), true
	case c >= 'a' && c <= 'z':
		return int(c-'a') + 26, true
	case c >= '0' && c <= '9':
		return int(c-'0') + 52, true
	case c == '+':
		return 62, true
	case c == '/':
		return 63, true
	}
	return 0, false
}

// swapPositions reproduces the index list used to undo the character swaps.
//
// The tail characters of the payload encode their own indices in a radix-4
// representation of the character's binary digits; that is not a typo and is
// required to match the server.
func swapPositions(encoded string) []int {
	length := len(encoded)
	if length < 4 {
		return nil
	}
	if length < 11 {
		return []int{0, 2}
	}

	n := (length + 9) / 10
	if n > 4 {
		n = 4
	}

	var tmp strings.Builder
	for i := length - 1; i >= length-n; i-- {
		tmp.WriteString(strconv.Itoa(radix4FromBinary(int(encoded[i]))))
	}
	t := tmp.String()

	m := length - n - 2
	if m <= 0 {
		return nil
	}
	step := len(strconv.Itoa(m))
	if step < 1 {
		step = 1
	}

	result := make([]int, 0, 10)
	for i := 0; len(result) < 10 && i+step < len(t); i += step {
		v1, ok := parseDigits(t[i : i+step])
		if !ok {
			break
		}
		result = append(result, v1%m)

		end := i + 1 + step
		if end > len(t) {
			break
		}
		v2, ok := parseDigits(t[i+1 : end])
		if !ok {
			break
		}
		result = append(result, v2%m)
	}
	return result
}

// radix4FromBinary parses the binary representation of v as base-4 digits.
func radix4FromBinary(v int) int {
	bin := strconv.FormatInt(int64(v), 2)
	out := 0
	for i := 0; i < len(bin); i++ {
		out = out*4 + int(bin[i]-'0')
	}
	return out
}

func parseDigits(s string) (int, bool) {
	v, err := strconv.Atoi(s)
	if err != nil {
		return 0, false
	}
	return v, true
}

func reverseSwaps(encoded string, positions []int) string {
	if len(positions) < 2 {
		return encoded
	}
	chars := []byte(encoded)
	for i := len(positions) - 1; i > 0; i -= 2 {
		for k := 1; k >= 0; k-- {
			left := positions[i] + k
			right := positions[i-1] + k
			if left < 0 || right < 0 || left >= len(chars) || right >= len(chars) {
				continue
			}
			chars[left], chars[right] = chars[right], chars[left]
		}
	}
	return string(chars)
}

// repairUTF8 decodes byte sequences that were previously flattened into one
// rune per byte, passing anything invalid through unchanged.
func repairUTF8(raw []byte) string {
	var out strings.Builder
	out.Grow(len(raw))
	for i := 0; i < len(raw); {
		b0 := raw[i]
		switch {
		case b0 >= 0xC0 && b0 <= 0xDF && i+1 < len(raw) && isCont(raw[i+1]):
			out.WriteRune(rune(uint32(b0&0x1F)<<6 | uint32(raw[i+1]&0x3F)))
			i += 2
		case b0 >= 0xE0 && b0 <= 0xEF && i+2 < len(raw) && isCont(raw[i+1]) && isCont(raw[i+2]):
			out.WriteRune(rune(uint32(b0&0x0F)<<12 | uint32(raw[i+1]&0x3F)<<6 | uint32(raw[i+2]&0x3F)))
			i += 3
		case b0 >= 0xF0 && b0 <= 0xF7 && i+3 < len(raw) && isCont(raw[i+1]) && isCont(raw[i+2]) && isCont(raw[i+3]):
			cp := uint32(b0&0x07)<<18 | uint32(raw[i+1]&0x3F)<<12 | uint32(raw[i+2]&0x3F)<<6 | uint32(raw[i+3]&0x3F)
			if cp > 0x10FFFF {
				cp = 0xFFFD
			}
			out.WriteRune(rune(cp))
			i += 4
		default:
			out.WriteRune(rune(b0))
			i++
		}
	}
	return out.String()
}

func isCont(b byte) bool { return b >= 0x80 && b <= 0xBF }
