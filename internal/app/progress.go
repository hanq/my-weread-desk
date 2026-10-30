package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"my-weread-desk/internal/library"
	"my-weread-desk/internal/weread"
	"my-weread-desk/internal/weread/protocol"
)

// reporter owns read-time and progress reporting. Reports are throttled,
// sequential and only sent while the reader has been active recently, so the
// app never looks like a background bot.
type reporter struct {
	app *App

	mu             sync.Mutex
	pending        map[string]Position
	stateAt        map[string]*weread.ReaderState
	lastReport     map[string]time.Time
	lastActivity   time.Time
	lastLocalTick  time.Time
	lastActiveBook string

	stopCh chan struct{}
	wg     sync.WaitGroup
}

func newReporter(a *App) *reporter {
	return &reporter{
		app:          a,
		pending:      map[string]Position{},
		stateAt:      map[string]*weread.ReaderState{},
		lastReport:   map[string]time.Time{},
		lastActivity: time.Time{},
		stopCh:       make(chan struct{}),
	}
}

func (r *reporter) start() {
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-r.stopCh:
				return
			case <-ticker.C:
				r.tick()
			}
		}
	}()
}

func (r *reporter) stop() {
	close(r.stopCh)
	r.wg.Wait()
}

func (r *reporter) touch(p Position) {
	r.mu.Lock()
	defer r.mu.Unlock()
	// Seed the report clock on the first position update. Without this the
	// first report would see a zero time and claim the full 180s cap.
	if _, ok := r.pending[p.BookID]; !ok {
		if _, reported := r.lastReport[p.BookID]; !reported {
			r.lastReport[p.BookID] = time.Now()
		}
	}
	r.pending[p.BookID] = p
	r.lastActivity = time.Now()
	r.lastActiveBook = p.BookID
}

func (r *reporter) tick() {
	now := time.Now()
	r.tickLocal(now)
	settings := r.app.settings.Get()
	if !settings.Reader.ReportProgress && !settings.Reader.ReportReadTime {
		return
	}
	interval := time.Duration(settings.Reader.ReportInterval) * time.Second
	if interval < 15*time.Second {
		interval = 30 * time.Second
	}

	r.mu.Lock()
	if time.Since(r.lastActivity) > 3*time.Minute {
		// The user walked away: stop claiming reading time.
		r.mu.Unlock()
		return
	}
	var due []Position
	for bookID, p := range r.pending {
		if time.Since(r.lastReport[bookID]) >= interval {
			due = append(due, p)
		}
	}
	r.mu.Unlock()

	for _, p := range due {
		if err := r.report(p); err != nil {
			r.app.logf("同步阅读进度失败：%v", err)
			continue
		}
	}
}

// tickLocal accumulates the foreground reading clock into the local per-device
// table independently of whether the cloud report succeeds or is enabled.
// WeRead's stats API is account-wide, so this is the only source for
// "this device" time. The local counter is not an upload, so it must keep
// working even when the user turns off read-time reporting.
func (r *reporter) tickLocal(now time.Time) {
	r.mu.Lock()
	last := r.lastLocalTick
	r.lastLocalTick = now
	active := !r.lastActivity.IsZero() && now.Sub(r.lastActivity) <= 3*time.Minute
	r.mu.Unlock()

	if last.IsZero() || !active {
		return
	}
	delta := now.Sub(last)
	// A suspend/resume can produce a huge gap; cap it so the counter never
	// jumps by hours after the laptop wakes up.
	if delta > 10*time.Second {
		delta = 10 * time.Second
	}
	if delta < time.Second {
		return
	}
	if err := r.app.store.AddDeviceReadTime(now.Format("2006-01-02"), int64(delta/time.Second)); err != nil {
		r.app.logf("保存本设备阅读时长失败：%v", err)
	}
}

// reportNow forces an immediate report for a book.
func (r *reporter) reportNow(bookID string) error {
	r.mu.Lock()
	p, ok := r.pending[bookID]
	r.mu.Unlock()
	if !ok {
		return errors.New("没有待同步的阅读位置")
	}
	return r.report(p)
}

func (r *reporter) report(p Position) error {
	settings := r.app.settings.Get()
	if !settings.Reader.ReportProgress {
		return nil
	}
	state := r.app.cachedReaderState(p.BookID)
	if state == nil || state.PSVTS == "" {
		if fresh, err := r.app.ensureReaderStateSync(context.Background(), p.BookID, p.ChapterUID); err == nil {
			state = fresh
		}
	}
	if state == nil || state.PSVTS == "" {
		return errors.New("缺少阅读会话凭证")
	}

	r.mu.Lock()
	last := r.lastReport[p.BookID]
	if last.IsZero() {
		last = r.lastActivity
	}
	r.mu.Unlock()
	elapsed := 30
	if !last.IsZero() {
		elapsed = int(time.Since(last).Seconds())
	}
	if elapsed <= 0 {
		elapsed = 30
	}
	if elapsed > 180 {
		elapsed = 180
	}
	// Only claim reading time when the reader is actually in focus; the
	// frontend reports focus through TouchReader.
	report := protocol.ReadReport{
		BookID:         p.BookID,
		ChapterUID:     p.ChapterUID,
		ChapterIndex:   p.ChapterIndex,
		ChapterOffset:  p.Offset,
		Progress:       int(r.app.bookPercent(p.BookID, p.ChapterIndex, p.ChapterPercent)),
		Summary:        p.Summary,
		PSVTS:          state.PSVTS,
		PCLTS:          state.PCLTS,
		Token:          state.Token,
		ElapsedSeconds: elapsed,
	}
	if !settings.Reader.ReportReadTime {
		report.ElapsedSeconds = 0
	}

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	if err := r.app.web.ReportRead(ctx, report); err != nil {
		_ = r.app.store.SaveProgress(weread.Progress{
			BookID: p.BookID, ChapterUID: p.ChapterUID, ChapterIndex: p.ChapterIndex,
			Offset: p.Offset, Percent: float64(report.Progress), UpdateTime: time.Now().Unix(),
		}, true)
		return err
	}

	r.mu.Lock()
	r.lastReport[p.BookID] = time.Now()
	delete(r.pending, p.BookID)
	r.mu.Unlock()

	return r.app.store.SaveProgress(weread.Progress{
		BookID: p.BookID, ChapterUID: p.ChapterUID, ChapterIndex: p.ChapterIndex,
		Offset: p.Offset, Percent: float64(report.Progress), UpdateTime: time.Now().Unix(),
	}, false)
}

// flush reports every pending book once, used on shutdown.
func (r *reporter) flush() {
	r.mu.Lock()
	books := make([]Position, 0, len(r.pending))
	for _, p := range r.pending {
		books = append(books, p)
	}
	r.mu.Unlock()
	for _, p := range books {
		_ = r.report(p)
	}
}

// TouchReader records that the reader window is active. It is called from the
// frontend on focus and on every page turn.
func (a *App) TouchReader() {
	a.reporter.mu.Lock()
	a.reporter.lastActivity = time.Now()
	a.reporter.mu.Unlock()
}

// ---------------------------------------------------------------- downloads

// DownloadState is the progress payload for one book.
type DownloadState struct {
	BookID  string   `json:"bookId"`
	Running bool     `json:"running"`
	Total   int      `json:"total"`
	Done    int      `json:"done"`
	Failed  int      `json:"failed"`
	Current string   `json:"current"`
	Errors  []string `json:"errors"`
	Message string   `json:"message"`
}

type downloader struct {
	app    *App
	mu     sync.Mutex
	states map[string]*DownloadState
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func newDownloader(a *App) *downloader {
	return &downloader{app: a, states: map[string]*DownloadState{}}
}

func (d *downloader) stop() {
	d.mu.Lock()
	cancel := d.cancel
	d.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	d.wg.Wait()
}

func (d *downloader) State(bookID string) DownloadState {
	d.mu.Lock()
	defer d.mu.Unlock()
	if st, ok := d.states[bookID]; ok {
		return *st
	}
	return DownloadState{BookID: bookID}
}

func (d *downloader) snapshot(bookID string) *DownloadState {
	d.mu.Lock()
	defer d.mu.Unlock()
	st, ok := d.states[bookID]
	if !ok {
		return nil
	}
	cp := *st
	cp.Errors = append([]string{}, st.Errors...)
	return &cp
}

func (d *downloader) emit(st *DownloadState) {
	if d.app.ctx == nil || st == nil {
		return
	}
	runtime.EventsEmit(d.app.ctx, "download", st)
}

// StartDownload caches every chapter of a book in the background.
func (a *App) StartDownload(bookID string) (DownloadState, error) {
	return a.downloader.start(bookID)
}

func (d *downloader) start(bookID string) (DownloadState, error) {
	d.mu.Lock()
	if st, ok := d.states[bookID]; ok && st.Running {
		cp := *st
		d.mu.Unlock()
		return cp, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	d.cancel = cancel
	st := &DownloadState{BookID: bookID, Running: true, Message: "准备中"}
	d.states[bookID] = st
	d.mu.Unlock()

	d.wg.Add(1)
	go func() {
		defer d.wg.Done()
		defer cancel()
		d.run(ctx, bookID, st)
	}()

	return *st, nil
}

func (d *downloader) run(ctx context.Context, bookID string, st *DownloadState) {
	defer func() {
		d.mu.Lock()
		st.Running = false
		snapshot := *st
		snapshot.Errors = append([]string{}, st.Errors...)
		d.mu.Unlock()
		d.emit(&snapshot)
	}()

	chapters, err := d.app.store.Chapters(bookID)
	if err != nil || len(chapters) == 0 {
		chapters, err = d.app.eink.Chapters(ctx, bookID)
		if err != nil {
			d.setMessage(bookID, "读取目录失败："+err.Error())
			return
		}
		_ = d.app.store.ReplaceChapters(bookID, chapters)
	}
	cached, _ := d.app.store.CachedChapterUIDs(bookID)

	d.mu.Lock()
	st.Total = len(chapters)
	st.Done = len(cached)
	st.Message = "下载中"
	d.mu.Unlock()
	d.emit(d.snapshot(bookID))

	for _, ch := range chapters {
		if ctx.Err() != nil {
			d.setMessage(bookID, "已取消")
			return
		}
		if cached[ch.UID] {
			continue
		}
		d.setCurrent(bookID, ch.Title)

		var lastErr error
		for attempt := 0; attempt < 3; attempt++ {
			if attempt > 0 {
				if !sleepCtx(ctx, time.Duration(attempt)*2*time.Second) {
					return
				}
			}
			if _, err := d.app.fetchChapter(ctx, bookID, ch, chapters); err == nil {
				lastErr = nil
				break
			} else {
				lastErr = err
				if !retryable(err) {
					break
				}
			}
		}

		d.mu.Lock()
		if lastErr == nil {
			st.Done++
		} else {
			st.Failed++
			if len(st.Errors) < 30 {
				st.Errors = append(st.Errors, fmt.Sprintf("%s：%s", ch.Title, shorten(lastErr.Error(), 120)))
			}
		}
		snapshot := *st
		snapshot.Errors = append([]string{}, st.Errors...)
		d.mu.Unlock()
		d.emit(&snapshot)

		// A human-paced gap keeps the request rate defensible.
		if !sleepCtx(ctx, 350*time.Millisecond+time.Duration(ch.UID%400)*time.Millisecond) {
			d.setMessage(bookID, "已取消")
			return
		}
	}

	d.setMessage(bookID, "完成")
}

func (d *downloader) setCurrent(bookID, title string) {
	d.mu.Lock()
	if st, ok := d.states[bookID]; ok {
		st.Current = title
	}
	d.mu.Unlock()
	d.emit(d.snapshot(bookID))
}

func (d *downloader) setMessage(bookID, msg string) {
	d.mu.Lock()
	if st, ok := d.states[bookID]; ok {
		st.Message = msg
	}
	d.mu.Unlock()
	d.emit(d.snapshot(bookID))
}

// GetDownloadState reports progress for one book.
func (a *App) GetDownloadState(bookID string) DownloadState {
	return a.downloader.State(bookID)
}

// CancelDownload stops the running download.
func (a *App) CancelDownload() {
	a.downloader.mu.Lock()
	cancel := a.downloader.cancel
	a.downloader.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func sleepCtx(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

func retryable(err error) bool {
	if errors.Is(err, weread.ErrNotEntitled) {
		return false
	}
	msg := err.Error()
	return !strings.Contains(msg, "无权") && !strings.Contains(msg, "不存在")
}

func shorten(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

var _ = library.ErrNotFound
