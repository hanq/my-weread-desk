package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"my-weread-desk/internal/weread"
	"my-weread-desk/internal/weread/httpx"
)

// The browser channel exposes the same book metadata, reading position and
// underlines as the E-Ink API. These helpers exist so an account that only has
// a cookie session (credentials minted by the old browser login) can still
// browse, open and read books.

// BookInfo returns book metadata through the browser channel.
func (c *Client) BookInfo(ctx context.Context, bookID string) (*weread.Book, error) {
	q := url.Values{}
	q.Set("bookId", bookID)

	var raw struct {
		BookID      weread.FlexString `json:"bookId"`
		Title       string            `json:"title"`
		Author      string            `json:"author"`
		Cover       string            `json:"cover"`
		Intro       string            `json:"intro"`
		Category    string            `json:"category"`
		Publisher   string            `json:"publisher"`
		PublishTime string            `json:"publishTime"`
		ISBN        string            `json:"isbn"`
		WordCount   weread.FlexInt    `json:"wordCount"`
		NewRating   weread.FlexFloat  `json:"newRating"`
		RatingCount weread.FlexInt    `json:"newRatingCount"`
		Price       weread.FlexFloat  `json:"price"`
		Finished    weread.FlexInt    `json:"finished"`
	}
	err := c.withRetry(ctx, func() error {
		resp, err := c.get(ctx, "/web/book/info", q)
		if err != nil {
			return err
		}
		if err := httpx.CheckEnvelope("web/book/info", resp.Status, resp.Body); err != nil {
			return err
		}
		return json.Unmarshal(resp.Body, &raw)
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
		Rating:    normaliseRating(raw.NewRating.Float64()),
		RatingN:   int(raw.RatingCount.Int64()),
		Price:     raw.Price.Float64(),
		Publisher: raw.Publisher,
		PublishAt: raw.PublishTime,
		ISBN:      raw.ISBN,
		Finished:  raw.Finished.Int64() == 1,
		IsMP:      weread.IsMP(bookID),
	}, nil
}

// Progress returns the cloud reading position through the browser channel.
func (c *Client) Progress(ctx context.Context, bookID string) (*weread.Progress, error) {
	q := url.Values{}
	q.Set("bookId", bookID)

	var raw struct {
		Book struct {
			ChapterUID    weread.FlexInt   `json:"chapterUid"`
			ChapterIdx    weread.FlexInt   `json:"chapterIdx"`
			ChapterOffset weread.FlexInt   `json:"chapterOffset"`
			Progress      weread.FlexFloat `json:"progress"`
			UpdateTime    weread.FlexInt   `json:"updateTime"`
			IsStart       weread.FlexInt   `json:"isStartReading"`
			FinishTime    weread.FlexInt   `json:"finishTime"`
		} `json:"book"`
	}
	err := c.withRetry(ctx, func() error {
		resp, err := c.get(ctx, "/web/book/getProgress", q)
		if err != nil {
			return err
		}
		if err := httpx.CheckEnvelope("web/book/getProgress", resp.Status, resp.Body); err != nil {
			return err
		}
		return json.Unmarshal(resp.Body, &raw)
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

// Notes returns the underlines of a book through the browser channel.
func (c *Client) Notes(ctx context.Context, bookID string) ([]weread.Note, error) {
	q := url.Values{}
	q.Set("bookId", bookID)

	var raw struct {
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
	err := c.withRetry(ctx, func() error {
		resp, err := c.get(ctx, "/web/book/bookmarklist", q)
		if err != nil {
			return err
		}
		if err := httpx.CheckEnvelope("web/book/bookmarklist", resp.Status, resp.Body); err != nil {
			return err
		}
		return json.Unmarshal(resp.Body, &raw)
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
