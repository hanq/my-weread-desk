package story

import "strings"

// Patch limits bound what one intent may change. A model that decides to rewrite
// the whole world in a single step is a failure mode, not a feature.
const (
	maxPatchAssumptions = 4
	maxPatchKills       = 3
	maxPatchFacts       = 8
	maxPatchLearns      = 16
)

// ValidatePatch checks a state patch before it is applied.
//
// This is structural validation only: it cannot judge whether a death is
// dramatically justified. What it can do is stop oversized or malformed
// patches, and stop a patch from re-killing a character or learning the same
// fact twice. Semantic checks belong to the scene validator.
func ValidatePatch(p StatePatch, st *State) []Violation {
	var out []Violation
	add := func(rule, evidence string, sev Severity) {
		out = append(out, Violation{Rule: rule, Evidence: evidence, Severity: sev})
	}

	if len(p.AddAssumptions) > maxPatchAssumptions {
		add("状态补丁过大", "假设条目过多", SeverityHard)
	}
	if len(p.Kill) > maxPatchKills {
		add("状态补丁过大", "死亡条目过多", SeverityHard)
	}
	if len(p.SetFacts) > maxPatchFacts {
		add("状态补丁过大", "事实条目过多", SeverityHard)
	}
	if len(p.StatDelta) > MaxWorldStats {
		add("状态补丁过大", "属性条目过多", SeverityHard)
	}
	if len(p.ItemDelta) > MaxInventorySlots {
		add("状态补丁过大", "道具条目过多", SeverityHard)
	}

	learnCount := 0
	for who, facts := range p.Learn {
		if strings.TrimSpace(who) == "" {
			add("知情条目无效", "人物为空", SeverityHard)
		}
		for _, fact := range facts {
			if strings.TrimSpace(fact) == "" {
				add("知情条目无效", who, SeverityHard)
			}
		}
		learnCount += len(facts)
	}
	if learnCount > maxPatchLearns {
		add("状态补丁过大", "知情条目过多", SeverityHard)
	}

	for _, a := range p.AddAssumptions {
		if strings.TrimSpace(a) == "" {
			add("假设条目无效", "空假设", SeverityHard)
		}
	}
	for _, name := range p.Kill {
		name = strings.TrimSpace(name)
		if name == "" {
			add("死亡条目无效", "空姓名", SeverityHard)
			continue
		}
		if st != nil && st.IsDead(name) {
			add("重复死亡", name, SeveritySoft)
		}
	}
	for key := range p.SetFacts {
		if strings.TrimSpace(key) == "" {
			add("事实条目无效", "空键", SeverityHard)
		}
	}
	for key := range p.SetItems {
		if strings.TrimSpace(key) == "" {
			add("物件条目无效", "空键", SeverityHard)
		}
	}
	return out
}
