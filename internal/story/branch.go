package story

import (
	"fmt"
	"sort"
	"strings"
)

// Branch status values.
const (
	BranchActive = "active"
	BranchJoined = "joined"
)

// Branch is one world line.
//
// The constitution and the state ledger travel with the row, so a branch can be
// resumed, exported or (later) imported without any side file.
type Branch struct {
	ID        string `json:"id"`
	BookID    string `json:"bookId"`
	ForkSeq   int    `json:"forkSeq"`
	RejoinSeq int    `json:"rejoinSeq"`
	Title     string `json:"title"`
	Status    string `json:"status"`
	Summary   string `json:"summary"`
	State     State  `json:"state"`
	MemoryMD  string `json:"memoryMd"`
	CreatedAt int64  `json:"createdAt"`
	UpdatedAt int64  `json:"updatedAt"`
}

// BranchScene is one generated scene of a world line.
type BranchScene struct {
	BranchID string      `json:"branchId"`
	Seq      int         `json:"seq"`
	CanonSeq int         `json:"canonSeq"`
	Text     string      `json:"text"`
	Summary  string      `json:"summary"`
	Verdicts []Violation `json:"verdicts,omitempty"`
	// Choices are the decision points offered at the end of the scene. Picking
	// one becomes the intent for the next scene, which is what makes a branch
	// playable instead of a chat box.
	Choices   []Choice `json:"choices,omitempty"`
	Model     string   `json:"model"`
	CreatedAt int64    `json:"createdAt"`
}

// SceneGenInput is everything one generation step needs.
type SceneGenInput struct {
	BookTitle  string
	CanonTitle string
	CanonText  string
	Memory     *Memory
	State      *State
	// World is the book's derived stat block. Optional: a branch can run without
	// one, in which case the prompt simply omits the resource section.
	World    *WorldSheet
	Pressure *Pressure
	Intent   string
	Previous []string
	// Violations from a rejected draft are fed back so the retry fixes the
	// specific rule it broke instead of rolling the dice again.
	Violations []Violation
}

// BuildSceneGenPrompt turns the constitution and the world state into a prompt.
//
// The rules are written as a checklist rather than a request: a model that sees
// "请遵守设定" ignores half of it, while an enumerated list survives better and
// can be checked afterwards by ValidateScene.
func BuildSceneGenPrompt(in SceneGenInput) (system, user string) {
	title := strings.TrimSpace(in.BookTitle)
	if title == "" {
		title = "这本书"
	}

	var s strings.Builder
	fmt.Fprintf(&s, "你在续写《%s》的一条平行世界线。只输出正文，不要标题、不要解释、不要 Markdown 标记。\n\n", title)
	writeRules := func(name string, rules []string) {
		if len(rules) == 0 {
			return
		}
		s.WriteString("## " + name + "\n")
		for _, rule := range rules {
			s.WriteString("- " + rule + "\n")
		}
		s.WriteString("\n")
	}
	if in.Memory != nil {
		writeRules("强制规则", in.Memory.HardRules)
		writeRules("世界规则", in.Memory.WorldRules)
		writeRules("用户规则", in.Memory.UserRules)
		writeRules("走向控制", in.Memory.ControlRules)
	}
	s.WriteString("写作要求：\n")
	s.WriteString("- 800 到 1200 字。\n")
	s.WriteString("- 不要复述已经发生过的情节，只推进新内容。\n")
	s.WriteString("- 结尾停在一个具体的动作或一句话上，不要做总结。\n")
	s.WriteString("- 不要使用「仿佛」「不禁」「五味杂陈」这类套话。\n")

	var u strings.Builder
	if in.CanonTitle != "" {
		fmt.Fprintf(&u, "原著这一幕：%s\n", in.CanonTitle)
	}
	if in.CanonText != "" {
		u.WriteString("原著节选：\n")
		u.WriteString(firstRunes(in.CanonText, 1500))
		u.WriteString("\n\n")
	}
	if in.State != nil {
		if len(in.State.Assumptions) > 0 {
			u.WriteString("本世界线的既有设定：\n")
			for _, a := range in.State.Assumptions {
				u.WriteString("- " + a + "\n")
			}
			u.WriteString("\n")
		}
		if len(in.State.Dead) > 0 {
			u.WriteString("已死亡人物（不得出场）：" + strings.Join(in.State.Dead, "、") + "\n\n")
		}
	}
	if in.World != nil && in.State != nil {
		stats := make([]string, 0, len(in.World.Stats))
		for _, def := range in.World.Stats {
			stats = append(stats, fmt.Sprintf("%s %.0f/%.0f", def.Label, in.State.Stats[def.Key], def.Max))
		}
		if len(stats) > 0 {
			u.WriteString("当前状态：" + strings.Join(stats, "，") + "\n")
		}
		if len(in.State.Inventory) > 0 {
			// Sorted so the prompt (and therefore the tests) is deterministic.
			ids := make([]string, 0, len(in.State.Inventory))
			for id := range in.State.Inventory {
				ids = append(ids, id)
			}
			sort.Strings(ids)
			items := make([]string, 0, len(ids))
			for _, id := range ids {
				name := id
				if item, ok := in.World.Item(id); ok {
					name = item.Name
				}
				items = append(items, fmt.Sprintf("%s×%d", name, in.State.Inventory[id]))
			}
			u.WriteString("携带物：" + strings.Join(items, "，") + "\n")
		}
		u.WriteString("\n")
	}
	if len(in.Previous) > 0 {
		u.WriteString("上一幕结尾：\n")
		u.WriteString(firstRunes(in.Previous[len(in.Previous)-1], 400))
		u.WriteString("\n\n")
	}
	if in.Pressure != nil {
		fmt.Fprintf(&u, "世界意志：本幕需要推进「%s」，力度 %.2f。%s\n\n",
			in.Pressure.Summary, in.Pressure.Level, MeansHint(in.Pressure.Means))
	}
	if intent := strings.TrimSpace(in.Intent); intent != "" {
		u.WriteString("读者的额外想法：\n" + intent + "\n\n")
	}
	if len(in.Violations) > 0 {
		u.WriteString("上一稿违反了这些规则，请修正：\n")
		for _, v := range in.Violations {
			u.WriteString("- " + v.Rule + "：" + v.Evidence + "\n")
		}
		u.WriteString("\n")
	}
	u.WriteString("请写出本幕。")
	return s.String(), u.String()
}

// MeansHint explains the pressure lever in the prompt. The ladder is ordered so
// that the character's own motivation is the last resort: changing a mind
// directly is the shortest path to an out-of-character scene.
func MeansHint(means string) string {
	switch means {
	case "environment":
		return "通过环境事件施压（天气、路况、时机），不要直接改变角色动机。"
	case "third_party":
		return "让第三方人物带来变化，不要直接改变主角的想法。"
	case "info_leak":
		return "让某个秘密被意外发现或泄露，推动局面变化。"
	case "resource":
		return "通过资源变化施压（钱、信物、伤病），不要靠角色顿悟。"
	case "motivation":
		return "在已有铺垫的前提下，让角色自己做出决定。"
	default:
		return ""
	}
}

// EstimateRejoin picks a rejoin scene a few steps ahead.
//
// Rejoining immediately makes a choice feel cosmetic; rejoining too far away
// forces a long regeneration whose consistency decays. Four scenes is the
// compromise used by the default settings.
func EstimateRejoin(forkSeq, maxSeq, span int) int {
	if span <= 0 {
		span = 4
	}
	target := forkSeq + span
	if target > maxSeq {
		target = maxSeq
	}
	if target < forkSeq {
		target = forkSeq
	}
	return target
}

// BuildBranchSummaryPrompt condenses a world line so it can rejoin canon.
// A summary is what makes "jump convergence" honest: the reader keeps the
// consequences of their choices without paying for thirty rewritten chapters.
func BuildBranchSummaryPrompt(bookTitle string, scenes []BranchScene) (system, user string) {
	system = "你是小说编辑。用 150 字以内概括这条世界线发生了什么，只输出摘要正文，不要标题。"

	var b strings.Builder
	fmt.Fprintf(&b, "书名：%s\n世界线：\n", bookTitle)
	for _, sc := range scenes {
		summary := strings.TrimSpace(sc.Summary)
		if summary == "" {
			summary = firstRunes(sc.Text, 60)
		}
		fmt.Fprintf(&b, "%d. %s\n", sc.Seq+1, summary)
	}
	b.WriteString("\n请概括这条世界线与原著的差异，以及它最终回到了哪个局面。")
	return system, b.String()
}
