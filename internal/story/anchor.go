package story

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// Anchor kinds. The kind only affects how the generator is told to use the
// anchor; gravity itself treats every kind the same.
const (
	AnchorIdentity     = "identity"
	AnchorRelationship = "relationship"
	AnchorReveal       = "reveal"
	AnchorClimax       = "climax"
	AnchorEnding       = "ending"
	AnchorBeat         = "beat"
)

// CanonAnchor is a load-bearing event of the original work.
//
// SceneSeq is where it happens in canon; DeadlineSeq is the last scene at which
// a branch may still avoid it. The gap between the two is the room a branch has
// to reach the same beat by a different route.
type CanonAnchor struct {
	ID          string  `json:"id"`
	BookID      string  `json:"bookId"`
	SceneSeq    int     `json:"sceneSeq"`
	DeadlineSeq int     `json:"deadlineSeq"`
	Kind        string  `json:"kind"`
	Summary     string  `json:"summary"`
	Weight      float64 `json:"weight"`
	Source      string  `json:"source"`
}

// Satisfaction marks anchors a branch has already realised.
type Satisfaction map[string]bool

// Pressure is the steering instruction for one scene.
type Pressure struct {
	AnchorID string  `json:"anchorId"`
	Summary  string  `json:"summary"`
	Level    float64 `json:"level"`
	DueIn    int     `json:"dueIn"`
	Means    string  `json:"means"`
}

// Gravity presets, exposed so the UI can offer three named settings.
const (
	GravityStrict  = 1.0
	GravityBalance = 0.7
	GravityFree    = 0.3
)

// Gravity returns 0..1: how hard the next scene should be pulled toward canon.
//
// Only anchors that are close to their deadline contribute. Pulling every scene
// "to stay close to the book" is what makes a branch feel railroaded; a reader
// should notice the world pushing back only when it matters.
func Gravity(currentSeq int, anchors []CanonAnchor, sat Satisfaction) float64 {
	var out float64
	for _, a := range anchors {
		if sat[a.ID] {
			continue
		}
		if lvl := anchorLevel(currentSeq, a); lvl > out {
			out = lvl
		}
	}
	return out
}

// Divergence estimates how far a branch has drifted, 0..1.
//
// Coverage dominates: an overdue anchor is the clearest signal that the branch
// is no longer telling the same story. Relation drift and unresolved
// foreshadowing are inputs the caller supplies once those ledgers exist.
func Divergence(currentSeq int, anchors []CanonAnchor, sat Satisfaction, relationDrift, unresolvedGuns float64) float64 {
	due, satisfied := 0, 0
	for _, a := range anchors {
		if a.DeadlineSeq > currentSeq {
			continue
		}
		due++
		if sat[a.ID] {
			satisfied++
		}
	}
	coverage := 0.0
	if due > 0 {
		coverage = 1 - float64(satisfied)/float64(due)
	}
	return clamp01(0.45*coverage + 0.30*clamp01(relationDrift) + 0.25*clamp01(unresolvedGuns))
}

// PlanPressure chooses the anchor to push and the cheapest means that can do it.
//
// The means ladder is deliberate: environment first, character motivation last.
// Making a character change their mind is the shortest path to OOC, so it is
// reserved for the case where every external lever has already failed.
func PlanPressure(currentSeq int, anchors []CanonAnchor, sat Satisfaction, divergence float64) *Pressure {
	best := -1
	bestLevel := 0.0
	for i, a := range anchors {
		if sat[a.ID] {
			continue
		}
		lvl := anchorLevel(currentSeq, a)
		if best < 0 || lvl > bestLevel ||
			(lvl == bestLevel && a.DeadlineSeq < anchors[best].DeadlineSeq) {
			best, bestLevel = i, lvl
		}
	}
	if best < 0 {
		return nil
	}
	a := anchors[best]
	level := clamp01(bestLevel + divergence*0.5)
	return &Pressure{
		AnchorID: a.ID,
		Summary:  a.Summary,
		Level:    level,
		DueIn:    a.DeadlineSeq - currentSeq,
		Means:    meansFor(level),
	}
}

func anchorLevel(currentSeq int, a CanonAnchor) float64 {
	weight := a.Weight
	if weight <= 0 {
		weight = 0.8
	}
	dueIn := a.DeadlineSeq - currentSeq
	switch {
	case dueIn <= 0:
		return clamp01(1.0 * weight)
	case dueIn <= 2:
		return clamp01(0.7 * weight)
	case dueIn <= 4:
		return clamp01(0.35 * weight)
	default:
		return 0
	}
}

func meansFor(level float64) string {
	switch {
	case level < 0.35:
		return "environment"
	case level < 0.55:
		return "third_party"
	case level < 0.75:
		return "info_leak"
	case level < 0.9:
		return "resource"
	default:
		return "motivation"
	}
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// ---------------------------------------------------------------- suggestion

// AnchorFields is one candidate returned by the model.
type AnchorFields struct {
	SceneSeq int     `json:"sceneSeq"`
	Kind     string  `json:"kind"`
	Summary  string  `json:"summary"`
	Weight   float64 `json:"weight"`
}

// anchorPromptScenes bounds the prompt. A book can have hundreds of chapters;
// the first pass only needs enough to identify the spine.
const anchorPromptScenes = 80

// BuildAnchorPrompt asks for the load-bearing beats only. The "delete it and the
// rest still stands" test is stated explicitly because models otherwise return
// chapter summaries, which are useless as anchors.
func BuildAnchorPrompt(bookTitle string, scenes []Scene) (system, user string) {
	system = "你是中文小说的结构编辑。只输出一个 JSON 数组，不要输出解释，不要使用 Markdown 代码块。"

	var b strings.Builder
	fmt.Fprintf(&b, "书名：%s\n\n章节序列：\n", bookTitle)
	limit := len(scenes)
	if limit > anchorPromptScenes {
		limit = anchorPromptScenes
	}
	for _, sc := range scenes[:limit] {
		summary := strings.TrimSpace(sc.Summary)
		if summary == "" {
			summary = sc.ChapterTitle
		}
		fmt.Fprintf(&b, "%d. %s\n", sc.Seq, summary)
	}
	b.WriteString("\n请找出 5 到 10 个承重事件（删掉它，后面的故事就不成立）。\n")
	b.WriteString("每项输出：sceneSeq（事件发生的章节序号，必须是上面出现过的数字）、")
	b.WriteString("kind（identity/relationship/reveal/climax/ending/beat 之一）、")
	b.WriteString("summary（一句话，不超过 40 字）、weight（0 到 1，越不可让步越高）。\n")
	b.WriteString("只输出 JSON 数组，例如：[{\"sceneSeq\":3,\"kind\":\"reveal\",\"summary\":\"身世被揭穿\",\"weight\":0.9}]")
	return system, b.String()
}

// ParseAnchors accepts the model reply, tolerating fences and surrounding prose.
func ParseAnchors(raw string) ([]AnchorFields, error) {
	start := strings.Index(raw, "[")
	end := strings.LastIndex(raw, "]")
	if start < 0 || end <= start {
		return nil, errors.New("story: 模型返回中没有 JSON 数组")
	}
	var out []AnchorFields
	if err := json.Unmarshal([]byte(raw[start:end+1]), &out); err != nil {
		return nil, fmt.Errorf("story: 锚点 JSON 解析失败: %w", err)
	}

	cleaned := make([]AnchorFields, 0, len(out))
	for _, a := range out {
		a.Summary = strings.TrimSpace(a.Summary)
		if a.Summary == "" || a.SceneSeq < 0 {
			continue
		}
		if a.Kind == "" {
			a.Kind = AnchorBeat
		}
		if a.Weight <= 0 || a.Weight > 1 {
			a.Weight = 0.8
		}
		cleaned = append(cleaned, a)
		if len(cleaned) == 12 {
			break
		}
	}
	if len(cleaned) == 0 {
		return nil, errors.New("story: 锚点数组为空")
	}
	return cleaned, nil
}

// BuildAnchorCheckPrompt asks whether one scene realised one anchor.
func BuildAnchorCheckPrompt(a CanonAnchor, sceneText string) (system, user string) {
	system = "你是小说编辑。只回答 yes 或 no，不要解释。"

	var b strings.Builder
	fmt.Fprintf(&b, "锚点：%s\n\n", a.Summary)
	b.WriteString("本幕正文：\n")
	b.WriteString(firstRunes(sceneText, 1200))
	b.WriteString("\n\n这一幕是否让这个事件发生了？只回答 yes 或 no。")
	return system, b.String()
}

// ParseAnchorCheck reads a yes/no reply. Negatives are tested first because
// the Chinese word for "no" contains the character for "yes".
func ParseAnchorCheck(raw string) (bool, error) {
	lower := strings.ToLower(strings.TrimSpace(raw))
	if lower == "" {
		return false, errors.New("story: 锚点判定为空")
	}
	switch {
	case strings.Contains(lower, "no"),
		strings.Contains(lower, "不是"),
		strings.Contains(lower, "没有"),
		strings.HasPrefix(lower, "否"):
		return false, nil
	case strings.Contains(lower, "yes"),
		strings.Contains(lower, "是"),
		strings.Contains(lower, "发生"):
		return true, nil
	}
	// An ambiguous reply counts as not satisfied: a false positive would stop
	// the story from ever being pulled back toward that anchor.
	return false, nil
}
