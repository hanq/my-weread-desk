package story

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// Intent feasibility values.
const (
	IntentOK         = "ok"
	IntentNeedsSetup = "needs_setup"
	IntentViolates   = "violates"
)

// Intent is the compiled form of a free-form reader request.
//
// Compiling before generating is what lets a conflict be shown as a choice
// instead of surfacing as a failed generation: the reader sees why a request is
// impossible and what could replace it, rather than a scene that silently
// ignored them or broke a rule.
type Intent struct {
	Raw          string     `json:"raw"`
	Feasibility  string     `json:"feasibility"`
	Goal         string     `json:"goal"`
	Reason       string     `json:"reason"`
	BlockedBy    []string   `json:"blockedBy,omitempty"`
	Alternatives []string   `json:"alternatives,omitempty"`
	StatePatch   StatePatch `json:"statePatch"`
}

// PrecheckIntent catches the violations that need no model: banned words and a
// dead character being brought back. Running this first keeps the common
// conflicts free and deterministic.
func PrecheckIntent(raw string, mem *Memory, st *State) []Violation {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}

	var out []Violation
	if mem != nil {
		for _, term := range mem.Banned {
			term = strings.TrimSpace(term)
			if term != "" && strings.Contains(raw, term) {
				out = append(out, Violation{
					Rule:     "禁用词：" + term,
					Evidence: term,
					Severity: SeverityHard,
				})
			}
		}
	}

	if st != nil && len(st.Dead) > 0 {
		for _, kw := range revivalTerms {
			if strings.Contains(raw, kw) {
				out = append(out, Violation{
					Rule:     "死者不可复活",
					Evidence: kw,
					Severity: SeverityHard,
				})
				break
			}
		}
	}
	return out
}

// AlternativesFor offers concrete replacements for a blocked request. A refusal
// without an alternative is a dead end for the reader.
func AlternativesFor(vs []Violation) []string {
	for _, v := range vs {
		switch {
		case strings.Contains(v.Rule, "复活"):
			return []string{
				"改为：他其实没死，但失去了记忆",
				"改为：出现一个与他相似的人",
				"放弃这个想法",
			}
		case strings.Contains(v.Rule, "禁用词"):
			return []string{
				"换一种不出现「" + strings.TrimPrefix(v.Rule, "禁用词：") + "」的说法",
				"放弃这个想法",
			}
		}
	}
	return []string{"换一种不触碰硬规则的说法", "放弃这个想法"}
}

// BuildIntentPrompt asks the model to classify a request against the
// constitution and turn it into one concrete goal plus a state patch.
func BuildIntentPrompt(mem *Memory, st *State, raw string) (system, user string) {
	system = "你是小说续写的意图分析器。只输出一个 JSON 对象，不要解释，不要使用 Markdown 代码块。"

	var b strings.Builder
	fmt.Fprintf(&b, "读者的想法：%s\n\n", strings.TrimSpace(raw))
	if mem != nil {
		if len(mem.HardRules) > 0 {
			b.WriteString("强制规则：\n")
			for _, rule := range mem.HardRules {
				b.WriteString("- " + rule + "\n")
			}
		}
		if len(mem.Banned) > 0 {
			b.WriteString("禁用词：" + strings.Join(mem.Banned, "、") + "\n")
		}
	}
	if st != nil {
		if len(st.Dead) > 0 {
			b.WriteString("已死亡人物：" + strings.Join(st.Dead, "、") + "\n")
		}
		if len(st.Assumptions) > 0 {
			b.WriteString("既有设定：" + strings.Join(st.Assumptions, "；") + "\n")
		}
	}
	b.WriteString(`
判断这个想法能否执行：
- ok：可以直接写进下一幕
- needs_setup：需要一个过渡场景先铺垫
- violates：违反强制规则，不能执行

只输出：
{"feasibility":"ok","goal":"本幕要完成的一件事（不超过 40 字）","reason":"若不能执行，说明原因","blockedBy":[],"alternatives":[],"statePatch":{"addAssumptions":[],"kill":[],"learn":{}}}`)
	return system, b.String()
}

// ParseIntent accepts the model reply, tolerating fences and unknown shape.
func ParseIntent(raw string) (Intent, error) {
	var out Intent
	jsonText, err := extractJSONObject(raw)
	if err != nil {
		return out, err
	}
	if err := json.Unmarshal([]byte(jsonText), &out); err != nil {
		return out, fmt.Errorf("story: 意图 JSON 解析失败: %w", err)
	}
	out.Goal = strings.TrimSpace(out.Goal)
	out.Reason = strings.TrimSpace(out.Reason)
	out.BlockedBy = compactNames(out.BlockedBy)
	out.Alternatives = compactNames(out.Alternatives)
	switch out.Feasibility {
	case IntentOK, IntentNeedsSetup, IntentViolates:
	default:
		out.Feasibility = IntentOK
	}
	if out.Goal == "" && out.Feasibility == IntentOK {
		return out, errors.New("story: 意图缺少 goal")
	}
	return out, nil
}
