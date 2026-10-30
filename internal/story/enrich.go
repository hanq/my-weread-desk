package story

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"my-weread-desk/internal/weread"
)

// SceneFields is the model's contribution to a scene: labels the deterministic
// pass cannot know.
type SceneFields struct {
	Summary      string   `json:"summary"`
	Location     string   `json:"location"`
	TimeHint     string   `json:"timeHint"`
	POV          string   `json:"pov"`
	Mood         string   `json:"mood"`
	Participants []string `json:"participants"`
}

// scenePromptRunes bounds the chapter text sent to the model. A chapter is
// usually well under this; the cap protects against a pathological one and
// keeps the per-chapter cost predictable.
const scenePromptRunes = 3000

// BuildScenePrompt returns plain strings instead of an ai.Message so this
// package keeps no dependency on the transport layer.
func BuildScenePrompt(bookTitle string, ch weread.Chapter, text string) (system, user string) {
	system = "你是中文小说的结构分析器。只输出一个 JSON 对象，不要输出解释，不要使用 Markdown 代码块。"

	var b strings.Builder
	fmt.Fprintf(&b, "书名：%s\n章节：%s\n\n正文：\n%s\n\n", bookTitle, chapterLabel(ch), firstRunes(text, scenePromptRunes))
	b.WriteString("请概括本章，输出以下字段：\n")
	b.WriteString("- summary：一句话说明本章发生了什么，不超过 60 字\n")
	b.WriteString("- location：主要地点，无法判断则留空\n")
	b.WriteString("- timeHint：时间提示，如“清晨”“当夜”“三日后”，无法判断则留空\n")
	b.WriteString("- pov：视角人物，无法判断则留空\n")
	b.WriteString("- mood：情绪基调，两到四个字\n")
	b.WriteString("- participants：出场人物姓名数组，不要包含代词\n")
	return system, b.String()
}

func chapterLabel(ch weread.Chapter) string {
	title := strings.TrimSpace(ch.Title)
	if title == "" {
		return fmt.Sprintf("第 %d 章", ch.Index)
	}
	return fmt.Sprintf("第 %d 章 %s", ch.Index, title)
}

// ParseSceneFields accepts the model's reply, tolerating code fences and
// leading prose. It refuses a reply with no usable field so a silently empty
// result is not written over a good deterministic scene.
func ParseSceneFields(raw string) (SceneFields, error) {
	var f SceneFields
	jsonText, err := extractJSONObject(raw)
	if err != nil {
		return f, err
	}
	if err := json.Unmarshal([]byte(jsonText), &f); err != nil {
		return f, fmt.Errorf("story: 场景 JSON 解析失败: %w", err)
	}
	f.Summary = strings.TrimSpace(f.Summary)
	f.Location = strings.TrimSpace(f.Location)
	f.TimeHint = strings.TrimSpace(f.TimeHint)
	f.POV = strings.TrimSpace(f.POV)
	f.Mood = strings.TrimSpace(f.Mood)
	f.Participants = compactNames(f.Participants)
	if f.Summary == "" && f.Location == "" && f.POV == "" && f.Mood == "" && len(f.Participants) == 0 {
		return f, errors.New("story: 场景 JSON 没有任何可用字段")
	}
	return f, nil
}

func extractJSONObject(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", errors.New("story: 模型返回为空")
	}
	if strings.Contains(raw, "```") {
		start := strings.Index(raw, "```")
		rest := raw[start+3:]
		rest = strings.TrimPrefix(rest, "json")
		if end := strings.Index(rest, "```"); end >= 0 {
			raw = strings.TrimSpace(rest[:end])
		}
	}
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start < 0 || end <= start {
		return "", errors.New("story: 模型返回中没有 JSON 对象")
	}
	return raw[start : end+1], nil
}
