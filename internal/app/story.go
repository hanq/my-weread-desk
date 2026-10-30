package app

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"sync"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"my-weread-desk/internal/ai"
	"my-weread-desk/internal/library"
	"my-weread-desk/internal/story"
	"my-weread-desk/internal/weread"
)

// StoryGraphStatus is the extraction state of one book.
type StoryGraphStatus struct {
	BookID     string `json:"bookId"`
	Status     string `json:"status"`
	Total      int    `json:"total"`
	Done       int    `json:"done"`
	Failed     int    `json:"failed"`
	Error      string `json:"error"`
	SceneCount int    `json:"sceneCount"`
	UpdatedAt  int64  `json:"updatedAt"`
}

// storyBuilder runs the scene-extraction job.
//
// The job is resumable because progress lives in story_job_item, not in this
// struct: closing the app mid-book must not discard chapters that already cost
// model calls.
type storyBuilder struct {
	app *App

	mu      sync.Mutex
	running map[string]bool
	cancel  map[string]context.CancelFunc
}

func newStoryBuilder(a *App) *storyBuilder {
	return &storyBuilder{
		app:     a,
		running: map[string]bool{},
		cancel:  map[string]context.CancelFunc{},
	}
}

func (b *storyBuilder) status(bookID string) StoryGraphStatus {
	job, err := b.app.store.StoryJob(bookID, library.StoryJobSceneExtract)
	if err != nil {
		return StoryGraphStatus{BookID: bookID, Status: "none"}
	}
	count, _ := b.app.store.SceneCount(bookID)
	return StoryGraphStatus{
		BookID:     bookID,
		Status:     job.Status,
		Total:      job.Total,
		Done:       job.Done,
		Failed:     job.Failed,
		Error:      job.Error,
		SceneCount: count,
		UpdatedAt:  job.UpdatedAt,
	}
}

func (b *storyBuilder) start(bookID string) (StoryGraphStatus, error) {
	bookID = strings.TrimSpace(bookID)
	if bookID == "" {
		return StoryGraphStatus{}, errors.New("缺少书籍 ID")
	}

	b.mu.Lock()
	if b.running[bookID] {
		b.mu.Unlock()
		return b.status(bookID), nil
	}
	b.mu.Unlock()

	chapters, err := b.app.store.Chapters(bookID)
	if err != nil {
		return StoryGraphStatus{}, err
	}
	blocks := make(map[int64][]weread.Block, len(chapters))
	keys := make([]string, 0, len(chapters))
	for _, ch := range chapters {
		content, cerr := b.app.store.ChapterContent(bookID, ch.UID)
		if cerr != nil || len(content.Blocks) == 0 {
			continue
		}
		blocks[ch.UID] = content.Blocks
		keys = append(keys, strconv.FormatInt(ch.UID, 10))
	}
	if len(keys) == 0 {
		return StoryGraphStatus{}, errors.New("没有已缓存的章节正文，请先下载本书")
	}

	if _, err := b.app.store.InitStoryJob(bookID, library.StoryJobSceneExtract, keys); err != nil {
		return StoryGraphStatus{}, err
	}
	if err := b.app.store.SetStoryJobStatus(bookID, library.StoryJobSceneExtract, "running", ""); err != nil {
		return StoryGraphStatus{}, err
	}

	ctx, cancel := context.WithCancel(b.app.ctxOrBackground())
	b.mu.Lock()
	b.running[bookID] = true
	b.cancel[bookID] = cancel
	b.mu.Unlock()

	go func() {
		defer func() {
			b.mu.Lock()
			delete(b.running, bookID)
			delete(b.cancel, bookID)
			b.mu.Unlock()
			cancel()
		}()
		b.run(ctx, bookID, chapters, blocks)
	}()

	return b.status(bookID), nil
}

func (b *storyBuilder) run(ctx context.Context, bookID string, chapters []weread.Chapter, blocks map[int64][]weread.Block) {
	const job = library.StoryJobSceneExtract

	pending, err := b.app.store.PendingStoryItems(bookID, job)
	if err != nil {
		_ = b.app.store.SetStoryJobStatus(bookID, job, "failed", err.Error())
		b.emit(bookID)
		return
	}

	seqOf := make(map[int64]int, len(chapters))
	byUID := make(map[int64]weread.Chapter, len(chapters))
	for i, ch := range chapters {
		seqOf[ch.UID] = i
		byUID[ch.UID] = ch
	}

	useAI := b.app.aiReady()
	title := bookID
	if book, berr := b.app.store.Book(bookID); berr == nil && book != nil && book.Title != "" {
		title = book.Title
	}

	for _, key := range pending {
		if ctx.Err() != nil {
			_ = b.app.store.SetStoryJobStatus(bookID, job, "cancelled", "")
			b.emit(bookID)
			return
		}
		uid, perr := strconv.ParseInt(key, 10, 64)
		ch, ok := byUID[uid]
		if perr != nil || !ok {
			_ = b.app.store.MarkStoryItem(bookID, job, key, "failed", "章节不存在")
			continue
		}

		scene := story.ChapterScene(bookID, ch, blocks[uid])
		scene.Seq = seqOf[uid]

		warning := ""
		if useAI {
			fields, ferr := b.enrich(ctx, title, ch, blocks[uid])
			switch {
			case ferr != nil && ctx.Err() != nil:
				_ = b.app.store.SetStoryJobStatus(bookID, job, "cancelled", "")
				b.emit(bookID)
				return
			case ferr != nil:
				// A failed label must not cost the chapter: the deterministic
				// scene still carries a usable summary and block anchor.
				warning = "AI 标注失败：" + ferr.Error()
			default:
				scene = story.ApplySceneFields(scene, fields)
			}
		}

		if err := b.app.store.UpsertStoryScene(scene); err != nil {
			_ = b.app.store.MarkStoryItem(bookID, job, key, "failed", err.Error())
		} else {
			_ = b.app.store.MarkStoryItem(bookID, job, key, "done", warning)
		}
		b.emit(bookID)
	}

	final := "done"
	if st := b.status(bookID); st.Done == 0 && st.Failed > 0 {
		final = "failed"
	}
	_ = b.app.store.SetStoryJobStatus(bookID, job, final, "")
	b.emit(bookID)
}

// enrich asks the model for the labels the deterministic pass cannot produce.
// Failure is non-fatal by design: the chapter keeps its rule-based scene.
func (b *storyBuilder) enrich(ctx context.Context, bookTitle string, ch weread.Chapter, blocks []weread.Block) (story.SceneFields, error) {
	cfg := b.app.settings.Get().AI
	provider := ai.NewOpenAI(ai.Config{
		BaseURL:     cfg.BaseURL,
		APIKey:      cfg.APIKey,
		Model:       cfg.Model,
		Temperature: cfg.Temperature,
		MaxTokens:   400,
	})
	system, user := story.BuildScenePrompt(bookTitle, ch, story.PlainText(blocks))
	answer, err := provider.Chat(ctx, ai.Request{
		Model: cfg.Model,
		Messages: []ai.Message{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
		Temperature: 0.2,
		MaxTokens:   400,
	}, nil)
	if err != nil {
		return story.SceneFields{}, err
	}
	return story.ParseSceneFields(answer)
}

func (b *storyBuilder) emit(bookID string) {
	if b.app.ctx == nil {
		return
	}
	runtime.EventsEmit(b.app.ctx, "story:progress", b.status(bookID))
}

func (b *storyBuilder) cancelRun(bookID string) {
	b.mu.Lock()
	cancel := b.cancel[bookID]
	b.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// ---------------------------------------------------------------- bindings

// BuildStoryGraph starts (or resumes) scene extraction. It returns immediately;
// progress arrives through the "story:progress" event.
func (a *App) BuildStoryGraph(bookID string) (StoryGraphStatus, error) {
	return a.storyIndex.start(bookID)
}

// StoryGraphStatus reports extraction progress without touching the network.
func (a *App) StoryGraphStatus(bookID string) StoryGraphStatus {
	return a.storyIndex.status(bookID)
}

// CancelStoryGraph stops a running extraction. Finished chapters are kept.
func (a *App) CancelStoryGraph(bookID string) {
	a.storyIndex.cancelRun(bookID)
}

// StoryTimeline returns the scenes in reading order.
func (a *App) StoryTimeline(bookID string) ([]story.Scene, error) {
	scenes, err := a.store.StoryScenes(bookID)
	if err != nil {
		return nil, err
	}
	if scenes == nil {
		return []story.Scene{}, nil
	}
	return scenes, nil
}

// DropStoryGraph removes the graph and its job state.
func (a *App) DropStoryGraph(bookID string) error {
	a.storyIndex.cancelRun(bookID)
	return a.store.DropStoryGraph(bookID)
}

// aiReady reports whether enrichment may send chapter text to a provider.
// Consent is required: the text is copyrighted and leaves the machine.
func (a *App) aiReady() bool {
	cfg := a.settings.Get().AI
	return cfg.Enabled && cfg.Consent &&
		strings.TrimSpace(cfg.BaseURL) != "" && strings.TrimSpace(cfg.Model) != ""
}
