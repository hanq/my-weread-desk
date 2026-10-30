package story

import (
	"strings"

	"my-weread-desk/internal/weread"
)

// Scene is one node in the timeline.
//
// BlockStart/BlockEnd use the same block indexes as the reader and the RAG
// chunks, so a scene can open the exact passage it was extracted from.
type Scene struct {
	BookID       string   `json:"bookId"`
	Seq          int      `json:"seq"`
	ChapterUID   int64    `json:"chapterUid"`
	ChapterIdx   int      `json:"chapterIdx"`
	ChapterTitle string   `json:"chapterTitle"`
	BlockStart   int      `json:"blockStart"`
	BlockEnd     int      `json:"blockEnd"`
	Location     string   `json:"location"`
	TimeHint     string   `json:"timeHint"`
	POV          string   `json:"pov"`
	Mood         string   `json:"mood"`
	Summary      string   `json:"summary"`
	Participants []string `json:"participants"`
	Source       string   `json:"source"`
}

// ChapterScene builds the deterministic scene for one chapter.
//
// It needs no model, which matters twice: the timeline can render before any AI
// is configured, and a failed enrichment degrades to this shape instead of
// dropping the chapter from the graph.
func ChapterScene(bookID string, ch weread.Chapter, blocks []weread.Block) Scene {
	sc := Scene{
		BookID:       bookID,
		ChapterUID:   ch.UID,
		ChapterIdx:   ch.Index,
		ChapterTitle: ch.Title,
		BlockStart:   0,
		BlockEnd:     len(blocks) - 1,
		Summary:      firstRunes(PlainText(blocks), 80),
		Participants: []string{},
		Source:       "chapter",
	}
	if sc.BlockEnd < 0 {
		sc.BlockEnd = 0
	}
	return sc
}

// PlainText flattens the renderable blocks into text suitable for a prompt.
// Images and dividers are skipped because they carry no narrative content.
func PlainText(blocks []weread.Block) string {
	var b strings.Builder
	for _, blk := range blocks {
		switch blk.Kind {
		case "heading", "para", "quote", "pre":
			t := strings.TrimSpace(blk.Text)
			if t == "" {
				continue
			}
			if b.Len() > 0 {
				b.WriteByte('\n')
			}
			b.WriteString(t)
		}
	}
	return b.String()
}

// ApplySceneFields merges extracted labels into a scene. Empty fields keep the
// deterministic value so a partial model reply still improves the timeline.
func ApplySceneFields(sc Scene, f SceneFields) Scene {
	if s := strings.TrimSpace(f.Summary); s != "" {
		sc.Summary = s
	}
	if s := strings.TrimSpace(f.Location); s != "" {
		sc.Location = s
	}
	if s := strings.TrimSpace(f.TimeHint); s != "" {
		sc.TimeHint = s
	}
	if s := strings.TrimSpace(f.POV); s != "" {
		sc.POV = s
	}
	if s := strings.TrimSpace(f.Mood); s != "" {
		sc.Mood = s
	}
	if len(f.Participants) > 0 {
		sc.Participants = compactNames(f.Participants)
	}
	sc.Source = "llm"
	return sc
}

func compactNames(names []string) []string {
	out := make([]string, 0, len(names))
	seen := map[string]bool{}
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, name)
	}
	return out
}

func firstRunes(s string, n int) string {
	clean := strings.Join(strings.Fields(s), " ")
	runes := []rune(clean)
	if len(runes) <= n {
		return clean
	}
	return string(runes[:n]) + "…"
}
