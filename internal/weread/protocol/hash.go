// Package protocol implements the WeRead wire protocol primitives:
// request signing, the `_e` hash, response shard decoding and the read
// reporting payload.
//
// Everything in this package is a pure function of its inputs so it can be
// covered by table tests against the reference implementations.
package protocol

import (
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/rand"
	"sort"
	"strconv"
	"strings"
	"time"
)

// SkillVersion is sent with official gateway requests.
const SkillVersion = "1.0.5"

// ReaderToken is the fallback token used to sign read reports when the reader
// page did not expose `reader.token`. It has been stable in the web reader.
const ReaderToken = "3c5c8717f3daf09iop3423zafeqoi"

// DefaultUserAgent is the desktop browser identity used for web endpoints.
// Keep it identical to the reader page UA so `appId` stays consistent.
const DefaultUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) " +
	"AppleWebKit/537.36 (KHTML, like Gecko) Chrome/135.0.0.0 Safari/537.36 Edg/135.0.0.0"

// MD5Hex returns the lowercase hex MD5 of s.
func MD5Hex(s string) string {
	sum := md5.Sum([]byte(s))
	return hex.EncodeToString(sum[:])
}

// SHA256Hex returns the lowercase hex SHA-256 of s.
func SHA256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// E implements WeRead's `_e()` value encoding, used for reader URLs and for
// the `b`, `c` and `pc` request fields.
func E(value string) string {
	h := MD5Hex(value)
	var b strings.Builder
	b.WriteString(h[:3])

	var chunks []string
	typeFlag := "3"
	numeric := value != "" && isDigits(value)
	if numeric {
		for i := 0; i < len(value); i += 9 {
			end := i + 9
			if end > len(value) {
				end = len(value)
			}
			n, err := strconv.ParseUint(value[i:end], 10, 64)
			if err != nil {
				numeric = false
				break
			}
			chunks = append(chunks, strconv.FormatUint(n, 16))
		}
	}
	if !numeric {
		// Non-numeric input: concatenate the hex code points of every rune.
		typeFlag = "4"
		var sb strings.Builder
		for _, r := range value {
			sb.WriteString(strconv.FormatInt(int64(r), 16))
		}
		chunks = []string{sb.String()}
	}

	b.WriteString(typeFlag)
	b.WriteString("2")
	b.WriteString(h[len(h)-2:])

	for i, chunk := range chunks {
		b.WriteString(fmt.Sprintf("%02x", len(chunk)))
		b.WriteString(chunk)
		if i < len(chunks)-1 {
			b.WriteString("g")
		}
	}

	out := b.String()
	if len(out) < 20 {
		out += h[:20-len(out)]
	}
	return out + MD5Hex(out)[:3]
}

func isDigits(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// EncodeValue reproduces JavaScript's spelling of a value before URL encoding
// (Python's `True`/`False` break the signature, so booleans are textual).
func EncodeValue(v any) string {
	switch t := v.(type) {
	case nil:
		return "null"
	case bool:
		if t {
			return "true"
		}
		return "false"
	case string:
		return t
	case int:
		return strconv.Itoa(t)
	case int64:
		return strconv.FormatInt(t, 10)
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	default:
		return fmt.Sprint(t)
	}
}

// Escape percent-encodes a value the way `encodeURIComponent` does for the
// characters that matter here. Only RFC 3986 unreserved characters survive.
func Escape(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'A' && c <= 'Z', c >= 'a' && c <= 'z', c >= '0' && c <= '9',
			c == '-', c == '_', c == '.', c == '~':
			b.WriteByte(c)
		default:
			b.WriteString(fmt.Sprintf("%%%02X", c))
		}
	}
	return b.String()
}

// SortedQuery builds the canonical query string used for signing. The `s`
// field itself is always excluded.
func SortedQuery(params map[string]any) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		if k == "s" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+Escape(EncodeValue(params[k])))
	}
	return strings.Join(parts, "&")
}

// Sign computes the `s` field of a request from its canonical query string.
func Sign(query string) string {
	a := uint32(0x15051505)
	b := a
	length := len(query)
	for i := length - 1; i > 0; i -= 2 {
		a = (a ^ (uint32(query[i]) << uint((length-i)%30))) & 0x7fffffff
		b = (b ^ (uint32(query[i-1]) << uint(i%30))) & 0x7fffffff
	}
	return strconv.FormatUint(uint64(a+b), 16)
}

// WebAppID derives the deterministic browser `appId` from a user agent.
func WebAppID(userAgent string) string {
	userAgent = strings.TrimSpace(userAgent)
	if userAgent == "" {
		userAgent = DefaultUserAgent
	}
	parts := strings.Split(userAgent, " ")
	if len(parts) > 12 {
		parts = parts[:12]
	}
	var prefix strings.Builder
	for _, p := range parts {
		prefix.WriteString(strconv.Itoa(len(p) % 10))
	}

	hash := uint32(0)
	for i := 0; i < len(userAgent); i++ {
		hash = (0x83*hash + uint32(userAgent[i])) & 0x7fffffff
	}
	return "wb" + prefix.String() + "h" + strconv.FormatUint(uint64(hash), 10)
}

// ContentParams builds the signed body for a chapter shard request.
//
// `style` selects the CSS shard (`e_2`), which alone requires st=1.
func ContentParams(bookID, chapterUID, psvts string, style bool, sc int) map[string]any {
	now := time.Now().Unix()
	if E(strconv.FormatInt(now, 10)) == psvts {
		now++
	}
	ct := strconv.FormatInt(now, 10)

	st := 0
	if style {
		st = 1
	}
	r := rand.Intn(10000)
	params := map[string]any{
		"b":           E(bookID),
		"c":           E(chapterUID),
		"r":           strconv.Itoa(r * r),
		"ct":          ct,
		"ps":          psvts,
		"pc":          E(ct),
		"sc":          sc,
		"prevChapter": false,
		"st":          st,
	}
	params["s"] = Sign(SortedQuery(params))
	return params
}

// ReadReport carries one read-time/progress report.
type ReadReport struct {
	BookID         string
	ChapterUID     int64
	ChapterIndex   int
	ChapterOffset  int
	Progress       int
	Summary        string
	PSVTS          string
	PCLTS          string
	Token          string
	ElapsedSeconds int
	UserAgent      string
	// Now overrides the clock in tests. Zero means time.Now().
	Now time.Time
}

// ReadPayload builds a signed `/web/book/read` body.
func ReadPayload(r ReadReport) map[string]any {
	now := r.Now
	if now.IsZero() {
		now = time.Now()
	}
	ct := now.Unix()
	ts := now.UnixMilli()
	rn := rand.Intn(1000)

	pc := r.PCLTS
	if pc == "" || pc == "0" {
		pc = E(strconv.FormatInt(ct, 10))
	}
	progress := r.Progress
	if progress < 0 {
		progress = 0
	}
	if progress > 100 {
		progress = 100
	}
	token := r.Token
	if token == "" {
		token = ReaderToken
	}

	params := map[string]any{
		"appId": WebAppID(r.UserAgent),
		"b":     E(r.BookID),
		"c":     E(strconv.FormatInt(r.ChapterUID, 10)),
		"ci":    r.ChapterIndex,
		"co":    r.ChapterOffset,
		"sm":    TruncateRunes(r.Summary, 20),
		"pr":    progress,
		"rt":    r.ElapsedSeconds,
		"ts":    ts,
		"rn":    rn,
		"sg":    SHA256Hex(strconv.FormatInt(ts, 10) + strconv.Itoa(rn) + token),
		"ct":    ct,
		"ps":    r.PSVTS,
		"pc":    pc,
	}
	params["s"] = Sign(SortedQuery(params))
	return params
}

// TruncateRunes returns at most n runes of s, never splitting a character.
func TruncateRunes(s string, n int) string {
	if n <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n])
}

// ReaderURL builds the web reader URL for a book, optionally at a chapter.
func ReaderURL(bookID string, chapterUID int64) string {
	u := "https://weread.qq.com/web/reader/" + E(bookID)
	if chapterUID > 0 {
		u += "k" + E(strconv.FormatInt(chapterUID, 10))
	}
	return u
}

// MPReaderURL builds the official-account reader URL.
func MPReaderURL(bookID string) string {
	return "https://weread.qq.com/web/mp/reader/" + E(bookID)
}

// IsMPBook reports whether a book id belongs to a WeChat official account.
func IsMPBook(bookID string) bool {
	return strings.HasPrefix(bookID, "MP_WXS_")
}
