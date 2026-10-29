package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"my-weread-desk/internal/content"
	"my-weread-desk/internal/library"
	"my-weread-desk/internal/weread"
	"my-weread-desk/internal/weread/web"
)

// ReaderSession is the payload returned when a chapter is opened.
type ReaderSession struct {
	Book      weread.Book           `json:"book"`
	Chapter   weread.Chapter        `json:"chapter"`
	Chapters  []weread.Chapter      `json:"chapters"`
	Content   weread.ChapterContent `json:"content"`
	Progress  *weread.Progress      `json:"progress,omitempty"`
	FromCache bool                  `json:"fromCache"`
}

// OpenChapter resolves a chapter, using the local cache when possible.
func (a *App) OpenChapter(bookID string, chapterUID int64) (*ReaderSession, error) {
	if bookID == "" {
		return nil, errors.New("缺少书籍 ID")
	}
	ctx, cancel := context.WithTimeout(a.ctxOrBackground(), 60*time.Second)
	defer cancel()

	chapters, err := a.store.Chapters(bookID)
	if err != nil {
		return nil, err
	}
	if len(chapters) == 0 {
		if chapters, err = a.eink.Chapters(ctx, bookID); err != nil {
			return nil, err
		}
		_ = a.store.ReplaceChapters(bookID, chapters)
	}
	if len(chapters) == 0 {
		return nil, fmt.Errorf("%w: 该书没有可读章节", weread.ErrProtocolChanged)
	}

	chapter, ok := pickChapter(chapters, chapterUID)
	if !ok {
		chapter = chapters[0]
	}

	book, _ := a.store.Book(bookID)
	if book == nil {
		book = &weread.Book{ID: bookID, Title: chapter.Title}
	}
	sess := &ReaderSession{Book: *book, Chapter: chapter, Chapters: chapters}

	if cached, err := a.store.ChapterContent(bookID, chapter.UID); err == nil {
		sess.Content = *cached
		sess.FromCache = true
		if p, perr := a.store.Progress(bookID); perr == nil {
			sess.Progress = p
		}
		// Refresh the reader state in the background so progress reporting
		// works even when the chapter came from disk.
		go a.ensureReaderState(bookID, chapter.UID)
		return sess, nil
	}

	ch, err := a.fetchChapter(ctx, bookID, chapter, chapters)
	if err != nil {
		return nil, err
	}
	sess.Content = *ch
	if p, perr := a.store.Progress(bookID); perr == nil {
		sess.Progress = p
	}
	return sess, nil
}

func pickChapter(chapters []weread.Chapter, uid int64) (weread.Chapter, bool) {
	if uid != 0 {
		for _, c := range chapters {
			if c.UID == uid {
				return c, true
			}
		}
	}
	if len(chapters) > 0 {
		return chapters[0], true
	}
	return weread.Chapter{}, false
}

// fetchChapter downloads, decodes and caches one chapter.
func (a *App) fetchChapter(ctx context.Context, bookID string, chapter weread.Chapter, all []weread.Chapter) (*weread.ChapterContent, error) {
	state, err := a.ensureReaderStateSync(ctx, bookID, chapter.UID)
	if err != nil {
		return nil, err
	}

	// The E-Ink catalogue carries no resource package URL, so a web catalogue
	// pass is needed once per book to get chapter tar archives and the format.
	if chapter.Tar == "" || chapter.Format == "" {
		if webChapters, format, werr := a.web.Catalog(ctx, bookID); werr == nil && len(webChapters) > 0 {
			for i := range webChapters {
				if webChapters[i].Format == "" {
					webChapters[i].Format = format
				}
			}
			_ = a.store.ReplaceChapters(bookID, webChapters)
			all = webChapters
			if c, ok := pickChapter(webChapters, chapter.UID); ok {
				chapter = c
			}
			_ = a.store.SetBookFormat(bookID, format)
		}
	}

	result, err := a.web.Chapter(ctx, bookID, chapter, state)
	if err != nil {
		return nil, err
	}

	media := map[string]string{}
	if result.Tar != "" {
		media = a.cacheChapterImages(ctx, bookID, result.Tar)
	}

	resolve := func(src string) (string, bool) {
		name := webMediaName(src)
		if name == "" {
			return "", false
		}
		if _, ok := media[name]; ok {
			return "/media/" + bookID + "/" + name, true
		}
		if a.store.HasMedia(bookID, name) {
			return "/media/" + bookID + "/" + name, true
		}
		return "", false
	}

	blocks := content.Parse(result.HTML, resolve)
	if len(blocks) == 0 {
		return nil, fmt.Errorf("%w: 章节正文为空", weread.ErrProtocolChanged)
	}

	out := &weread.ChapterContent{
		BookID:     bookID,
		ChapterUID: chapter.UID,
		Title:      weread.FirstString(result.Title, chapter.Title),
		Format:     result.Format,
		Blocks:     blocks,
		CSS:        result.CSS,
		FetchedAt:  time.Now().Unix(),
	}
	if err := a.store.SaveChapterContent(out); err != nil {
		a.logf("写入章节缓存失败：%v", err)
	}
	_ = a.store.SetBookFormat(bookID, result.Format)
	a.rememberReaderState(bookID, state, chapter.UID)
	return out, nil
}

func (a *App) cacheChapterImages(ctx context.Context, bookID, tarURL string) map[string]string {
	out := map[string]string{}
	data, err := a.downloadTar(ctx, tarURL)
	if err != nil {
		a.logf("下载章节图片失败：%v", err)
		return out
	}
	files, err := parseTar(data)
	if err != nil {
		return out
	}
	for name, bytes := range files {
		ext := imageExt(bytes)
		stored := name
		if !strings.HasSuffix(strings.ToLower(stored), ext) {
			stored += ext
		}
		if err := a.store.SaveMedia(bookID, stored, imageMime(ext), bytes); err != nil {
			continue
		}
		out[name] = stored
		out[stored] = stored
	}
	return out
}

// SavePosition records the reading position locally and marks it dirty.
type Position struct {
	BookID         string  `json:"bookId"`
	ChapterUID     int64   `json:"chapterUid"`
	ChapterIndex   int     `json:"chapterIndex"`
	Offset         int     `json:"offset"`
	ChapterPercent float64 `json:"chapterPercent"`
	Summary        string  `json:"summary"`
}

// SavePosition persists a local reading position (no network).
func (a *App) SavePosition(p Position) error {
	if p.BookID == "" {
		return errors.New("缺少书籍 ID")
	}
	percent := a.bookPercent(p.BookID, p.ChapterIndex, p.ChapterPercent)
	err := a.store.SaveProgress(weread.Progress{
		BookID:       p.BookID,
		ChapterUID:   p.ChapterUID,
		ChapterIndex: p.ChapterIndex,
		Offset:       p.Offset,
		Percent:      percent,
		UpdateTime:   time.Now().Unix(),
	}, true)
	if err != nil {
		return err
	}
	a.reporter.touch(p)
	return nil
}

func (a *App) bookPercent(bookID string, chapterIndex int, chapterPercent float64) float64 {
	chapters, err := a.store.Chapters(bookID)
	if err != nil || len(chapters) == 0 {
		return chapterPercent
	}
	idx := chapterIndex
	if idx <= 0 {
		idx = 1
	}
	total := float64(len(chapters))
	p := (float64(idx-1) + clamp01(chapterPercent/100)) / total * 100
	return clamp(p, 0, 100)
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func clamp01(v float64) float64 { return clamp(v, 0, 1) }

// StoredProgress returns the local reading position.
func (a *App) StoredProgress(bookID string) (*weread.Progress, error) {
	p, err := a.store.Progress(bookID)
	if errors.Is(err, library.ErrNotFound) {
		return nil, nil
	}
	return p, err
}

// SyncProgress fetches the cloud position and stores it when newer.
func (a *App) SyncProgress(bookID string) (*weread.Progress, error) {
	ctx, cancel := context.WithTimeout(a.ctxOrBackground(), 25*time.Second)
	defer cancel()
	cloud, err := a.eink.Progress(ctx, bookID)
	if err != nil {
		return nil, err
	}
	local, lerr := a.store.Progress(bookID)
	if lerr == nil && local.UpdateTime > cloud.UpdateTime && local.Offset > 0 {
		// Local is ahead: keep it and let the reporter push it upstream.
		return local, nil
	}
	_ = a.store.SaveProgress(*cloud, false)
	return cloud, nil
}

// ReportNow pushes the current position immediately.
func (a *App) ReportNow(bookID string) error {
	return a.reporter.reportNow(bookID)
}

// ---------------------------------------------------------------- reader state

func (a *App) rememberReaderState(bookID string, st *weread.ReaderState, chapterUID int64) {
	if st == nil {
		return
	}
	cp := *st
	cp.Chapter = chapterUID
	a.mu.Lock()
	a.readerStates[bookID] = &cp
	a.mu.Unlock()
}

func (a *App) cachedReaderState(bookID string) *weread.ReaderState {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.readerStates[bookID]
}

// ensureReaderStateSync returns a reader state, fetching it when missing.
func (a *App) ensureReaderStateSync(ctx context.Context, bookID string, chapterUID int64) (*weread.ReaderState, error) {
	if st := a.cachedReaderState(bookID); st != nil && st.PSVTS != "" {
		return st, nil
	}
	st, err := a.web.ReaderState(ctx, bookID, chapterUID)
	if err != nil {
		return nil, err
	}
	a.rememberReaderState(bookID, st, chapterUID)
	return st, nil
}

func (a *App) ensureReaderState(bookID string, chapterUID int64) {
	ctx, cancel := context.WithTimeout(a.ctxOrBackground(), 30*time.Second)
	defer cancel()
	if _, err := a.ensureReaderStateSync(ctx, bookID, chapterUID); err != nil {
		a.logf("获取阅读会话失败：%v", err)
	}
}

// ---------------------------------------------------------------- helpers

func (a *App) downloadTar(ctx context.Context, tarURL string) ([]byte, error) {
	return a.web.DownloadResource(ctx, tarURL)
}

func parseTar(data []byte) (map[string][]byte, error) { return web.ParseTar(data) }
func imageExt(data []byte) string                     { return web.ImageExt(data) }
func imageMime(ext string) string                     { return web.ImageMime(ext) }

// webMediaName maps an image reference to its archive base name.
var webMediaName = web.MediaName
