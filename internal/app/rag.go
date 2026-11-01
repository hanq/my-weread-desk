package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"my-weread-desk/internal/ai"
	"my-weread-desk/internal/config"
	"my-weread-desk/internal/library"
	"my-weread-desk/internal/rag"
	"my-weread-desk/internal/weread"
)

// RAGStatus is the indexing state of one book as the UI sees it.
type RAGStatus struct {
	BookID     string `json:"bookId"`
	Status     string `json:"status"`
	Indexed    bool   `json:"indexed"`
	ChunkCount int    `json:"chunkCount"`
	CharCount  int    `json:"charCount"`
	Error      string `json:"error"`
	UpdatedAt  int64  `json:"updatedAt"`
}

// AskEvent is one streaming update for a question. Delta carries appended text,
// Text carries the final answer, and exactly one of Error/Text is set on the
// last event of a request.
type AskEvent struct {
	RequestID string      `json:"requestId"`
	Delta     string      `json:"delta,omitempty"`
	Text      string      `json:"text,omitempty"`
	Sources   []ai.Source `json:"sources,omitempty"`
	Model     string      `json:"model,omitempty"`
	Error     string      `json:"error,omitempty"`
}

var askSeq int64

// IndexBook builds the retrieval index from chapter bodies already on disk.
//
// Nothing is fetched: a book has to be downloaded first. That is deliberate —
// indexing must never trigger the request throttling that chapter downloading
// already needs, and it keeps re-indexing free and repeatable.
func (a *App) IndexBook(bookID string) (RAGStatus, error) {
	bookID = strings.TrimSpace(bookID)
	if bookID == "" {
		return RAGStatus{}, errors.New("缺少书籍 ID")
	}
	_ = a.store.SetRAGStatus(bookID, "indexing", "")

	chapters, err := a.store.Chapters(bookID)
	if err != nil {
		_ = a.store.SetRAGStatus(bookID, "failed", err.Error())
		return RAGStatus{}, err
	}
	if len(chapters) == 0 {
		err = errors.New("没有章节目录，请先打开或下载本书")
		_ = a.store.SetRAGStatus(bookID, "failed", err.Error())
		return RAGStatus{}, err
	}

	blocks := make(map[int64][]weread.Block, len(chapters))
	for _, ch := range chapters {
		content, cerr := a.store.ChapterContent(bookID, ch.UID)
		if cerr != nil || len(content.Blocks) == 0 {
			continue
		}
		blocks[ch.UID] = content.Blocks
	}
	if len(blocks) == 0 {
		err = errors.New("没有已缓存的章节正文，请先下载本书")
		_ = a.store.SetRAGStatus(bookID, "failed", err.Error())
		return RAGStatus{}, err
	}

	chunks := rag.ChunkBook(bookID, chapters, blocks, rag.DefaultOptions())
	if len(chunks) == 0 {
		err = errors.New("章节正文为空，无法建立索引")
		_ = a.store.SetRAGStatus(bookID, "failed", err.Error())
		return RAGStatus{}, err
	}
	if err := a.store.ReplaceRAGChunks(bookID, chunks); err != nil {
		_ = a.store.SetRAGStatus(bookID, "failed", err.Error())
		return RAGStatus{}, err
	}
	a.ragIndex.Invalidate(bookID)
	a.logf("索引完成：book=%s 章节=%d 块=%d", bookID, len(blocks), len(chunks))
	return a.RAGStatus(bookID), nil
}

// RAGStatus reports the index state of a book without touching the network.
func (a *App) RAGStatus(bookID string) RAGStatus {
	meta, err := a.store.RAGIndex(bookID)
	if err != nil || meta == nil {
		return RAGStatus{BookID: bookID, Status: "none"}
	}
	status := meta.Status
	errText := meta.Error
	if status == "ready" && meta.ChunkerVersion != library.RAGChunkerVersion {
		status = "stale"
		errText = "分块版本已更新，请重建索引"
	}
	return RAGStatus{
		BookID:     meta.BookID,
		Status:     status,
		Indexed:    status == "ready" && meta.ChunkCount > 0,
		ChunkCount: meta.ChunkCount,
		CharCount:  meta.CharCount,
		Error:      errText,
		UpdatedAt:  meta.UpdatedAt,
	}
}

// DropRAGIndex removes a book's index.
func (a *App) DropRAGIndex(bookID string) error {
	if err := a.store.DropRAGIndex(bookID); err != nil {
		return err
	}
	a.ragIndex.Invalidate(bookID)
	return nil
}

// AskBook starts a grounded question and streams the answer through the
// "ai:ask" event. It returns immediately with a request id, so the UI never
// blocks on a model call.
func (a *App) AskBook(bookID, question string) (string, error) {
	bookID = strings.TrimSpace(bookID)
	question = strings.TrimSpace(question)
	if bookID == "" {
		return "", errors.New("请先选择一本书")
	}
	if question == "" {
		return "", errors.New("问题不能为空")
	}

	settings := a.settings.Get()
	if !settings.AI.Enabled {
		return "", errors.New("AI 助手未启用")
	}
	if !settings.AI.Consent {
		return "", errors.New("请先确认：检索到的正文片段会发送到你配置的 AI 服务")
	}
	if strings.TrimSpace(settings.AI.BaseURL) == "" || strings.TrimSpace(settings.AI.Model) == "" {
		return "", errors.New("请先填写 AI 服务地址与模型")
	}
	if err := a.requireTextIndex(bookID); err != nil {
		return "", err
	}

	id := fmt.Sprintf("ask-%d", atomic.AddInt64(&askSeq, 1))
	ctx, cancel := context.WithCancel(a.ctxOrBackground())
	a.askMu.Lock()
	a.asks[id] = cancel
	a.askMu.Unlock()

	go func() {
		defer func() {
			a.askMu.Lock()
			delete(a.asks, id)
			a.askMu.Unlock()
			cancel()
		}()
		a.runAsk(ctx, id, bookID, question, settings.AI)
	}()
	return id, nil
}

// CancelAsk stops a streaming answer. The partial text already emitted stays on
// screen; the event stream simply ends.
func (a *App) CancelAsk(requestID string) {
	a.askMu.Lock()
	cancel := a.asks[requestID]
	a.askMu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (a *App) runAsk(ctx context.Context, id, bookID, question string, cfg config.AISettings) {
	emit := func(ev AskEvent) {
		if a.ctx != nil {
			runtime.EventsEmit(a.ctx, "ai:ask", ev)
		}
	}

	index, err := a.ragIndex.Index(bookID)
	if err != nil {
		emit(AskEvent{RequestID: id, Error: err.Error()})
		return
	}
	hits := index.Search(question, 8)
	if len(hits) == 0 {
		emit(AskEvent{RequestID: id, Text: "书中没有检索到与这个问题相关的内容。", Sources: []ai.Source{}})
		return
	}
	hits = expandHits(index, hits)

	title := bookID
	if book, berr := a.store.Book(bookID); berr == nil && book != nil && book.Title != "" {
		title = book.Title
	}

	provider := ai.NewOpenAI(ai.Config{
		BaseURL:     cfg.BaseURL,
		APIKey:      cfg.APIKey,
		Model:       cfg.Model,
		Temperature: cfg.Temperature,
		MaxTokens:   cfg.MaxTokens,
	})
	messages := ai.BuildMessages(ai.AskInput{BookTitle: title, Question: question, Hits: hits})
	answer, err := provider.Chat(ctx, ai.Request{
		Model:       cfg.Model,
		Messages:    messages,
		Temperature: cfg.Temperature,
		MaxTokens:   cfg.MaxTokens,
	}, func(delta string) {
		emit(AskEvent{RequestID: id, Delta: delta})
	})
	if err != nil {
		// A cancelled request is not an error the user needs to see.
		if ctx.Err() != nil {
			return
		}
		emit(AskEvent{RequestID: id, Error: err.Error()})
		return
	}
	emit(AskEvent{RequestID: id, Text: answer, Model: cfg.Model, Sources: ai.Sources(hits)})
}

// expandHits appends the chunk that continues a hit inside the same chapter.
// A retrieved passage often stops mid-scene; the following paragraph is already
// in memory, so continuity costs one lookup rather than a second retrieval.
// The block span is widened so the citation still points at everything quoted.
func expandHits(index *rag.Index, hits []rag.Hit) []rag.Hit {
	out := make([]rag.Hit, 0, len(hits))
	for _, hit := range hits {
		next, ok := index.BySeq(hit.Chunk.Seq + 1)
		if ok && next.ChapterUID == hit.Chunk.ChapterUID && next.BlockStart <= hit.Chunk.BlockEnd+1 {
			hit.Chunk.Text += "\n" + next.Text
			hit.Chunk.BlockEnd = next.BlockEnd
			hit.Chunk.CharCount += next.CharCount
		}
		out = append(out, hit)
	}
	return out
}
