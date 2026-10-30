package ai

import (
	"fmt"
	"strings"

	"my-weread-desk/internal/rag"
)

// Source is one citation offered to the UI. BlockStart/BlockEnd let the reader
// open the exact paragraph instead of showing a quote the user cannot locate.
type Source struct {
	Index        int    `json:"index"`
	BookID       string `json:"bookId"`
	ChapterUID   int64  `json:"chapterUid"`
	ChapterIdx   int    `json:"chapterIdx"`
	ChapterTitle string `json:"chapterTitle"`
	BlockStart   int    `json:"blockStart"`
	BlockEnd     int    `json:"blockEnd"`
	Excerpt      string `json:"excerpt"`
}

// AskInput is everything the prompt needs.
type AskInput struct {
	BookTitle string
	Question  string
	Hits      []rag.Hit
}

// excerptRunes caps one passage. The chunker already targets ~500 characters,
// so this only trims pathological blocks and keeps the prompt predictable.
const excerptRunes = 700

// BuildMessages assembles a grounded prompt.
//
// The three rules that matter: answer only from the passages, cite the numbered
// passage in place, and admit when the book does not cover the question. Without
// the third rule the model fills gaps from its own training data, which is
// exactly the failure a reader cannot detect.
func BuildMessages(in AskInput) []Message {
	title := strings.TrimSpace(in.BookTitle)
	if title == "" {
		title = "这本书"
	}
	system := fmt.Sprintf(`你是《%s》的阅读助手。

规则：
1. 只依据下面提供的原文片段回答，不得补充书外知识，也不得改写事实。
2. 每个片段前有编号 [n]。引用原文时，在句子末尾标注对应编号，例如 [2]。
3. 如果片段不足以回答，直接说明「书中这部分没有提到」，不要猜测。
4. 用简洁的中文回答，不要整段复述原文，也不要编造页码或章节号。`, title)

	var b strings.Builder
	b.WriteString("原文片段：\n")
	for i, hit := range in.Hits {
		c := hit.Chunk
		chapter := strings.TrimSpace(c.ChapterTitle)
		if chapter == "" {
			chapter = fmt.Sprintf("第 %d 章", c.ChapterIdx)
		}
		fmt.Fprintf(&b, "\n[%d] 《%s》%s\n%s\n", i+1, title, chapter, excerpt(c.Text))
	}
	fmt.Fprintf(&b, "\n问题：%s", strings.TrimSpace(in.Question))

	return []Message{
		{Role: "system", Content: system},
		{Role: "user", Content: b.String()},
	}
}

// Sources turns hits into citations. The excerpt is deliberately short: the UI
// shows enough to recognise the passage, and the block span is the real payload.
func Sources(hits []rag.Hit) []Source {
	out := make([]Source, 0, len(hits))
	for i, hit := range hits {
		c := hit.Chunk
		out = append(out, Source{
			Index:        i + 1,
			BookID:       c.BookID,
			ChapterUID:   c.ChapterUID,
			ChapterIdx:   c.ChapterIdx,
			ChapterTitle: c.ChapterTitle,
			BlockStart:   c.BlockStart,
			BlockEnd:     c.BlockEnd,
			Excerpt:      excerpt(c.Text),
		})
	}
	return out
}

func excerpt(text string) string {
	clean := strings.Join(strings.Fields(text), " ")
	runes := []rune(clean)
	if len(runes) <= excerptRunes {
		return clean
	}
	return string(runes[:excerptRunes]) + "…"
}
