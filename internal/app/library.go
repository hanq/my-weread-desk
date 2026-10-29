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
)

// Books returns the cached shelf immediately (never blocks on the network).
func (a *App) Books() []weread.Book {
	books, err := a.store.Books()
	if err != nil {
		a.logf("读取本地书架失败：%v", err)
		return []weread.Book{}
	}
	if books == nil {
		return []weread.Book{}
	}
	return books
}

// SyncShelf pulls the cloud shelf and reconciles the cache.
func (a *App) SyncShelf() ([]weread.Book, error) {
	ctx, cancel := context.WithTimeout(a.ctxOrBackground(), 45*time.Second)
	defer cancel()
	return a.syncShelf(ctx)
}

func (a *App) syncShelf(ctx context.Context) ([]weread.Book, error) {
	books, err := a.eink.Shelf(ctx)
	if err != nil {
		// Credentials minted by the browser login carry a cookie session but no
		// API token, so the E-Ink shelf answers -2012. The browser shelf serves
		// the same account and keeps the UI usable without a re-login.
		if a.session.Credentials().HasWebSession() {
			if webBooks, werr := a.web.Shelf(ctx); werr == nil {
				a.logf("E-Ink 书架不可用，已改用网页会话：%v", err)
				books = webBooks
				err = nil
			} else {
				a.logf("网页书架同步失败：%v", werr)
			}
		}
	}
	if err != nil {
		// Fall back to the cache so the UI stays usable offline.
		cached, cerr := a.store.Books()
		if cerr == nil && len(cached) > 0 {
			a.logf("同步书架失败，使用本地缓存：%v", err)
			return cached, nil
		}
		return nil, err
	}
	if err := a.store.ReplaceShelf(books); err != nil {
		return nil, err
	}
	_ = a.store.SetMeta("shelfSyncedAt", fmt.Sprint(time.Now().Unix()))
	return books, nil
}

// Search queries the bookstore.
func (a *App) Search(keyword, scope string, page int) (*weread.SearchResult, error) {
	keyword = strings.TrimSpace(keyword)
	if keyword == "" {
		return &weread.SearchResult{}, nil
	}
	if page < 0 {
		page = 0
	}
	ctx, cancel := context.WithTimeout(a.ctxOrBackground(), 30*time.Second)
	defer cancel()
	res, err := a.eink.Search(ctx, keyword, scope, page*20, 20)
	if err != nil {
		// Credentials minted by the browser login carry a cookie session but no
		// usable API token. The browser store search serves the same catalogue,
		// so searching keeps working until the user re-scans.
		if a.session.Credentials().HasWebSession() {
			if webRes, werr := a.web.Search(ctx, keyword, page*20, 20); werr == nil {
				a.logf("E-Ink 搜索不可用，已改用网页会话：%v", err)
				res, err = webRes, nil
			} else {
				a.logf("网页搜索失败：%v", werr)
			}
		}
	}
	if err != nil {
		return nil, err
	}
	for _, b := range res.Books {
		_ = a.store.UpsertBook(b)
	}
	return res, nil
}

// webSession reports whether the cookie channel can serve as a fallback.
func (a *App) webSession() bool {
	c := a.session.Credentials()
	return c != nil && c.HasWebSession()
}

// BookDetail bundles everything the detail screen needs.
type BookDetail struct {
	Book         weread.Book      `json:"book"`
	Chapters     []weread.Chapter `json:"chapters"`
	Progress     *weread.Progress `json:"progress,omitempty"`
	CachedCount  int              `json:"cachedCount"`
	ChapterCount int              `json:"chapterCount"`
	InShelf      bool             `json:"inShelf"`
	Notes        []weread.Note    `json:"notes"`
	Offline      bool             `json:"offline"`
}

// BookDetail loads metadata, chapters and progress for one book.
func (a *App) BookDetail(bookID string) (*BookDetail, error) {
	if bookID == "" {
		return nil, errors.New("缺少书籍 ID")
	}
	ctx, cancel := context.WithTimeout(a.ctxOrBackground(), 40*time.Second)
	defer cancel()

	detail := &BookDetail{Book: weread.Book{ID: bookID}, Chapters: []weread.Chapter{}, Notes: []weread.Note{}}

	if cached, err := a.store.Book(bookID); err == nil {
		detail.Book = *cached
		detail.InShelf = true
	}
	if chapters, err := a.store.Chapters(bookID); err == nil && len(chapters) > 0 {
		detail.Chapters = chapters
	}

	if info, err := a.eink.Info(ctx, bookID); err == nil {
		detail.Book = *info
		_ = a.store.UpsertBook(*info)
	} else if a.webSession() {
		if info, werr := a.web.BookInfo(ctx, bookID); werr == nil {
			detail.Book = *info
			_ = a.store.UpsertBook(*info)
		} else {
			detail.Offline = len(detail.Chapters) > 0
			a.logf("读取书籍详情失败：%v（网页通道：%v）", err, werr)
		}
	} else {
		detail.Offline = len(detail.Chapters) > 0
		a.logf("读取书籍详情失败：%v", err)
	}

	if chapters, err := a.eink.Chapters(ctx, bookID); err == nil && len(chapters) > 0 {
		detail.Chapters = chapters
		_ = a.store.ReplaceChapters(bookID, chapters)
	} else if a.webSession() {
		if chapters, _, werr := a.web.Catalog(ctx, bookID); werr == nil && len(chapters) > 0 {
			detail.Chapters = chapters
			_ = a.store.ReplaceChapters(bookID, chapters)
		}
	}

	if p, err := a.eink.Progress(ctx, bookID); err == nil {
		detail.Progress = p
		_ = a.store.SaveProgress(*p, false)
	} else if a.webSession() {
		if p, werr := a.web.Progress(ctx, bookID); werr == nil {
			detail.Progress = p
			_ = a.store.SaveProgress(*p, false)
		} else if p, serr := a.store.Progress(bookID); serr == nil {
			detail.Progress = p
		}
	} else if p, err := a.store.Progress(bookID); err == nil {
		detail.Progress = p
	}

	if cached, err := a.store.CachedChapterUIDs(bookID); err == nil {
		detail.CachedCount = len(cached)
	}
	detail.ChapterCount = len(detail.Chapters)
	if notes, err := a.eink.Notes(ctx, bookID); err == nil {
		detail.Notes = notes
	} else if a.webSession() {
		if notes, werr := a.web.Notes(ctx, bookID); werr == nil {
			detail.Notes = notes
		}
	}
	return detail, nil
}

// Chapters returns the cached table of contents.
func (a *App) Chapters(bookID string) ([]weread.Chapter, error) {
	chapters, err := a.store.Chapters(bookID)
	if err != nil {
		return nil, err
	}
	if len(chapters) == 0 {
		ctx, cancel := context.WithTimeout(a.ctxOrBackground(), 30*time.Second)
		defer cancel()
		fresh, ferr := a.eink.Chapters(ctx, bookID)
		if ferr != nil {
			if a.webSession() {
				if webChapters, _, werr := a.web.Catalog(ctx, bookID); werr == nil && len(webChapters) > 0 {
					_ = a.store.ReplaceChapters(bookID, webChapters)
					return webChapters, nil
				}
			}
			return chapters, nil
		}
		_ = a.store.ReplaceChapters(bookID, fresh)
		return fresh, nil
	}
	return chapters, nil
}

// ShelfMutation groups the small shelf edits.
type ShelfMutation struct {
	Action string `json:"action"` // add | remove | finish | unfinish | secret | public
	BookID string `json:"bookId"`
}

// MutateShelf performs a shelf edit and refreshes the cache entry.
func (a *App) MutateShelf(req ShelfMutation) error {
	ctx, cancel := context.WithTimeout(a.ctxOrBackground(), 25*time.Second)
	defer cancel()

	switch req.Action {
	case "add":
		if err := a.eink.AddToShelf(ctx, req.BookID); err != nil {
			return err
		}
	case "remove":
		if err := a.eink.RemoveFromShelf(ctx, req.BookID); err != nil {
			return err
		}
	case "finish":
		return a.eink.MarkFinished(ctx, req.BookID, true)
	case "unfinish":
		return a.eink.MarkFinished(ctx, req.BookID, false)
	case "secret":
		return a.eink.SetSecret(ctx, req.BookID, true)
	case "public":
		return a.eink.SetSecret(ctx, req.BookID, false)
	default:
		return fmt.Errorf("未知的书架操作：%s", req.Action)
	}

	_, err := a.syncShelf(ctx)
	return err
}

// Notes returns the underlines of a book.
func (a *App) Notes(bookID string) ([]weread.Note, error) {
	ctx, cancel := context.WithTimeout(a.ctxOrBackground(), 30*time.Second)
	defer cancel()
	notes, err := a.eink.Notes(ctx, bookID)
	if err == nil {
		return notes, nil
	}
	if a.webSession() {
		if webNotes, werr := a.web.Notes(ctx, bookID); werr == nil {
			return webNotes, nil
		}
	}
	return nil, err
}

// Stats returns reading statistics.
func (a *App) Stats(mode string, baseTime int64) (*weread.ReadStats, error) {
	ctx, cancel := context.WithTimeout(a.ctxOrBackground(), 30*time.Second)
	defer cancel()
	stats, err := a.eink.Stats(ctx, mode, baseTime)
	if err != nil {
		return nil, err
	}
	_ = a.store.SaveReadStats(mode, baseTime, stats)
	return stats, nil
}

// MPArticles lists an official account's articles.
func (a *App) MPArticles(accountID string, offset int) ([]weread.MPArticle, error) {
	ctx, cancel := context.WithTimeout(a.ctxOrBackground(), 30*time.Second)
	defer cancel()
	return a.eink.MPArticles(ctx, accountID, offset)
}

// OpenMPArticle loads an official-account article as blocks.
func (a *App) OpenMPArticle(reviewID string) (*weread.ChapterContent, error) {
	if cached, blocks, err := a.store.MPArticle(reviewID); err == nil {
		return &weread.ChapterContent{
			BookID:     cached.Account,
			ChapterUID: 0,
			Title:      cached.Title,
			Format:     "mp",
			Blocks:     blocks,
			FetchedAt:  cached.CreateTime,
		}, nil
	}

	ctx, cancel := context.WithTimeout(a.ctxOrBackground(), 45*time.Second)
	defer cancel()
	html, title, err := a.web.MPArticle(ctx, reviewID)
	if err != nil {
		return nil, err
	}
	blocks := content.Parse(html, nil)
	out := &weread.ChapterContent{
		Title: title, Format: "mp", Blocks: blocks, FetchedAt: time.Now().Unix(),
	}
	_ = a.store.SaveMPArticle(weread.MPArticle{
		ReviewID: reviewID, Title: title, CreateTime: time.Now().Unix(),
	}, blocks)
	return out, nil
}

// CacheCoverage reports how much of a book is available offline.
type CacheCoverage struct {
	Cached int `json:"cached"`
	Total  int `json:"total"`
}

// CachedChapterCount reports cache coverage for the shelf badges.
func (a *App) CachedChapterCount(bookID string) CacheCoverage {
	cached, err := a.store.CachedChapterUIDs(bookID)
	if err != nil {
		return CacheCoverage{}
	}
	chapters, err := a.store.Chapters(bookID)
	if err != nil {
		return CacheCoverage{Cached: len(cached), Total: len(cached)}
	}
	return CacheCoverage{Cached: len(cached), Total: len(chapters)}
}

// ClearChapterCache drops decoded chapters for one book.
func (a *App) ClearChapterCache(bookID string) error {
	return a.store.DropChapterContent(bookID)
}

var _ = library.ErrNotFound
