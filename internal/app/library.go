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
	return a.enrichBookKinds(books)
}

// SyncShelf pulls the cloud shelf and reconciles the cache.
func (a *App) SyncShelf() ([]weread.Book, error) {
	ctx, cancel := context.WithTimeout(a.ctxOrBackground(), 45*time.Second)
	defer cancel()
	return a.syncShelf(ctx)
}

func (a *App) syncShelf(ctx context.Context) ([]weread.Book, error) {
	books, groups, err := a.eink.ShelfSnapshot(ctx)
	if err != nil {
		// Credentials minted by the browser login carry a cookie session but no
		// API token, so the E-Ink shelf answers -2012. The browser shelf serves
		// the same account and keeps the UI usable without a re-login.
		if a.session.Credentials().HasWebSession() {
			if webBooks, werr := a.web.Shelf(ctx); werr == nil {
				a.logf("E-Ink 书架不可用，已改用网页会话：%v", err)
				books, groups, err = webBooks, nil, nil
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
			return a.enrichBookKinds(cached), nil
		}
		return nil, err
	}
	if err := a.store.ReplaceShelf(books); err != nil {
		return nil, err
	}
	// Folder membership is optional: a missing archive list must not discard
	// the books that were just synced, so the error is logged and ignored.
	if err := a.store.ReplaceGroups(groups); err != nil {
		a.logf("同步书架分组失败：%v", err)
	}
	_ = a.store.SetMeta("shelfSyncedAt", fmt.Sprint(time.Now().Unix()))
	return a.enrichBookKinds(books), nil
}

// Groups returns the cached shelf folders immediately (never blocks on the
// network).
func (a *App) Groups() []weread.ShelfGroup {
	groups, err := a.store.Groups()
	if err != nil {
		a.logf("读取分组失败：%v", err)
		return []weread.ShelfGroup{}
	}
	if groups == nil {
		return []weread.ShelfGroup{}
	}
	return groups
}

// CreateGroup creates a folder that lives only in this app. Writing folders
// back to the account is intentionally not offered: /shelf/archive is a write
// endpoint and repeated calls from a desktop client look like automation.
func (a *App) CreateGroup(name string) (*weread.ShelfGroup, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("分组名称不能为空")
	}
	if len([]rune(name)) > 24 {
		return nil, errors.New("分组名称不能超过 24 个字")
	}
	g := weread.ShelfGroup{
		ID:     fmt.Sprintf("local-%d", time.Now().UnixNano()),
		Name:   name,
		Source: "local",
		Sort:   1000, // local folders sort after the account's own folders
	}
	if err := a.store.UpsertGroup(g); err != nil {
		return nil, err
	}
	return &g, nil
}

// RenameGroup renames a folder. A folder mirrored from the account is renamed
// only locally; the account keeps its original name.
func (a *App) RenameGroup(groupID, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("分组名称不能为空")
	}
	if len([]rune(name)) > 24 {
		return errors.New("分组名称不能超过 24 个字")
	}
	return a.store.RenameGroup(groupID, name)
}

// DeleteGroup removes a folder and clears its membership. A folder mirrored
// from the account reappears on the next shelf sync.
func (a *App) DeleteGroup(groupID string) error {
	return a.store.DeleteGroup(groupID)
}

// SetBookGroup moves books into a folder, or out of one when groupID is empty.
func (a *App) SetBookGroup(bookIDs []string, groupID string) error {
	if len(bookIDs) == 0 {
		return errors.New("没有选择书籍")
	}
	name := ""
	if groupID != "" {
		g, err := a.store.Group(groupID)
		if err != nil {
			return err
		}
		name = g.Name
	}
	return a.store.SetBookGroup(bookIDs, groupID, name)
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
	if res == nil {
		return &weread.SearchResult{Query: keyword, Books: []weread.Book{}}, nil
	}
	// Empty results from both channels are nil slices, which JSON-encodes as
	// null. The UI expects an array, so normalise here as well as defensively
	// on the frontend.
	if res.Books == nil {
		res.Books = []weread.Book{}
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
	detail.Book = a.enrichBookKind(detail.Book)
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
	if err == nil {
		_ = a.store.SaveReadStats(mode, baseTime, stats)
		return stats, nil
	}
	// Credentials from the old browser login have no E-Ink token. /readdata/detail
	// is also served by the Web/Mobile channel, so try the cookie session before
	// surfacing the "re-scan" error.
	if a.webSession() {
		if webStats, werr := a.web.Stats(ctx, mode, baseTime); werr == nil {
			_ = a.store.SaveReadStats(mode, baseTime, webStats)
			return webStats, nil
		} else {
			a.logf("网页统计失败：%v", werr)
		}
	}
	return nil, err
}

// MPArticles lists an official account's articles.
func (a *App) MPArticles(accountID string, offset int) ([]weread.MPArticle, error) {
	einkCtx, einkCancel := context.WithTimeout(a.ctxOrBackground(), 20*time.Second)
	articles, einkErr := a.eink.MPArticles(einkCtx, accountID, offset)
	einkCancel()

	// The E-Ink MP list is not available to every credential/session and may
	// return an empty page while the web cookie session still has the articles.
	// Try the web channel's /web/mp/articles endpoint before showing “暂无文章”.
	if len(articles) == 0 && a.webSession() {
		webCtx, webCancel := context.WithTimeout(a.ctxOrBackground(), 20*time.Second)
		webArticles, webErr := a.web.MPArticles(webCtx, accountID, offset)
		webCancel()
		if webErr == nil {
			if webArticles == nil {
				webArticles = []weread.MPArticle{}
			}
			return webArticles, nil
		}
		if einkErr == nil {
			einkErr = webErr
		}
	}

	// The renderer treats this as an array; a nil slice marshals as null and
	// crashes it when the account has no articles yet.
	if articles == nil {
		articles = []weread.MPArticle{}
	}
	return articles, einkErr
}

// OpenMPArticle loads an official-account article as blocks.
func (a *App) OpenMPArticle(reviewID string) (*weread.ChapterContent, error) {
	accountID := mpAccountID(reviewID)
	if cached, blocks, err := a.store.MPArticle(reviewID); err == nil {
		if blocks == nil {
			blocks = []weread.Block{}
		}
		if accountID == "" {
			accountID = cached.Account
		}
		// Older caches may contain only lazy-load placeholders. In that case
		// refetch once; otherwise rewrite direct image URLs to the proxy.
		if accountID != "" && !mpBlocksNeedRefresh(blocks) {
			rewriteMPImageSources(blocks, accountID)
			return &weread.ChapterContent{
				BookID:     accountID,
				ChapterUID: 0,
				Title:      cached.Title,
				Format:     "mp",
				Blocks:     blocks,
				FetchedAt:  cached.CreateTime,
			}, nil
		}
	}

	ctx, cancel := context.WithTimeout(a.ctxOrBackground(), 45*time.Second)
	defer cancel()
	html, title, err := a.web.MPArticle(ctx, reviewID)
	if err != nil {
		return nil, err
	}

	var resolve content.ImageResolver
	if accountID != "" {
		resolve = func(src string) (string, bool) {
			remote, ok := remoteImageSrc(src)
			if !ok {
				return "", false
			}
			return imageProxyURL(accountID, 0, remote), true
		}
	}
	blocks := content.Parse(html, resolve)
	if blocks == nil {
		blocks = []weread.Block{}
	}
	out := &weread.ChapterContent{
		BookID: accountID, Title: title, Format: "mp", Blocks: blocks, FetchedAt: time.Now().Unix(),
	}
	_ = a.store.SaveMPArticle(weread.MPArticle{
		ReviewID: reviewID, Account: accountID, Title: title, CreateTime: time.Now().Unix(),
	}, blocks)
	return out, nil
}

// mpAccountID extracts MP_WXS_<account> from an article review id. The review
// id is MP_WXS_<account>_<article>; older cached rows may not carry Account.
func mpAccountID(reviewID string) string {
	const prefix = "MP_WXS_"
	if !strings.HasPrefix(reviewID, prefix) {
		return ""
	}
	rest := strings.TrimPrefix(reviewID, prefix)
	if i := strings.Index(rest, "_"); i > 0 {
		return prefix + rest[:i]
	}
	return reviewID
}

// mpBlocksNeedRefresh detects caches written before lazy image URLs were
// preferred. Missing blocks are left alone: refetching will not recover them.
func mpBlocksNeedRefresh(blocks []weread.Block) bool {
	for _, block := range blocks {
		if block.Kind != "image" {
			continue
		}
		src := strings.TrimSpace(block.Src)
		if src == "" || strings.HasPrefix(src, "data:") {
			return true
		}
	}
	return false
}

// rewriteMPImageSources turns cached direct CDN URLs into proxy routes so the
// article reader can load hotlink-protected mmbiz images through the app.
func rewriteMPImageSources(blocks []weread.Block, accountID string) {
	for i := range blocks {
		if blocks[i].Kind != "image" {
			continue
		}
		src := strings.TrimSpace(blocks[i].Src)
		if src == "" || strings.HasPrefix(src, "data:") ||
			strings.HasPrefix(src, "/media/") || strings.HasPrefix(src, "/media-proxy?") {
			continue
		}
		if remote, ok := remoteImageSrc(src); ok {
			blocks[i].Src = imageProxyURL(accountID, 0, remote)
		}
	}
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
