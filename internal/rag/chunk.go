// Package rag turns cached chapter blocks into retrieval units and ranks them
// without any external dependency.
//
// A book is a small, closed corpus: a few hundred chunks. That makes an
// in-memory index cheaper and more predictable than any vector store, and it
// keeps the full text on the machine. Block offsets are carried through the
// whole pipeline so every answer can jump back to the exact paragraph in the
// reader instead of quoting text the user cannot locate.
package rag

import (
	"strings"

	"my-weread-desk/internal/weread"
)

// Options controls chunking.
type Options struct {
	// TargetChars is the soft upper bound of a chunk, measured in runes.
	TargetChars int
	// OverlapBlocks repeats this many trailing blocks at the start of the next
	// chunk, so a passage split across a boundary stays retrievable.
	OverlapBlocks int
	// MinChars merges a trailing chunk shorter than this into the previous one
	// rather than emitting a fragment. It never drops text.
	MinChars int
}

// DefaultOptions suits Chinese prose: roughly 500 characters per chunk with one
// block of overlap.
func DefaultOptions() Options {
	return Options{TargetChars: 500, OverlapBlocks: 1, MinChars: 80}
}

// Chunk is one retrieval unit. BlockStart and BlockEnd are inclusive indexes
// into the chapter's Block slice; that pair is what lets a citation reopen the
// reader at the right paragraph.
type Chunk struct {
	BookID       string `json:"bookId"`
	Seq          int    `json:"seq"`
	ChapterUID   int64  `json:"chapterUid"`
	ChapterIdx   int    `json:"chapterIdx"`
	ChapterTitle string `json:"chapterTitle"`
	BlockStart   int    `json:"blockStart"`
	BlockEnd     int    `json:"blockEnd"`
	CharCount    int    `json:"charCount"`
	Text         string `json:"text"`
}

// ChunkBook chunks every chapter in reading order and assigns a book-wide
// sequence number, so a neighbour lookup never has to cross-reference chapters.
func ChunkBook(bookID string, chapters []weread.Chapter, blocks map[int64][]weread.Block, opt Options) []Chunk {
	if opt.TargetChars <= 0 {
		opt = DefaultOptions()
	}
	var out []Chunk
	for _, ch := range chapters {
		body, ok := blocks[ch.UID]
		if !ok {
			continue
		}
		part := ChunkChapter(bookID, ch.UID, ch.Index, ch.Title, body, opt)
		for i := range part {
			part[i].Seq = len(out)
			out = append(out, part[i])
		}
	}
	return out
}

// ChunkChapter splits one chapter's blocks. Images, dividers and the
// boilerplate lines WeRead injects are skipped: they carry no retrievable text
// and would otherwise pollute short queries.
func ChunkChapter(bookID string, chapterUID int64, chapterIdx int, chapterTitle string, blocks []weread.Block, opt Options) []Chunk {
	if opt.TargetChars <= 0 {
		opt = DefaultOptions()
	}
	units := collectUnits(blocks)
	if len(units) == 0 {
		return nil
	}

	var out []Chunk
	start := 0
	for start < len(units) {
		end := start
		size := 0
		for end < len(units) {
			add := runeLen(units[end].text) + 2
			if end > start && size+add > opt.TargetChars {
				break
			}
			size += add
			end++
		}

		text := joinUnits(units[start:end])
		n := runeLen(text)
		if n < opt.MinChars && len(out) > 0 {
			// Only a trailing fragment can be this small, because every earlier
			// chunk stopped only after crossing TargetChars. Fold it back so no
			// text is lost and no one-line chunk pollutes the ranking.
			prev := &out[len(out)-1]
			prev.Text += "\n" + text
			prev.BlockEnd = units[end-1].idx
			prev.CharCount += n
		} else {
			out = append(out, Chunk{
				BookID:       bookID,
				ChapterUID:   chapterUID,
				ChapterIdx:   chapterIdx,
				ChapterTitle: chapterTitle,
				BlockStart:   units[start].idx,
				BlockEnd:     units[end-1].idx,
				CharCount:    n,
				Text:         text,
			})
		}

		if end >= len(units) {
			break
		}
		next := end - opt.OverlapBlocks
		if next <= start {
			next = start + 1
		}
		start = next
	}
	return out
}

type unit struct {
	idx  int
	text string
}

func collectUnits(blocks []weread.Block) []unit {
	out := make([]unit, 0, len(blocks))
	for i, b := range blocks {
		switch b.Kind {
		case "heading", "para", "quote", "pre":
			text := strings.TrimSpace(b.Text)
			if text == "" || isBoilerplate(text) {
				continue
			}
			out = append(out, unit{idx: i, text: text})
		}
	}
	return out
}

// boilerplateMarkers are lines WeRead injects into chapter bodies. They are not
// part of the book and answering with them would look like a hallucination, so
// they are removed before indexing.
var boilerplateMarkers = []string{
	"微信读书", "本书由", "版权所有", "扫码阅读", "关注公众号",
	"试读结束", "购买本书", "继续阅读请", "章节内容加载失败",
}

func isBoilerplate(s string) bool {
	if runeLen(s) > 48 {
		return false
	}
	for _, marker := range boilerplateMarkers {
		if strings.Contains(s, marker) {
			return true
		}
	}
	return false
}

func joinUnits(units []unit) string {
	if len(units) == 0 {
		return ""
	}
	var b strings.Builder
	for i, u := range units {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(u.text)
	}
	return b.String()
}

func runeLen(s string) int { return len([]rune(s)) }
