// Package web implements the cookie-authenticated web reader channel. It is
// the only channel that can serve chapter bodies, styles, images and
// official-account articles.
package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"my-weread-desk/internal/weread"
	"my-weread-desk/internal/weread/httpx"
	"my-weread-desk/internal/weread/protocol"
	"my-weread-desk/internal/weread/session"
)

// Client talks to weread.qq.com with the account cookies.
type Client struct {
	sess *session.Session
	// AlwaysFreshPSVTS forces a per-chapter reader page fetch. Some accounts
	// return a stale psvts on the book page, which makes shards fail.
	AlwaysFreshPSVTS bool
}

// New creates a web client bound to a session.
func New(sess *session.Session) *Client {
	sess.Web().UserAgent = protocol.DefaultUserAgent
	sess.Web().SetHeader("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	return &Client{sess: sess}
}

func (c *Client) get(ctx context.Context, path string, query url.Values) (*httpx.Response, error) {
	u := session.WebBase() + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	resp, err := c.sess.Web().Do(ctx, httpx.Request{
		URL:     u,
		Method:  "GET",
		Referer: session.WebBase() + "/",
	})
	if err != nil {
		return nil, err
	}
	c.persistCookies()
	return resp, nil
}

func (c *Client) post(ctx context.Context, path string, body any, referer string) (*httpx.Response, error) {
	if referer == "" {
		referer = session.WebBase() + "/"
	}
	resp, err := c.sess.Web().Do(ctx, httpx.Request{
		URL:     session.WebBase() + path,
		Method:  "POST",
		Body:    body,
		Referer: referer,
		Origin:  session.WebBase(),
		NoRetry: true,
	})
	if err != nil {
		return nil, err
	}
	c.persistCookies()
	return resp, nil
}

func (c *Client) persistCookies() {
	c.sess.MergeCookies(c.sess.Web().Cookies(session.WebBase()))
}

// renew refreshes cookies once and retries.
func (c *Client) renew(ctx context.Context) error {
	return c.sess.RefreshWeb(ctx)
}

// withRetry runs fn, renewing the cookie session once on -2012.
func (c *Client) withRetry(ctx context.Context, fn func() error) error {
	err := fn()
	if err == nil || !weread.IsAuthExpired(err) {
		return err
	}
	if rerr := c.renew(ctx); rerr != nil {
		return fmt.Errorf("%w: %v", weread.ErrAuthRequired, rerr)
	}
	return fn()
}

// ReaderState loads `window.__INITIAL_STATE__` for a book (or a chapter).
func (c *Client) ReaderState(ctx context.Context, bookID string, chapterUID int64) (*weread.ReaderState, error) {
	path := "/web/reader/" + protocol.E(bookID)
	if chapterUID > 0 {
		path += "k" + protocol.E(strconv.FormatInt(chapterUID, 10))
	}

	var state *weread.ReaderState
	err := c.withRetry(ctx, func() error {
		resp, err := c.get(ctx, path, nil)
		if err != nil {
			return err
		}
		if err := httpx.CheckEnvelope("reader", resp.Status, resp.Body); err != nil {
			return err
		}
		st, err := parseInitialState(resp.Body)
		if err != nil {
			return err
		}
		state = st
		return nil
	})
	if err != nil {
		return nil, err
	}
	if state.BookID == "" {
		state.BookID = bookID
	}
	state.Fetched = time.Now().Unix()
	return state, nil
}

// Catalog returns the web table of contents including per-chapter tar URLs and
// the book's internal format.
func (c *Client) Catalog(ctx context.Context, bookID string) ([]weread.Chapter, string, error) {
	var chapters []weread.Chapter
	format := ""

	body := map[string]any{"bookIds": []string{bookID}}
	err := c.withRetry(ctx, func() error {
		resp, err := c.post(ctx, "/web/book/chapterInfos", body, protocol.ReaderURL(bookID, 0))
		if err != nil {
			return err
		}
		if err := httpx.CheckEnvelope("chapterInfos", resp.Status, resp.Body); err != nil {
			return err
		}
		chapters, format, err = parseCatalog(resp.Body, bookID)
		return err
	})
	if err != nil {
		return nil, "", err
	}
	return chapters, format, nil
}

// chapterFormat probes the first readable chapter: the EPUB endpoint answers
// with chapter metadata JSON for TXT books.
func (c *Client) chapterFormat(ctx context.Context, bookID string, ch weread.Chapter, st *weread.ReaderState) (string, error) {
	params := protocol.ContentParams(bookID, strconv.FormatInt(ch.UID, 10), st.PSVTS, false, 1)
	resp, err := c.post(ctx, "/web/book/chapter/e_0", params, protocol.ReaderURL(bookID, ch.UID))
	if err != nil {
		return "", err
	}
	text := string(resp.Body)
	if len(text) == 0 || text == "{}" {
		return "", fmt.Errorf("%w: 正文为空，可能无权阅读该章节", weread.ErrNotEntitled)
	}
	if strings.HasPrefix(strings.TrimSpace(text), "{") {
		if strings.Contains(text, "\"bookId\"") {
			return "txt", nil
		}
		if err := httpx.CheckEnvelope("chapter/e_0", resp.Status, resp.Body); err != nil {
			return "", err
		}
		return "", fmt.Errorf("%w: 无法识别的正文响应", weread.ErrProtocolChanged)
	}
	return "epub", nil
}

// ChapterResult is a decoded chapter ready for the content parser.
type ChapterResult struct {
	HTML   string
	CSS    string
	Format string
	Title  string
	Tar    string
}

// Chapter fetches and decodes a chapter body.
func (c *Client) Chapter(ctx context.Context, bookID string, ch weread.Chapter, st *weread.ReaderState) (*ChapterResult, error) {
	if st == nil {
		s, err := c.ReaderState(ctx, bookID, ch.UID)
		if err != nil {
			return nil, err
		}
		st = s
	}
	if st.PSVTS == "" || c.AlwaysFreshPSVTS || st.Chapter != ch.UID {
		if fresh, err := c.ReaderState(ctx, bookID, ch.UID); err == nil {
			// Keep the book-level token but take the fresh counters.
			cp := *st
			cp.PSVTS = fresh.PSVTS
			cp.PCLTS = fresh.PCLTS
			cp.Token = weread.FirstString(fresh.Token, st.Token)
			cp.Chapter = ch.UID
			st = &cp
		}
	}
	if st.PSVTS == "" {
		return nil, fmt.Errorf("%w: 未能获取阅读会话凭证（psvts 为空）", weread.ErrProtocolChanged)
	}

	format := ch.Format
	if format == "" {
		f, err := c.chapterFormat(ctx, bookID, ch, st)
		if err != nil {
			return nil, err
		}
		format = f
	}

	res := &ChapterResult{Format: format, Title: ch.Title, Tar: ch.Tar}
	err := c.withRetry(ctx, func() error {
		switch format {
		case "txt":
			html, err := c.fetchTXT(ctx, bookID, ch, st)
			if err != nil {
				return err
			}
			res.HTML = html
		default:
			html, css, err := c.fetchEPUB(ctx, bookID, ch, st)
			if err != nil {
				return err
			}
			res.HTML = html
			res.CSS = css
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (c *Client) shard(ctx context.Context, bookID string, ch weread.Chapter, st *weread.ReaderState, kind string, style bool) (string, error) {
	params := protocol.ContentParams(bookID, strconv.FormatInt(ch.UID, 10), st.PSVTS, style, 1)
	resp, err := c.post(ctx, "/web/book/chapter/"+kind, params, protocol.ReaderURL(bookID, ch.UID))
	if err != nil {
		return "", err
	}
	text := strings.TrimSpace(string(resp.Body))
	if text == "" || text == "{}" {
		return "", nil
	}
	if strings.HasPrefix(text, "{") {
		if err := httpx.CheckEnvelope("chapter/"+kind, resp.Status, resp.Body); err != nil {
			return "", err
		}
		return "", fmt.Errorf("%w: 章节 %s 返回空数据", weread.ErrNotEntitled, kind)
	}
	return text, nil
}

func (c *Client) fetchEPUB(ctx context.Context, bookID string, ch weread.Chapter, st *weread.ReaderState) (string, string, error) {
	e0, err := c.shard(ctx, bookID, ch, st, "e_0", false)
	if err != nil {
		return "", "", err
	}
	e1, err := c.shard(ctx, bookID, ch, st, "e_1", false)
	if err != nil {
		return "", "", err
	}
	e3, err := c.shard(ctx, bookID, ch, st, "e_3", false)
	if err != nil {
		return "", "", err
	}
	html, err := protocol.DecodeShards(e0, e1, e3)
	if err != nil {
		return "", "", err
	}
	css := ""
	if e2, err := c.shard(ctx, bookID, ch, st, "e_2", true); err == nil && e2 != "" {
		if decoded, derr := protocol.DecodeStyle(e2); derr == nil {
			css = decoded
		}
	}
	return html, css, nil
}

func (c *Client) fetchTXT(ctx context.Context, bookID string, ch weread.Chapter, st *weread.ReaderState) (string, error) {
	t0, err := c.shard(ctx, bookID, ch, st, "t_0", false)
	if err != nil {
		return "", err
	}
	t1, err := c.shard(ctx, bookID, ch, st, "t_1", false)
	if err != nil {
		return "", err
	}
	body, err := protocol.DecodeShards(t0, t1, "")
	if err != nil {
		return "", err
	}
	return txtToHTML(body), nil
}

func txtToHTML(body string) string {
	body = strings.ReplaceAll(body, "\r\n", "\n")
	lines := strings.Split(body, "\n")
	var b strings.Builder
	for _, line := range lines {
		line = strings.TrimRight(line, " \t")
		if strings.TrimSpace(line) == "" {
			continue
		}
		b.WriteString("<p>")
		b.WriteString(escapeHTML(line))
		b.WriteString("</p>")
	}
	return b.String()
}

func escapeHTML(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")
	return r.Replace(s)
}

// MPArticle returns the body HTML of an official-account article.
func (c *Client) MPArticle(ctx context.Context, reviewID string) (string, string, error) {
	q := url.Values{}
	q.Set("reviewId", reviewID)

	var html, title string
	err := c.withRetry(ctx, func() error {
		resp, err := c.get(ctx, "/web/mp/content", q)
		if err != nil {
			return err
		}
		if resp.Status != 200 {
			return &weread.APIError{Endpoint: "mp/content", Status: resp.Status}
		}
		html, title = extractMPBody(string(resp.Body))
		return nil
	})
	if err != nil {
		return "", "", err
	}
	if html == "" {
		return "", "", fmt.Errorf("%w: 公众号正文为空", weread.ErrProtocolChanged)
	}
	return html, title, nil
}

// ReportRead sends a signed reading/progress report.
func (c *Client) ReportRead(ctx context.Context, r protocol.ReadReport) error {
	if r.UserAgent == "" {
		r.UserAgent = protocol.DefaultUserAgent
	}
	payload := protocol.ReadPayload(r)
	resp, err := c.post(ctx, "/web/book/read", payload, protocol.ReaderURL(r.BookID, r.ChapterUID))
	if err != nil {
		return err
	}
	if err := httpx.CheckEnvelope("book/read", resp.Status, resp.Body); err != nil {
		if weread.IsAuthExpired(err) {
			if rerr := c.renew(ctx); rerr != nil {
				return err
			}
			resp, err = c.post(ctx, "/web/book/read", protocol.ReadPayload(r), protocol.ReaderURL(r.BookID, r.ChapterUID))
			if err != nil {
				return err
			}
			return httpx.CheckEnvelope("book/read", resp.Status, resp.Body)
		}
		return err
	}
	var out struct {
		Succ int `json:"succ"`
	}
	if err := json.Unmarshal(resp.Body, &out); err == nil && out.Succ != 1 {
		return errors.New("阅读进度上报被拒绝")
	}
	return nil
}

// DownloadResource fetches a chapter resource archive (images, fonts) from the
// WeRead CDN. The signed redirect target is followed even though it leaves the
// weread.qq.com host.
func (c *Client) DownloadResource(ctx context.Context, rawURL string) ([]byte, error) {
	if rawURL == "" {
		return nil, errors.New("空的资源地址")
	}
	if !strings.HasPrefix(rawURL, "https://") {
		rawURL = "https://" + strings.TrimPrefix(rawURL, "//")
	}
	var data []byte
	err := c.withRetry(ctx, func() error {
		resp, err := c.sess.Web().Do(ctx, httpx.Request{
			URL:     rawURL,
			Method:  "GET",
			Referer: session.WebBase() + "/",
		})
		if err != nil {
			return err
		}
		if resp.Status != 200 {
			return &weread.APIError{Endpoint: "resource", Status: resp.Status}
		}
		data = resp.Body
		return nil
	})
	if err != nil {
		return nil, err
	}
	c.persistCookies()
	return data, nil
}
