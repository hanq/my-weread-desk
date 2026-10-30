// Package eink implements the E-Ink client channel: metadata, shelf, search,
// notes, reviews and statistics. It never serves chapter bodies.
package eink

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"my-weread-desk/internal/weread"
	"my-weread-desk/internal/weread/httpx"
	"my-weread-desk/internal/weread/session"
)

// Client talks to https://i.weread.qq.com with vid/accessToken headers.
type Client struct {
	sess *session.Session
}

// New creates an E-Ink client bound to a session.
func New(sess *session.Session) *Client { return &Client{sess: sess} }

func (c *Client) call(ctx context.Context, method, path string, query url.Values, body any, out any) error {
	u := session.EinkBase() + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	return c.sess.Eink().JSON(ctx, httpx.Request{
		URL:     u,
		Method:  method,
		Body:    body,
		Referer: "https://weread.qq.com/",
	}, out)
}

// withAuthRetry performs fn and, on an expired-session error, renews the token
// and retries exactly once.
func (c *Client) withAuthRetry(ctx context.Context, fn func() error) error {
	err := fn()
	if err == nil || (!weread.IsAuthExpired(err) && !weread.IsNoSuchUser(err)) {
		return err
	}
	if rerr := c.sess.RefreshEink(ctx); rerr != nil {
		return fmt.Errorf("%w: %v", weread.ErrAuthRequired, rerr)
	}
	return fn()
}

// ---------------------------------------------------------------- shelf

type shelfArchiveWire struct {
	ArchiveID weread.FlexString   `json:"archiveId"`
	ID        weread.FlexString   `json:"id"`
	Name      string              `json:"name"`
	BookIDs   []weread.FlexString `json:"bookIds"`
}

type shelfWire struct {
	Books []struct {
		BookID         weread.FlexString `json:"bookId"`
		Title          string            `json:"title"`
		Author         string            `json:"author"`
		Cover          string            `json:"cover"`
		Category       string            `json:"category"`
		Intro          string            `json:"intro"`
		ReadUpdateTime weread.FlexInt    `json:"readUpdateTime"`
		UpdateTime     weread.FlexInt    `json:"updateTime"`
		FinishReading  weread.FlexInt    `json:"finishReading"`
		Secret         weread.FlexInt    `json:"secret"`
		Price          weread.FlexFloat  `json:"price"`
		Paid           weread.FlexBool   `json:"paid"`
		IsTop          weread.FlexInt    `json:"isTop"`
	} `json:"books"`
	MP *struct {
		BookID string `json:"bookId"`
		Title  string `json:"title"`
		Author string `json:"author"`
		Cover  string `json:"cover"`
	} `json:"mp"`
	Albums []struct {
		AlbumInfo struct {
			AlbumID    weread.FlexString `json:"albumId"`
			Name       string            `json:"name"`
			AuthorName string            `json:"authorName"`
			Cover      string            `json:"cover"`
			TrackCount weread.FlexInt    `json:"trackCount"`
		} `json:"albumInfo"`
	} `json:"albums"`
	// The service has shipped the folder list under both spellings, so both are
	// decoded and merged rather than failing on whichever one is absent.
	Archive   []shelfArchiveWire `json:"archive"`
	Archives  []shelfArchiveWire `json:"archives"`
	BookCount weread.FlexInt     `json:"bookCount"`
}

// ShelfSnapshot returns the shelf entries plus the folder membership reported
// by /shelf/sync. The folder list is optional: an account without folders, or
// a response that omits it, still yields a usable book list.
func (c *Client) ShelfSnapshot(ctx context.Context) ([]weread.Book, []weread.ShelfGroup, error) {
	var raw shelfWire
	err := c.withAuthRetry(ctx, func() error {
		return c.call(ctx, "GET", "/shelf/sync", nil, nil, &raw)
	})
	if err != nil {
		return nil, nil, err
	}
	if raw.Books == nil && raw.BookCount.Int64() > 0 {
		return nil, nil, fmt.Errorf("%w: 书架响应缺少 books 字段", weread.ErrProtocolChanged)
	}

	groups, membership := normaliseArchives(raw.Archive, raw.Archives)

	out := make([]weread.Book, 0, len(raw.Books)+len(raw.Albums)+1)
	for _, b := range raw.Books {
		id := b.BookID.String()
		if id == "" {
			continue
		}
		book := weread.Book{
			ID:       id,
			Title:    b.Title,
			Author:   b.Author,
			Cover:    b.Cover,
			Category: b.Category,
			Intro:    b.Intro,
			Price:    b.Price.Float64(),
			Paid:     b.Paid.Bool(),
			IsMP:     strings.HasPrefix(id, "MP_WXS_"),
			Finished: b.FinishReading.Int64() == 1,
			Secret:   b.Secret.Int64() == 1,
			LastRead: b.ReadUpdateTime.Int64(),
			UpdateAt: b.UpdateTime.Int64(),
		}
		if g, ok := membership[id]; ok {
			book.GroupID = g.ID
			book.GroupName = g.Name
		}
		out = append(out, book)
	}
	// Albums are audiobooks. They are shelf entries and the account counts them
	// as such, so the shelf must show them; IsAudio tells the UI not to open the
	// text reader for them.
	for _, a := range raw.Albums {
		id := a.AlbumInfo.AlbumID.String()
		if id == "" {
			continue
		}
		book := weread.Book{
			ID:         id,
			Title:      a.AlbumInfo.Name,
			Author:     a.AlbumInfo.AuthorName,
			Cover:      a.AlbumInfo.Cover,
			IsAudio:    true,
			TrackCount: int(a.AlbumInfo.TrackCount.Int64()),
		}
		if g, ok := membership[id]; ok {
			book.GroupID = g.ID
			book.GroupName = g.Name
		}
		out = append(out, book)
	}
	if raw.MP != nil && raw.MP.BookID != "" {
		out = append(out, weread.Book{
			ID:     raw.MP.BookID,
			Title:  weread.FirstString(raw.MP.Title, "公众号"),
			Author: weread.FirstString(raw.MP.Author, "公众号"),
			Cover:  raw.MP.Cover,
			IsMP:   true,
		})
	}
	// Fold albums into the folder membership so a folder containing only
	// audiobooks is not reported as empty.
	for i := range groups {
		for _, b := range out {
			if b.GroupID != groups[i].ID {
				continue
			}
			groups[i].BookIDs = appendUnique(groups[i].BookIDs, b.ID)
		}
	}
	return out, groups, nil
}

// Shelf returns just the entries, preserving the previous call shape.
func (c *Client) Shelf(ctx context.Context) ([]weread.Book, error) {
	books, _, err := c.ShelfSnapshot(ctx)
	return books, err
}

// normaliseArchives merges the two folder spellings and builds a bookId → folder
// lookup. Duplicate folder ids keep the first non-empty name.
func normaliseArchives(lists ...[]shelfArchiveWire) ([]weread.ShelfGroup, map[string]weread.ShelfGroup) {
	var groups []weread.ShelfGroup
	index := map[string]int{}
	for _, list := range lists {
		for _, a := range list {
			id := firstNonEmpty(a.ArchiveID.String(), a.ID.String())
			if id == "" {
				continue
			}
			if at, ok := index[id]; ok {
				if groups[at].Name == "" {
					groups[at].Name = a.Name
				}
				continue
			}
			g := weread.ShelfGroup{ID: id, Name: a.Name, Source: "cloud", Sort: len(groups)}
			for _, bid := range a.BookIDs {
				if v := bid.String(); v != "" {
					g.BookIDs = append(g.BookIDs, v)
				}
			}
			index[id] = len(groups)
			groups = append(groups, g)
		}
	}
	membership := map[string]weread.ShelfGroup{}
	for _, g := range groups {
		for _, bid := range g.BookIDs {
			membership[bid] = g
		}
	}
	return groups, membership
}

func appendUnique(list []string, v string) []string {
	for _, existing := range list {
		if existing == v {
			return list
		}
	}
	return append(list, v)
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// ---------------------------------------------------------------- search

// SearchScopes mirrors the documented `scope` values.
var SearchScopes = map[string]int{
	"all": 0, "ebook": 10, "novel": 16, "audio": 14,
	"author": 6, "fulltext": 12, "booklist": 13, "mp": 2, "article": 4,
}

type searchWire struct {
	SID     string         `json:"sid"`
	HasMore any            `json:"hasMore"`
	Total   weread.FlexInt `json:"totalCount"`
	Results []struct {
		Title string `json:"title"`
		Scope int    `json:"scope"`
		Books []struct {
			SearchIdx    weread.FlexInt   `json:"searchIdx"`
			NewRating    weread.FlexFloat `json:"newRating"`
			ReadingCount weread.FlexInt   `json:"readingCount"`
			BookInfo     struct {
				BookID      weread.FlexString `json:"bookId"`
				Title       string            `json:"title"`
				Author      string            `json:"author"`
				Cover       string            `json:"cover"`
				Category    string            `json:"category"`
				Intro       string            `json:"intro"`
				WordCount   weread.FlexInt    `json:"wordCount"`
				NewRating   weread.FlexFloat  `json:"newRating"`
				PublishTime string            `json:"publishTime"`
				Publisher   string            `json:"publisher"`
				Price       weread.FlexFloat  `json:"price"`
			} `json:"bookInfo"`
		} `json:"books"`
	} `json:"results"`
}

// Search queries the store.
func (c *Client) Search(ctx context.Context, keyword, scope string, offset, count int) (*weread.SearchResult, error) {
	if count <= 0 || count > 50 {
		count = 20
	}
	scopeValue, ok := SearchScopes[scope]
	if !ok {
		scopeValue = 10
	}
	q := url.Values{}
	q.Set("keyword", keyword)
	q.Set("scope", strconv.Itoa(scopeValue))
	q.Set("maxIdx", strconv.Itoa(offset))
	q.Set("count", strconv.Itoa(count))

	var raw searchWire
	err := c.withAuthRetry(ctx, func() error {
		return c.call(ctx, "GET", "/store/search", q, nil, &raw)
	})
	if err != nil {
		return nil, err
	}

	result := &weread.SearchResult{SID: raw.SID, Query: keyword, Total: int(raw.Total.Int64())}
	seen := map[string]bool{}
	for _, group := range raw.Results {
		for _, hit := range group.Books {
			info := hit.BookInfo
			id := info.BookID.String()
			if id == "" || seen[id] {
				continue
			}
			seen[id] = true
			result.Books = append(result.Books, weread.Book{
				ID:        id,
				Title:     info.Title,
				Author:    info.Author,
				Cover:     info.Cover,
				Category:  info.Category,
				Intro:     info.Intro,
				WordCount: int(info.WordCount.Int64()),
				Rating:    firstNonZero(info.NewRating.Float64(), hit.NewRating.Float64()),
				Price:     info.Price.Float64(),
				Publisher: info.Publisher,
				PublishAt: info.PublishTime,
			})
		}
	}
	result.HasMore = truthy(raw.HasMore)
	if result.Total == 0 {
		result.Total = len(result.Books)
	}
	return result, nil
}

func firstNonZero(values ...float64) float64 {
	for _, v := range values {
		if v != 0 {
			return v
		}
	}
	return 0
}

func truthy(v any) bool {
	switch t := v.(type) {
	case bool:
		return t
	case string:
		return t == "1" || strings.EqualFold(t, "true")
	case float64:
		return t != 0
	case json.Number:
		n, _ := t.Int64()
		return n != 0
	}
	return false
}

// ---------------------------------------------------------------- book info

type infoWire struct {
	BookID        weread.FlexString `json:"bookId"`
	Title         string            `json:"title"`
	Author        string            `json:"author"`
	Translator    string            `json:"translator"`
	Cover         string            `json:"cover"`
	Intro         string            `json:"intro"`
	Category      string            `json:"category"`
	Publisher     string            `json:"publisher"`
	PublishTime   string            `json:"publishTime"`
	ISBN          string            `json:"isbn"`
	WordCount     weread.FlexInt    `json:"wordCount"`
	NewRating     weread.FlexFloat  `json:"newRating"`
	RatingCount   weread.FlexInt    `json:"newRatingCount"`
	Price         weread.FlexFloat  `json:"price"`
	Paid          weread.FlexBool   `json:"paid"`
	FinishReading weread.FlexInt    `json:"finishReading"`
	Secret        weread.FlexInt    `json:"secret"`
}

// Info returns book metadata.
func (c *Client) Info(ctx context.Context, bookID string) (*weread.Book, error) {
	q := url.Values{}
	q.Set("bookId", bookID)
	var raw infoWire
	err := c.withAuthRetry(ctx, func() error {
		return c.call(ctx, "GET", "/book/info", q, nil, &raw)
	})
	if err != nil {
		return nil, err
	}
	if raw.Title == "" && raw.BookID.String() == "" {
		return nil, fmt.Errorf("%w: 书籍详情为空", weread.ErrProtocolChanged)
	}
	return &weread.Book{
		ID:        weread.FirstString(raw.BookID.String(), bookID),
		Title:     raw.Title,
		Author:    raw.Author,
		Cover:     raw.Cover,
		Category:  raw.Category,
		Intro:     raw.Intro,
		WordCount: int(raw.WordCount.Int64()),
		Rating:    raw.NewRating.Float64(),
		RatingN:   int(raw.RatingCount.Int64()),
		Price:     raw.Price.Float64(),
		Paid:      raw.Paid.Bool(),
		Publisher: raw.Publisher,
		PublishAt: raw.PublishTime,
		ISBN:      raw.ISBN,
		Finished:  raw.FinishReading.Int64() == 1,
		Secret:    raw.Secret.Int64() == 1,
		IsMP:      weread.IsMP(bookID),
	}, nil
}

// ---------------------------------------------------------------- chapters

type chapterWire struct {
	Data []struct {
		BookID          weread.FlexString `json:"bookId"`
		Synckey         weread.FlexInt    `json:"synckey"`
		ChapterUpdateAt weread.FlexInt    `json:"chapterUpdateTime"`
		Updated         []struct {
			ChapterUID weread.FlexInt `json:"chapterUid"`
			ChapterIdx weread.FlexInt `json:"chapterIdx"`
			Title      string         `json:"title"`
			WordCount  weread.FlexInt `json:"wordCount"`
			Level      weread.FlexInt `json:"level"`
			Price      weread.FlexInt `json:"price"`
			Paid       weread.FlexInt `json:"paid"`
			UpdateTime weread.FlexInt `json:"updateTime"`
		} `json:"updated"`
	} `json:"data"`
	// Some deployments answer with a bare object.
	BookID  weread.FlexString `json:"bookId"`
	Updated []struct {
		ChapterUID weread.FlexInt `json:"chapterUid"`
		ChapterIdx weread.FlexInt `json:"chapterIdx"`
		Title      string         `json:"title"`
		WordCount  weread.FlexInt `json:"wordCount"`
		Level      weread.FlexInt `json:"level"`
		Price      weread.FlexInt `json:"price"`
		Paid       weread.FlexInt `json:"paid"`
	} `json:"updated"`
}

type chapterEntry struct {
	ChapterUID weread.FlexInt
	ChapterIdx weread.FlexInt
	Title      string
	WordCount  weread.FlexInt
	Level      weread.FlexInt
	Price      weread.FlexInt
	Paid       weread.FlexInt
}

// Chapters returns the table of contents.
func (c *Client) Chapters(ctx context.Context, bookID string) ([]weread.Chapter, error) {
	body := map[string]any{
		"bookIds":     []string{bookID},
		"synckeys":    []int{0},
		"updateTimes": []int{0},
		"maxfreeIdx":  []int{0},
	}
	var raw chapterWire
	err := c.withAuthRetry(ctx, func() error {
		return c.call(ctx, "POST", "/book/chapterInfos", nil, body, &raw)
	})
	if err != nil {
		return nil, err
	}

	var entries []chapterEntry
	for _, d := range raw.Data {
		if d.BookID.String() != "" && d.BookID.String() != bookID {
			continue
		}
		for _, u := range d.Updated {
			entries = append(entries, chapterEntry{u.ChapterUID, u.ChapterIdx, u.Title, u.WordCount, u.Level, u.Price, u.Paid})
		}
	}
	if len(entries) == 0 {
		for _, u := range raw.Updated {
			entries = append(entries, chapterEntry{u.ChapterUID, u.ChapterIdx, u.Title, u.WordCount, u.Level, u.Price, u.Paid})
		}
	}

	out := make([]weread.Chapter, 0, len(entries))
	for i, e := range entries {
		idx := int(e.ChapterIdx.Int64())
		if idx == 0 {
			idx = i + 1
		}
		out = append(out, weread.Chapter{
			UID:       e.ChapterUID.Int64(),
			Index:     idx,
			Title:     e.Title,
			Level:     int(e.Level.Int64()),
			WordCount: int(e.WordCount.Int64()),
			Price:     int(e.Price.Int64()),
			Paid:      e.Paid.Int64() != 0,
		})
	}
	return out, nil
}

// ---------------------------------------------------------------- progress

type progressWire struct {
	BookID weread.FlexString `json:"bookId"`
	Book   struct {
		ChapterUID    weread.FlexInt   `json:"chapterUid"`
		ChapterIdx    weread.FlexInt   `json:"chapterIdx"`
		ChapterOffset weread.FlexInt   `json:"chapterOffset"`
		Progress      weread.FlexFloat `json:"progress"`
		UpdateTime    weread.FlexInt   `json:"updateTime"`
		IsStart       weread.FlexInt   `json:"isStartReading"`
		FinishTime    weread.FlexInt   `json:"finishTime"`
	} `json:"book"`
}

// Progress reads the cloud reading position.
func (c *Client) Progress(ctx context.Context, bookID string) (*weread.Progress, error) {
	q := url.Values{}
	q.Set("bookId", bookID)
	var raw progressWire
	err := c.withAuthRetry(ctx, func() error {
		return c.call(ctx, "GET", "/book/getProgress", q, nil, &raw)
	})
	if err != nil {
		return nil, err
	}
	return &weread.Progress{
		BookID:       bookID,
		ChapterUID:   raw.Book.ChapterUID.Int64(),
		ChapterIndex: int(raw.Book.ChapterIdx.Int64()),
		Offset:       int(raw.Book.ChapterOffset.Int64()),
		Percent:      raw.Book.Progress.Float64(),
		UpdateTime:   raw.Book.UpdateTime.Int64(),
		IsStart:      raw.Book.IsStart.Int64() == 1,
		Finished:     raw.Book.FinishTime.Int64() > 0,
	}, nil
}

// ---------------------------------------------------------------- notes

type bookmarkWire struct {
	Updated []struct {
		BookmarkID string            `json:"bookmarkId"`
		BookID     weread.FlexString `json:"bookId"`
		ChapterUID weread.FlexInt    `json:"chapterUid"`
		MarkText   string            `json:"markText"`
		CreateTime weread.FlexInt    `json:"createTime"`
		Type       weread.FlexInt    `json:"type"`
		Range      string            `json:"range"`
		ColorStyle weread.FlexInt    `json:"colorStyle"`
	} `json:"updated"`
	Chapters []struct {
		ChapterUID weread.FlexInt `json:"chapterUid"`
		Title      string         `json:"title"`
	} `json:"chapters"`
}

// Notes returns the underlines of a book.
func (c *Client) Notes(ctx context.Context, bookID string) ([]weread.Note, error) {
	q := url.Values{}
	q.Set("bookId", bookID)
	var raw bookmarkWire
	err := c.withAuthRetry(ctx, func() error {
		return c.call(ctx, "GET", "/book/bookmarklist", q, nil, &raw)
	})
	if err != nil {
		return nil, err
	}
	titles := map[int64]string{}
	for _, ch := range raw.Chapters {
		titles[ch.ChapterUID.Int64()] = ch.Title
	}
	out := make([]weread.Note, 0, len(raw.Updated))
	for _, n := range raw.Updated {
		out = append(out, weread.Note{
			ID:          n.BookmarkID,
			BookID:      n.BookID.String(),
			ChapterUID:  n.ChapterUID.Int64(),
			ChapterName: titles[n.ChapterUID.Int64()],
			Text:        n.MarkText,
			Range:       n.Range,
			Color:       int(n.ColorStyle.Int64()),
			CreateTime:  n.CreateTime.Int64(),
			Type:        int(n.Type.Int64()),
		})
	}
	return out, nil
}

// ---------------------------------------------------------------- stats

// statsWire is the shared /readdata/detail payload. The conversion lives in
// the weread package so the E-Ink and Web paths cannot drift apart.
type statsWire = weread.StatsWire

// Stats returns reading statistics for a period.
func (c *Client) Stats(ctx context.Context, mode string, baseTime int64) (*weread.ReadStats, error) {
	if mode == "" {
		mode = "monthly"
	}
	q := url.Values{}
	q.Set("mode", mode)
	q.Set("baseTime", strconv.FormatInt(baseTime, 10))
	var raw statsWire
	err := c.withAuthRetry(ctx, func() error {
		return c.call(ctx, "GET", "/readdata/detail", q, nil, &raw)
	})
	if err != nil {
		return nil, err
	}
	return raw.ToReadStats(mode), nil
}

// ---------------------------------------------------------------- shelf edits

// AddToShelf adds books.
func (c *Client) AddToShelf(ctx context.Context, bookIDs ...string) error {
	body := map[string]any{"bookIds": bookIDs, "albumIds": []string{}, "follow": 0, "promoteId": 0}
	return c.withAuthRetry(ctx, func() error {
		return c.call(ctx, "POST", "/shelf/add", nil, body, nil)
	})
}

// RemoveFromShelf removes books.
func (c *Client) RemoveFromShelf(ctx context.Context, bookIDs ...string) error {
	body := map[string]any{"bookIds": bookIDs, "albumIds": []string{}, "archiveIds": []string{}}
	return c.withAuthRetry(ctx, func() error {
		return c.call(ctx, "POST", "/shelf/delete", nil, body, nil)
	})
}

// SetSecret toggles the private flag.
func (c *Client) SetSecret(ctx context.Context, bookID string, secret bool) error {
	private := 0
	if secret {
		private = 1
	}
	body := map[string]any{"bookIds": []string{bookID}, "albumIds": []string{}, "private": private}
	return c.withAuthRetry(ctx, func() error {
		return c.call(ctx, "POST", "/book/secret", nil, body, nil)
	})
}

// MarkFinished marks a book as read (or not).
func (c *Client) MarkFinished(ctx context.Context, bookID string, finished bool) error {
	isCancel := 1
	if finished {
		isCancel = 0
	}
	body := map[string]any{"auto": 0, "bookId": bookID, "finishInfo": 0, "isCancel": isCancel, "status": 4}
	return c.withAuthRetry(ctx, func() error {
		return c.call(ctx, "POST", "/book/markstatus", nil, body, nil)
	})
}

// ---------------------------------------------------------------- mp

type mpWire struct {
	Reviews []struct {
		CreateTime weread.FlexInt `json:"createTime"`
		SubCount   weread.FlexInt `json:"subCount"`
		SubReviews []struct {
			Review struct {
				ReviewID   string         `json:"reviewId"`
				CreateTime weread.FlexInt `json:"createTime"`
				MPInfo     struct {
					Title      string `json:"title"`
					OriginalID string `json:"originalId"`
					PicURL     string `json:"pic_url"`
				} `json:"mpInfo"`
			} `json:"review"`
		} `json:"subReviews"`
	} `json:"reviews"`
	Synckey  weread.FlexInt `json:"synckey"`
	ClearAll weread.FlexInt `json:"clearAll"`
}

// MPArticles lists the articles of an official account.
func (c *Client) MPArticles(ctx context.Context, accountID string, offset int) ([]weread.MPArticle, error) {
	q := url.Values{}
	q.Set("bookId", accountID)
	q.Set("offset", strconv.Itoa(offset))
	q.Set("count", "20")
	var raw mpWire
	err := c.withAuthRetry(ctx, func() error {
		return c.call(ctx, "GET", "/mp/chapters", q, nil, &raw)
	})
	if err != nil {
		return nil, err
	}
	// Non-nil so an account without articles marshals as [] rather than null;
	// the renderer treats the value as an array.
	out := make([]weread.MPArticle, 0, 16)
	for _, group := range raw.Reviews {
		for _, sub := range group.SubReviews {
			r := sub.Review
			if r.ReviewID == "" {
				continue
			}
			out = append(out, weread.MPArticle{
				ReviewID:   r.ReviewID,
				Title:      r.MPInfo.Title,
				Cover:      r.MPInfo.PicURL,
				CreateTime: r.CreateTime.Int64(),
				Account:    accountID,
			})
		}
	}
	return out, nil
}
