package story

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// Choice is one decision offered at the end of a scene.
//
// Choices are what turn a branch from "the reader typed an instruction" into a
// playable scene: the model proposes concrete options that follow from what just
// happened, and picking one becomes the intent for the next scene.
type Choice struct {
	ID   string `json:"id"`
	Text string `json:"text"`
	Hint string `json:"hint,omitempty"`
}

// maxChoices caps the list. More than three stops being a choice and starts
// being a menu.
const maxChoices = 3

// BuildChoicesPrompt asks for concrete options, explicitly forbidding outcomes.
// Asking for outcomes is how a model starts summarising the next scene instead
// of leaving the decision open.
func BuildChoicesPrompt(bookTitle, sceneText string) (system, user string) {
	system = "你是小说编辑。只输出一个 JSON 数组，不要解释，不要使用 Markdown 代码块。"

	var b strings.Builder
	fmt.Fprintf(&b, "书名：%s\n\n本幕正文：\n%s\n\n", bookTitle, firstRunes(sceneText, 1200))
	b.WriteString("这一幕结束时，主角面临哪些具体选择？给出 2 到 3 个。\n")
	b.WriteString("每个不超过 20 字，只写选择本身，不要写结果，不要写心理活动。\n")
	b.WriteString(`只输出：[{"text":"..."},{"text":"..."}]`)
	return system, b.String()
}

// ParseChoices accepts the model reply, tolerating fences and duplicates.
func ParseChoices(raw string) ([]Choice, error) {
	start := strings.Index(raw, "[")
	end := strings.LastIndex(raw, "]")
	if start < 0 || end <= start {
		return nil, errors.New("story: 模型返回中没有选项数组")
	}

	var items []struct {
		Text string `json:"text"`
		Hint string `json:"hint"`
	}
	if err := json.Unmarshal([]byte(raw[start:end+1]), &items); err != nil {
		return nil, fmt.Errorf("story: 选项 JSON 解析失败: %w", err)
	}

	out := make([]Choice, 0, maxChoices)
	seen := map[string]bool{}
	for _, item := range items {
		text := strings.TrimSpace(item.Text)
		if text == "" || seen[text] {
			continue
		}
		seen[text] = true
		out = append(out, Choice{
			ID:   fmt.Sprintf("c%d", len(out)+1),
			Text: text,
			Hint: strings.TrimSpace(item.Hint),
		})
		if len(out) == maxChoices {
			break
		}
	}
	if len(out) == 0 {
		return nil, errors.New("story: 选项数组为空")
	}
	return out, nil
}
