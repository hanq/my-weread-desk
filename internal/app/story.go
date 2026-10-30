package app

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

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

// ---------------------------------------------------------------- anchors

// StorySteering is the read-only view of the world-will logic.
type StorySteering struct {
	Gravity    float64         `json:"gravity"`
	Divergence float64         `json:"divergence"`
	Pressure   *story.Pressure `json:"pressure,omitempty"`
}

// StoryAnchors returns the canon anchors of a book.
func (a *App) StoryAnchors(bookID string) ([]story.CanonAnchor, error) {
	anchors, err := a.store.Anchors(bookID)
	if err != nil {
		return nil, err
	}
	if anchors == nil {
		return []story.CanonAnchor{}, nil
	}
	return anchors, nil
}

// SaveAnchor creates or updates one anchor and returns the new set.
func (a *App) SaveAnchor(anchor story.CanonAnchor) ([]story.CanonAnchor, error) {
	anchor.BookID = strings.TrimSpace(anchor.BookID)
	if anchor.BookID == "" {
		return nil, errors.New("缺少书籍 ID")
	}
	anchor.Summary = strings.TrimSpace(anchor.Summary)
	if anchor.Summary == "" {
		return nil, errors.New("锚点描述不能为空")
	}
	if anchor.SceneSeq < 0 {
		anchor.SceneSeq = 0
	}
	if anchor.DeadlineSeq < anchor.SceneSeq {
		anchor.DeadlineSeq = anchor.SceneSeq
	}
	if anchor.Weight <= 0 || anchor.Weight > 1 {
		anchor.Weight = 0.8
	}
	if anchor.Kind == "" {
		anchor.Kind = story.AnchorBeat
	}
	if anchor.ID == "" {
		anchor.ID = fmt.Sprintf("anchor-%d", time.Now().UnixNano())
	}
	if anchor.Source == "" {
		anchor.Source = "manual"
	}
	if err := a.store.UpsertAnchor(anchor); err != nil {
		return nil, err
	}
	return a.StoryAnchors(anchor.BookID)
}

// DeleteAnchor removes one anchor and returns the remaining set.
func (a *App) DeleteAnchor(bookID, anchorID string) ([]story.CanonAnchor, error) {
	if err := a.store.DeleteAnchor(bookID, anchorID); err != nil {
		return nil, err
	}
	return a.StoryAnchors(bookID)
}

// SuggestAnchors asks the model for candidate anchors.
//
// Candidates are returned unsaved. An anchor steers the story, so adopting one
// has to be an explicit act; silently writing model guesses into the canon
// would let the model decide what the book is about.
func (a *App) SuggestAnchors(bookID string) ([]story.CanonAnchor, error) {
	bookID = strings.TrimSpace(bookID)
	if bookID == "" {
		return nil, errors.New("缺少书籍 ID")
	}
	if !a.aiReady() {
		return nil, errors.New("需要先启用 AI 并确认正文片段发送")
	}
	scenes, err := a.store.StoryScenes(bookID)
	if err != nil {
		return nil, err
	}
	if len(scenes) == 0 {
		return nil, errors.New("请先建立时间线")
	}

	cfg := a.settings.Get().AI
	title := bookID
	if book, berr := a.store.Book(bookID); berr == nil && book != nil && book.Title != "" {
		title = book.Title
	}
	system, user := story.BuildAnchorPrompt(title, scenes)

	provider := ai.NewOpenAI(ai.Config{
		BaseURL: cfg.BaseURL, APIKey: cfg.APIKey, Model: cfg.Model,
		Temperature: cfg.Temperature, MaxTokens: 800,
	})
	ctx, cancel := context.WithTimeout(a.ctxOrBackground(), 90*time.Second)
	defer cancel()
	answer, err := provider.Chat(ctx, ai.Request{
		Model: cfg.Model,
		Messages: []ai.Message{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
		Temperature: 0.2,
		MaxTokens:   800,
	}, nil)
	if err != nil {
		return nil, err
	}
	fields, err := story.ParseAnchors(answer)
	if err != nil {
		return nil, err
	}

	seqs := make(map[int]bool, len(scenes))
	for _, sc := range scenes {
		seqs[sc.Seq] = true
	}
	out := make([]story.CanonAnchor, 0, len(fields))
	for i, f := range fields {
		if !seqs[f.SceneSeq] {
			continue
		}
		out = append(out, story.CanonAnchor{
			ID:          fmt.Sprintf("llm-%d-%d", time.Now().UnixNano(), i),
			BookID:      bookID,
			SceneSeq:    f.SceneSeq,
			DeadlineSeq: f.SceneSeq + 2,
			Kind:        f.Kind,
			Summary:     f.Summary,
			Weight:      f.Weight,
			Source:      "llm",
		})
	}
	if len(out) == 0 {
		return nil, errors.New("模型给出的锚点都不在时间线内")
	}
	return out, nil
}

// StorySteering reports gravity, divergence and the anchor that would be pushed
// at a given scene. No branch exists yet; this exposes the logic for inspection.
func (a *App) StorySteering(bookID string, currentSeq int) (StorySteering, error) {
	anchors, err := a.store.Anchors(bookID)
	if err != nil {
		return StorySteering{}, err
	}
	diverge := story.Divergence(currentSeq, anchors, nil, 0, 0)
	gravity := story.Gravity(currentSeq, anchors, nil)
	return StorySteering{
		Gravity:    gravity,
		Divergence: diverge,
		Pressure:   story.PlanPressure(currentSeq, anchors, nil, diverge),
	}, nil
}
