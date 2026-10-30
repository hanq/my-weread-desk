package story

import (
	"fmt"
	"strings"
)

// Severity separates a scene that must not be shown from one that is merely
// suspicious. Callers retry on hard violations and log soft ones.
type Severity string

const (
	SeverityHard Severity = "hard"
	SeveritySoft Severity = "soft"
)

// Violation is one failed check.
type Violation struct {
	Rule     string   `json:"rule"`
	Evidence string   `json:"evidence"`
	Severity Severity `json:"severity"`
}

// HasHardViolation reports whether a scene must be rejected.
func HasHardViolation(vs []Violation) bool {
	for _, v := range vs {
		if v.Severity == SeverityHard {
			return true
		}
	}
	return false
}

// revivalTerms are the phrasings that turn "a dead character is mentioned" into
// "a dead character came back". Mentioning the dead in memory or dialogue is
// normal fiction; bringing them back is a hard rule break.
var revivalTerms = []string{"复活", "死而复生", "起死回生"}

// ValidateScene checks a generated scene against the constitution and the state
// ledger.
//
// Every check here is heuristic and cheap on purpose. The expensive judgement —
// is this in character, does it contradict the previous scene — belongs to the
// LLM reviewer that runs only after these pass. Putting the cheap checks first
// keeps retries from doubling the token bill.
func ValidateScene(text string, m *Memory, st *State) []Violation {
	text = strings.TrimSpace(text)
	if text == "" {
		return []Violation{{Rule: "非空", Evidence: "场景为空", Severity: SeverityHard}}
	}
	if m == nil {
		return nil
	}

	var out []Violation

	for _, term := range m.Banned {
		term = strings.TrimSpace(term)
		if term == "" {
			continue
		}
		if strings.Contains(text, term) {
			out = append(out, Violation{
				Rule:     "禁用词：" + term,
				Evidence: term,
				Severity: SeverityHard,
			})
		}
	}

	n := len([]rune(text))
	lo, hi := m.SceneLength[0], m.SceneLength[1]
	if lo > 0 && n < lo*3/5 {
		out = append(out, Violation{
			Rule:     "篇幅过短",
			Evidence: fmt.Sprintf("%d 字，下限 %d", n, lo),
			Severity: SeveritySoft,
		})
	}
	if hi > 0 && n > hi*3/2 {
		out = append(out, Violation{
			Rule:     "篇幅过长",
			Evidence: fmt.Sprintf("%d 字，上限 %d", n, hi),
			Severity: SeveritySoft,
		})
	}

	if st != nil {
		revival := ""
		for _, kw := range revivalTerms {
			if strings.Contains(text, kw) {
				revival = kw
				break
			}
		}
		for _, name := range st.Dead {
			name = strings.TrimSpace(name)
			if name == "" || !strings.Contains(text, name) {
				continue
			}
			if revival != "" {
				out = append(out, Violation{
					Rule:     "死者不可复活",
					Evidence: name + " / " + revival,
					Severity: SeverityHard,
				})
			} else {
				out = append(out, Violation{
					Rule:     "死者出场需确认为回忆或提及",
					Evidence: name,
					Severity: SeveritySoft,
				})
			}
		}
	}

	pov := strings.ToLower(strings.TrimSpace(m.POV))
	firstPerson := strings.Count(text, "我")
	switch {
	case strings.HasPrefix(pov, "first") && firstPerson == 0:
		out = append(out, Violation{
			Rule:     "视角：第一人称",
			Evidence: "全文没有“我”",
			Severity: SeverityHard,
		})
	case strings.HasPrefix(pov, "third") && firstPerson >= 3:
		out = append(out, Violation{
			Rule:     "视角：第三人称限知",
			Evidence: fmt.Sprintf("出现 %d 次“我”", firstPerson),
			Severity: SeveritySoft,
		})
	}

	return out
}
