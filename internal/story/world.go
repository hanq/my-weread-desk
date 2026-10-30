package story

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// Stat kinds. The kind only affects presentation: a "condition" renders as a
// bar, a "resource" as a number, a "relation" as a value per character.
const (
	StatResource   = "resource"
	StatCondition  = "condition"
	StatRelation   = "relation"
	StatKnowledge  = "knowledge"
	StatReputation = "reputation"
)

// Caps. A model cannot track more than a handful of numbers reliably, and an
// inventory larger than a few slots stops being a choice.
const (
	MaxWorldStats     = 6
	MaxWorldItems     = 24
	MaxInventorySlots = 8
)

// WorldStat is one book-specific number.
type WorldStat struct {
	Key         string  `json:"key"`
	Label       string  `json:"label"`
	Kind        string  `json:"kind"`
	Min         float64 `json:"min"`
	Max         float64 `json:"max"`
	Initial     float64 `json:"initial"`
	Decay       float64 `json:"decay"`
	EvidenceSeq int     `json:"evidenceSeq"`
	Source      string  `json:"source"`
}

// WorldItem is one book-grounded object.
type WorldItem struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Kind           string `json:"kind"`
	Owner          string `json:"owner"`
	FirstSeq       int    `json:"firstSeq"`
	Effects        string `json:"effects"`
	LimitPerBranch int    `json:"limitPerBranch"`
	Source         string `json:"source"`
}

// WorldSheet is the derived stat block of one book.
type WorldSheet struct {
	BookID string      `json:"bookId"`
	Stats  []WorldStat `json:"stats"`
	Items  []WorldItem `json:"items"`
}

// Stat looks up a stat definition.
func (s *WorldSheet) Stat(key string) (WorldStat, bool) {
	if s == nil {
		return WorldStat{}, false
	}
	for _, def := range s.Stats {
		if def.Key == key {
			return def, true
		}
	}
	return WorldStat{}, false
}

// Item looks up an item definition.
func (s *WorldSheet) Item(id string) (WorldItem, bool) {
	if s == nil {
		return WorldItem{}, false
	}
	for _, item := range s.Items {
		if item.ID == id {
			return item, true
		}
	}
	return WorldItem{}, false
}

// InitState fills the initial values for stats the branch has not seen yet.
func (s *WorldSheet) InitState(st *State) {
	if s == nil || st == nil {
		return
	}
	st.Normalize()
	for _, def := range s.Stats {
		if _, ok := st.Stats[def.Key]; ok {
			continue
		}
		st.Stats[def.Key] = def.Initial
	}
}

// Tick applies per-scene decay.
//
// Time pressure belongs to the engine, not the model: a stat that drains on its
// own means "not acting" costs something, which is what pushes a branch toward
// the canon spine without the narrator being pushed.
func (s *WorldSheet) Tick(st *State) {
	if s == nil || st == nil {
		return
	}
	st.Normalize()
	for _, def := range s.Stats {
		if def.Decay == 0 {
			continue
		}
		next := st.Stats[def.Key] - def.Decay
		if next < def.Min {
			next = def.Min
		}
		st.Stats[def.Key] = next
	}
}

// ApplyWorld validates a world change and applies it atomically.
//
// Stats are clamped rather than rejected: the scene has already been written, so
// a number that overshoots becomes the boundary instead of an error. Items are
// different — an item that does not exist in the book, or a negative balance, is
// a hard failure and nothing from the patch is applied.
func ApplyWorld(sheet *WorldSheet, st *State, p StatePatch) []Violation {
	var out []Violation
	if st == nil {
		return out
	}
	st.Normalize()
	if sheet == nil {
		sheet = &WorldSheet{}
	}

	nextStats := make(map[string]float64, len(st.Stats))
	for k, v := range st.Stats {
		nextStats[k] = v
	}
	nextInv := make(map[string]int, len(st.Inventory))
	for k, v := range st.Inventory {
		nextInv[k] = v
	}

	slots := 0
	for _, qty := range nextInv {
		if qty > 0 {
			slots++
		}
	}

	hard := false
	for id, delta := range p.ItemDelta {
		if delta == 0 {
			continue
		}
		item, ok := sheet.Item(id)
		if !ok {
			out = append(out, Violation{Rule: "未知道具", Evidence: id, Severity: SeverityHard})
			hard = true
			continue
		}
		if delta < 0 && nextInv[id]+delta < 0 {
			out = append(out, Violation{Rule: "道具数量不足", Evidence: item.Name, Severity: SeverityHard})
			hard = true
			continue
		}
		if delta > 0 {
			limit := item.LimitPerBranch
			if limit <= 0 {
				limit = 1
			}
			if nextInv[id]+delta > limit {
				out = append(out, Violation{Rule: "道具超过上限", Evidence: item.Name, Severity: SeverityHard})
				hard = true
				continue
			}
			if nextInv[id] <= 0 {
				slots++
			}
			if slots > MaxInventorySlots {
				out = append(out, Violation{Rule: "背包已满", Evidence: item.Name, Severity: SeverityHard})
				hard = true
				continue
			}
		}
		nextInv[id] += delta
		if nextInv[id] <= 0 {
			delete(nextInv, id)
		}
	}
	if hard {
		return out
	}

	for key, delta := range p.StatDelta {
		if delta == 0 {
			continue
		}
		def, ok := sheet.Stat(key)
		if !ok {
			out = append(out, Violation{Rule: "未知属性", Evidence: key, Severity: SeveritySoft})
			continue
		}
		next := nextStats[key] + delta
		if next < def.Min {
			next = def.Min
		}
		if next > def.Max {
			next = def.Max
		}
		nextStats[key] = next
	}

	st.Stats = nextStats
	st.Inventory = nextInv
	return out
}

// ---------------------------------------------------------------- derivation

// WorldStatFields is a candidate stat returned by the model.
type WorldStatFields struct {
	Key         string  `json:"key"`
	Label       string  `json:"label"`
	Kind        string  `json:"kind"`
	Min         float64 `json:"min"`
	Max         float64 `json:"max"`
	Initial     float64 `json:"initial"`
	Decay       float64 `json:"decay"`
	EvidenceSeq int     `json:"evidenceSeq"`
}

// WorldItemFields is a candidate item returned by the model.
type WorldItemFields struct {
	Name     string `json:"name"`
	Kind     string `json:"kind"`
	Owner    string `json:"owner"`
	FirstSeq int    `json:"firstSeq"`
	Effects  string `json:"effects"`
}

// WorldSheetFields is the whole derivation result.
type WorldSheetFields struct {
	Stats []WorldStatFields `json:"stats"`
	Items []WorldItemFields `json:"items"`
}

// BuildWorldPrompt asks for the smallest number set that actually constrains
// action in this book. The "would it block a choice" test is stated explicitly
// because models otherwise return character sheets or chapter summaries.
func BuildWorldPrompt(bookTitle string, scenes []Scene) (system, user string) {
	system = "你是小说世界的系统设计师。只输出一个 JSON 对象，不要解释，不要使用 Markdown 代码块。"

	var b strings.Builder
	fmt.Fprintf(&b, "书名：%s\n\n章节序列：\n", bookTitle)
	limit := len(scenes)
	if limit > 60 {
		limit = 60
	}
	for _, sc := range scenes[:limit] {
		summary := strings.TrimSpace(sc.Summary)
		if summary == "" {
			summary = sc.ChapterTitle
		}
		fmt.Fprintf(&b, "%d. %s\n", sc.Seq, summary)
	}
	b.WriteString("\n请设计一个尽可能小的世界表。\n")
	b.WriteString("属性 3 到 6 项，只保留「会限制角色行动」的量（钱、伤势、声望、体面、剩余时间、信任）。\n")
	b.WriteString("不要设计等级、经验、攻击力这类与文学无关的数值。\n")
	b.WriteString("道具 5 到 12 件，必须是原著出现过、会影响剧情走向的物件。\n")
	b.WriteString("每项属性给出 key、label、kind（resource/condition/relation/knowledge/reputation）、min、max、initial、decay（每幕自然衰减，常为 0）、evidenceSeq（证据章节序号）。\n")
	b.WriteString("每件道具给出 name、kind、owner、firstSeq、effects（一句话说明它能做什么）。\n")
	b.WriteString(`只输出：{"stats":[{"key":"月钱","label":"月钱","kind":"resource","min":0,"max":100,"initial":2,"decay":1,"evidenceSeq":3}],"items":[{"name":"通灵宝玉","kind":"token","owner":"贾宝玉","firstSeq":3,"effects":"表明身份，可进入内院"}]}`)
	return system, b.String()
}

// ParseWorldFields accepts the model reply, tolerating fences and extra fields.
func ParseWorldFields(raw string) (WorldSheetFields, error) {
	var out WorldSheetFields
	jsonText, err := extractJSONObject(raw)
	if err != nil {
		return out, err
	}
	if err := json.Unmarshal([]byte(jsonText), &out); err != nil {
		return out, fmt.Errorf("story: 世界表 JSON 解析失败: %w", err)
	}

	seen := map[string]bool{}
	stats := make([]WorldStatFields, 0, MaxWorldStats)
	for _, s := range out.Stats {
		s.Key = strings.TrimSpace(s.Key)
		s.Label = strings.TrimSpace(s.Label)
		if s.Key == "" || seen[s.Key] {
			continue
		}
		if s.Label == "" {
			s.Label = s.Key
		}
		if s.Max <= s.Min {
			s.Max = s.Min + 100
		}
		if s.Initial < s.Min || s.Initial > s.Max {
			s.Initial = s.Min
		}
		seen[s.Key] = true
		stats = append(stats, s)
		if len(stats) == MaxWorldStats {
			break
		}
	}
	out.Stats = stats

	items := make([]WorldItemFields, 0, MaxWorldItems)
	seenItem := map[string]bool{}
	for _, it := range out.Items {
		it.Name = strings.TrimSpace(it.Name)
		if it.Name == "" || seenItem[it.Name] {
			continue
		}
		seenItem[it.Name] = true
		items = append(items, it)
		if len(items) == MaxWorldItems {
			break
		}
	}
	out.Items = items

	if len(stats) == 0 && len(items) == 0 {
		return out, errors.New("story: 世界表为空")
	}
	return out, nil
}
