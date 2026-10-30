// Package story holds the pieces a branch needs to stay coherent: the world
// constitution (memory.md), the state ledger it is judged against, and the
// validator that rejects a generated scene before it reaches the reader.
//
// The design rule that shapes this package: the constitution is data, not a
// prompt. Rules are parsed, hashed and checked; a violation is a value the
// caller can act on, not a suggestion in a system message.
package story

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// MemorySchema is bumped whenever the file layout changes. A reader that sees
// an unknown schema must refuse the file instead of guessing field meaning.
const MemorySchema = 1

var (
	// ErrSchema reports a file written by a newer or older layout.
	ErrSchema = errors.New("story: 不支持的 memory schema")
	// ErrTampered reports that the engine block no longer matches its hash.
	ErrTampered = errors.New("story: 引擎区被外部修改")
	// ErrNoUserBlock reports a constitution without its user rules.
	ErrNoUserBlock = errors.New("story: 缺少 USER 区块")
	// ErrUserBodyDrift reports a caller that mutated the parsed rule slices
	// instead of calling SetUserBody. Failing loudly is the point: silently
	// ignoring the edit would hide a rule the caller believes is active.
	ErrUserBodyDrift = errors.New("story: 用户规则被直接修改，请使用 SetUserBody")
)

const (
	userBegin   = "<!-- USER:BEGIN -->"
	userEnd     = "<!-- USER:END -->"
	engineBegin = "<!-- ENGINE:BEGIN -->"
	engineEnd   = "<!-- ENGINE:END -->"
)

// BookRef identifies the source edition. EditionHash is a content hash, not a
// WeRead book id: ids are account-scoped and do not survive sharing, while the
// text hash does.
type BookRef struct {
	Title       string
	Author      string
	EditionHash string
}

// EngineZone is the machine-maintained half of the constitution. Everything
// here is derived from what has actually happened in the branch, so it is
// written only by this package.
type EngineZone struct {
	SatisfiedAnchors []string
	Divergence       float64
	UpdatedAt        int64
	Summary          string
}

// Memory is the whole constitution.
//
// HardRules/WorldRules/UserRules/ControlRules are the *parsed view* of
// UserBody. Render never rebuilds UserBody from them: it writes the verbatim
// block that was loaded. That is the enforcement behind "the model may not
// rewrite the reader's rules" — a generation path simply has no function that
// can change UserBody.
type Memory struct {
	Schema        int
	Book          BookRef
	BranchID      string
	Mode          string // strict | balance | free
	Gravity       float64
	DivergenceCap float64
	POV           string
	Tense         string
	SceneLength   [2]int
	Banned        []string
	MustSatisfy   []string

	HardRules    []string
	WorldRules   []string
	UserRules    []string
	ControlRules []string

	Engine EngineZone

	// UserBody is the exact text between the USER markers, including headings.
	UserBody string

	engineHash string
}

// NewMemory returns a constitution with usable defaults. The default gravity is
// the "balance" setting: a branch may drift for a few scenes, then the story is
// pulled back.
func NewMemory(book BookRef) *Memory {
	m := &Memory{
		Schema:        MemorySchema,
		Book:          book,
		Mode:          "balance",
		Gravity:       0.7,
		DivergenceCap: 0.6,
		POV:           "third-limited",
		Tense:         "past",
		SceneLength:   [2]int{800, 1200},
		HardRules: []string{
			"不得改写已发生事件的结果",
			"死者不可复活",
			"不得引入原著不存在的能力体系",
			"不得让角色知道他在原著此时还不知道的信息",
		},
		WorldRules:   []string{"时代与地点遵循原著设定"},
		UserRules:    nil,
		ControlRules: []string{"锚点到期前 2 幕开始施压", "施压优先使用环境与第三方事件，最后才改角色动机"},
	}
	m.UserBody = renderUserBody(m)
	return m
}

// LoadMemory reads and validates a constitution from disk.
func LoadMemory(path string) (*Memory, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParseMemory(string(raw))
}

// Save validates and writes the constitution. The write is atomic so a crash
// cannot leave a half-written constitution behind.
func (m *Memory) Save(path string) error {
	if err := m.Validate(); err != nil {
		return err
	}
	rendered, err := m.Render()
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(rendered), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// Validate reports whether the constitution is internally consistent.
func (m *Memory) Validate() error {
	if m.Schema != MemorySchema {
		return ErrSchema
	}
	if strings.TrimSpace(m.Book.Title) == "" {
		return errors.New("story: 缺少书名")
	}
	if m.Gravity < 0 || m.Gravity > 1 {
		return fmt.Errorf("story: gravity 超出 0..1：%v", m.Gravity)
	}
	if strings.TrimSpace(m.UserBody) == "" {
		return ErrNoUserBlock
	}
	if m.engineHash != "" && m.engineHash != engineHash(m.Engine) {
		return ErrTampered
	}
	return nil
}

// SetUserBody replaces the user block. Only a user-facing edit path may call
// this; the generation path never does, which is what keeps the rules stable.
func (m *Memory) SetUserBody(body string) error {
	body = strings.TrimSpace(body)
	if body == "" {
		return ErrNoUserBlock
	}
	sections, err := parseSections(body)
	if err != nil {
		return err
	}
	m.UserBody = body
	m.HardRules = sections["强制规则"]
	m.WorldRules = sections["世界规则"]
	m.UserRules = sections["用户规则"]
	m.ControlRules = sections["走向控制"]
	return nil
}

// AddUserRule appends a rule to the user section and re-renders the block.
// This is the only programmatic path allowed to change user rules; the
// generation path never calls it, which is what keeps the constitution stable.
func (m *Memory) AddUserRule(rule string) error {
	rule = strings.TrimSpace(rule)
	if rule == "" {
		return nil
	}
	m.UserRules = append(m.UserRules, rule)
	m.UserBody = renderUserBody(m)
	return nil
}

// Render serialises the constitution. Front-matter keeps a flat key layout so
// the file stays diff-friendly and a hand edit cannot silently change nesting.
func (m *Memory) Render() (string, error) {
	if m.Schema != MemorySchema {
		return "", ErrSchema
	}
	if strings.TrimSpace(m.UserBody) == "" {
		m.UserBody = renderUserBody(m)
	}

	// The rendered USER block is the source of truth. If the parsed slices no
	// longer match it, the caller edited rules in a way Render cannot honour.
	parsed, err := parseSections(m.UserBody)
	if err != nil {
		return "", err
	}
	if !sameRules(parsed["强制规则"], m.HardRules) ||
		!sameRules(parsed["世界规则"], m.WorldRules) ||
		!sameRules(parsed["用户规则"], m.UserRules) ||
		!sameRules(parsed["走向控制"], m.ControlRules) {
		return "", ErrUserBodyDrift
	}

	m.engineHash = engineHash(m.Engine)
	var b strings.Builder
	b.WriteString("---\n")
	fmt.Fprintf(&b, "schema: %d\n", m.Schema)
	fmt.Fprintf(&b, "book.title: %s\n", m.Book.Title)
	fmt.Fprintf(&b, "book.author: %s\n", m.Book.Author)
	fmt.Fprintf(&b, "book.editionHash: %s\n", m.Book.EditionHash)
	fmt.Fprintf(&b, "branchId: %s\n", m.BranchID)
	fmt.Fprintf(&b, "mode: %s\n", m.Mode)
	fmt.Fprintf(&b, "gravity: %s\n", formatFloat(m.Gravity))
	fmt.Fprintf(&b, "divergenceCap: %s\n", formatFloat(m.DivergenceCap))
	fmt.Fprintf(&b, "pov: %s\n", m.POV)
	fmt.Fprintf(&b, "tense: %s\n", m.Tense)
	fmt.Fprintf(&b, "sceneLength: %d,%d\n", m.SceneLength[0], m.SceneLength[1])
	fmt.Fprintf(&b, "banned: %s\n", strings.Join(m.Banned, ","))
	fmt.Fprintf(&b, "mustSatisfy: %s\n", strings.Join(m.MustSatisfy, ","))
	fmt.Fprintf(&b, "engineHash: %s\n", m.engineHash)
	b.WriteString("---\n\n")

	b.WriteString(userBegin)
	b.WriteString("\n")
	b.WriteString(m.UserBody)
	if !strings.HasSuffix(m.UserBody, "\n") {
		b.WriteString("\n")
	}
	b.WriteString(userEnd)
	b.WriteString("\n\n")

	b.WriteString(engineBegin)
	b.WriteString("\n")
	b.WriteString(renderEngine(m.Engine))
	b.WriteString(engineEnd)
	b.WriteString("\n")
	return b.String(), nil
}

// ParseMemory reads the layout produced by Render. Unknown front-matter keys are
// rejected rather than ignored: a typo that silently drops "banned" would
// disable a hard constraint without telling anyone.
func ParseMemory(raw string) (*Memory, error) {
	front, body, err := splitFrontMatter(raw)
	if err != nil {
		return nil, err
	}
	m := &Memory{Schema: MemorySchema, SceneLength: [2]int{800, 1200}}
	if err := parseFrontMatter(front, m); err != nil {
		return nil, err
	}

	user, err := extractBlock(body, userBegin, userEnd)
	if err != nil {
		return nil, err
	}
	engineBody, err := extractBlock(body, engineBegin, engineEnd)
	if err != nil {
		return nil, err
	}
	m.Engine = parseEngine(engineBody)

	if m.engineHash != "" {
		if got := engineHash(m.Engine); got != m.engineHash {
			return nil, ErrTampered
		}
	}
	if err := m.SetUserBody(user); err != nil {
		return nil, err
	}
	if m.Schema != MemorySchema {
		return nil, ErrSchema
	}
	return m, nil
}

func splitFrontMatter(raw string) (front, body string, err error) {
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	if !strings.HasPrefix(raw, "---\n") {
		return "", "", errors.New("story: memory.md 必须以 --- 开头")
	}
	rest := raw[len("---\n"):]
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return "", "", errors.New("story: front-matter 没有结束标记")
	}
	front = rest[:end]
	body = rest[end+len("\n---"):]
	body = strings.TrimPrefix(body, "\n")
	return front, body, nil
}

func parseFrontMatter(front string, m *Memory) error {
	for _, line := range strings.Split(front, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			return fmt.Errorf("story: front-matter 行无法解析：%q", line)
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		switch key {
		case "schema":
			n, err := strconv.Atoi(value)
			if err != nil {
				return fmt.Errorf("story: schema 不是整数：%q", value)
			}
			m.Schema = n
		case "book.title":
			m.Book.Title = value
		case "book.author":
			m.Book.Author = value
		case "book.editionHash":
			m.Book.EditionHash = value
		case "branchId":
			m.BranchID = value
		case "mode":
			m.Mode = value
		case "gravity":
			m.Gravity = parseFloat(value, 0.7)
		case "divergenceCap":
			m.DivergenceCap = parseFloat(value, 0.6)
		case "pov":
			m.POV = value
		case "tense":
			m.Tense = value
		case "sceneLength":
			m.SceneLength = parseLength(value)
		case "banned":
			m.Banned = splitList(value)
		case "mustSatisfy":
			m.MustSatisfy = splitList(value)
		case "engineHash":
			m.engineHash = value
		default:
			return fmt.Errorf("story: 未知的 front-matter 字段 %q", key)
		}
	}
	return nil
}

func extractBlock(body, begin, end string) (string, error) {
	start := strings.Index(body, begin)
	if start < 0 {
		return "", fmt.Errorf("story: 缺少区块 %s", begin)
	}
	start += len(begin)
	rest := body[start:]
	stop := strings.Index(rest, end)
	if stop < 0 {
		return "", fmt.Errorf("story: 区块 %s 没有结束标记", begin)
	}
	return strings.TrimSpace(rest[:stop]), nil
}

func parseSections(body string) (map[string][]string, error) {
	out := map[string][]string{}
	var current string
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "## "):
			current = strings.TrimSpace(strings.TrimPrefix(trimmed, "## "))
			if current == "" {
				return nil, errors.New("story: 空的小节标题")
			}
			out[current] = nil
		case strings.HasPrefix(trimmed, "- "):
			if current == "" {
				return nil, errors.New("story: 规则出现在小节之外")
			}
			out[current] = append(out[current], strings.TrimSpace(strings.TrimPrefix(trimmed, "- ")))
		case trimmed == "":
			continue
		default:
			return nil, fmt.Errorf("story: USER 区块只允许 ## 标题与 - 列表，遇到 %q", trimmed)
		}
	}
	return out, nil
}

func renderUserBody(m *Memory) string {
	var b strings.Builder
	writeSection := func(title string, rules []string) {
		if len(rules) == 0 {
			return
		}
		b.WriteString("## ")
		b.WriteString(title)
		b.WriteString("\n")
		for _, rule := range rules {
			b.WriteString("- ")
			b.WriteString(rule)
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}
	writeSection("强制规则", m.HardRules)
	writeSection("世界规则", m.WorldRules)
	writeSection("用户规则", m.UserRules)
	writeSection("走向控制", m.ControlRules)
	return strings.TrimRight(b.String(), "\n")
}

// renderEngine is the canonical form used for hashing. Formatting is fixed so a
// whitespace-only edit to the file does not trip tamper detection, while a
// value edit does.
func renderEngine(e EngineZone) string {
	var b strings.Builder
	fmt.Fprintf(&b, "satisfiedAnchors: %s\n", strings.Join(e.SatisfiedAnchors, ","))
	fmt.Fprintf(&b, "divergence: %.4f\n", e.Divergence)
	fmt.Fprintf(&b, "updatedAt: %d\n", e.UpdatedAt)
	fmt.Fprintf(&b, "summary: %s\n", strings.ReplaceAll(e.Summary, "\n", " "))
	return b.String()
}

func parseEngine(body string) EngineZone {
	var e EngineZone
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		switch key {
		case "satisfiedAnchors":
			e.SatisfiedAnchors = splitList(value)
		case "divergence":
			e.Divergence = parseFloat(value, 0)
		case "updatedAt":
			n, _ := strconv.ParseInt(value, 10, 64)
			e.UpdatedAt = n
		case "summary":
			e.Summary = value
		}
	}
	return e
}

func engineHash(e EngineZone) string {
	sum := sha256.Sum256([]byte(renderEngine(e)))
	return "sha256:" + hex.EncodeToString(sum[:])
}

func splitList(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func parseLength(value string) [2]int {
	parts := splitList(value)
	if len(parts) == 1 {
		n, err := strconv.Atoi(parts[0])
		if err == nil && n > 0 {
			return [2]int{n, n}
		}
	}
	if len(parts) >= 2 {
		lo, err1 := strconv.Atoi(parts[0])
		hi, err2 := strconv.Atoi(parts[1])
		if err1 == nil && err2 == nil && lo > 0 && hi >= lo {
			return [2]int{lo, hi}
		}
	}
	return [2]int{800, 1200}
}

func parseFloat(value string, fallback float64) float64 {
	f, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return fallback
	}
	return f
}

func formatFloat(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}

func sameRules(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
