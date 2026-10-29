package web

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"path"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"

	"my-weread-desk/internal/weread"
)

// parseInitialState extracts and decodes window.__INITIAL_STATE__.
func parseInitialState(page []byte) (*weread.ReaderState, error) {
	marker := []byte("window.__INITIAL_STATE__")
	idx := bytes.Index(page, marker)
	if idx < 0 {
		return nil, fmt.Errorf("%w: 阅读页缺少 __INITIAL_STATE__", weread.ErrProtocolChanged)
	}
	rest := page[idx+len(marker):]
	start := bytes.IndexByte(rest, '{')
	if start < 0 {
		return nil, fmt.Errorf("%w: __INITIAL_STATE__ 结构异常", weread.ErrProtocolChanged)
	}
	obj, ok := scanBalancedJSON(rest[start:])
	if !ok {
		return nil, fmt.Errorf("%w: __INITIAL_STATE__ 不是合法 JSON", weread.ErrProtocolChanged)
	}

	var raw struct {
		Reader struct {
			BookInfo struct {
				BookID weread.FlexString `json:"bookId"`
				Title  string            `json:"title"`
				Author string            `json:"author"`
			} `json:"bookInfo"`
			PSVTS   string `json:"psvts"`
			PCLTS   string `json:"pclts"`
			Token   string `json:"token"`
			Chapter struct {
				ChapterUID weread.FlexInt `json:"chapterUid"`
			} `json:"currentChapter"`
		} `json:"reader"`
	}
	if err := json.Unmarshal(obj, &raw); err != nil {
		return nil, fmt.Errorf("%w: %v", weread.ErrProtocolChanged, err)
	}

	st := &weread.ReaderState{
		BookID:  raw.Reader.BookInfo.BookID.String(),
		Title:   raw.Reader.BookInfo.Title,
		Author:  raw.Reader.BookInfo.Author,
		PSVTS:   raw.Reader.PSVTS,
		PCLTS:   raw.Reader.PCLTS,
		Token:   raw.Reader.Token,
		Chapter: raw.Reader.Chapter.ChapterUID.Int64(),
	}
	return st, nil
}

// scanBalancedJSON returns the first complete JSON object in data, respecting
// strings and escapes.
func scanBalancedJSON(data []byte) ([]byte, bool) {
	depth := 0
	inString := false
	escaped := false
	for i := 0; i < len(data); i++ {
		c := data[i]
		if inString {
			switch {
			case escaped:
				escaped = false
			case c == '\\':
				escaped = true
			case c == '"':
				inString = false
			}
			continue
		}
		switch c {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return data[:i+1], true
			}
		}
	}
	return nil, false
}

type chapterJSON struct {
	ChapterUID weread.FlexInt `json:"chapterUid"`
	ChapterIdx weread.FlexInt `json:"chapterIdx"`
	Title      string         `json:"title"`
	WordCount  weread.FlexInt `json:"wordCount"`
	Level      weread.FlexInt `json:"level"`
	Price      weread.FlexInt `json:"price"`
	Paid       weread.FlexInt `json:"paid"`
	Tar        string         `json:"tar"`
	Files      []string       `json:"files"`
}

type catalogEntry struct {
	BookID   weread.FlexString `json:"bookId"`
	Format   string            `json:"format"`
	Synckey  weread.FlexInt    `json:"synckey"`
	Updated  []chapterJSON     `json:"updated"`
	Chapters []chapterJSON     `json:"chapters"`
	Book     struct {
		BookID weread.FlexString `json:"bookId"`
		Format string            `json:"format"`
	} `json:"book"`
}

// parseCatalog normalises the several shapes /web/book/chapterInfos has used.
func parseCatalog(body []byte, bookID string) ([]weread.Chapter, string, error) {
	entries, err := catalogEntries(body)
	if err != nil {
		return nil, "", err
	}
	for _, e := range entries {
		id := weread.FirstString(e.BookID.String(), e.Book.BookID.String())
		if id != "" && id != bookID {
			continue
		}
		list := e.Updated
		if len(list) == 0 {
			list = e.Chapters
		}
		if len(list) == 0 {
			continue
		}
		out := make([]weread.Chapter, 0, len(list))
		for i, c := range list {
			if c.ChapterUID.Int64() == 0 {
				continue
			}
			idx := int(c.ChapterIdx.Int64())
			if idx == 0 {
				idx = i + 1
			}
			out = append(out, weread.Chapter{
				UID:       c.ChapterUID.Int64(),
				Index:     idx,
				Title:     c.Title,
				Level:     int(c.Level.Int64()),
				WordCount: int(c.WordCount.Int64()),
				Price:     int(c.Price.Int64()),
				Paid:      c.Paid.Int64() != 0,
				Tar:       c.Tar,
				Format:    weread.FirstString(e.Format, e.Book.Format),
			})
		}
		if len(out) > 0 {
			return out, weread.FirstString(e.Format, e.Book.Format), nil
		}
	}
	return nil, "", fmt.Errorf("%w: 目录为空", weread.ErrProtocolChanged)
}

func catalogEntries(body []byte) ([]catalogEntry, error) {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return nil, fmt.Errorf("%w: 目录响应为空", weread.ErrProtocolChanged)
	}
	switch trimmed[0] {
	case '[':
		var list []catalogEntry
		if err := json.Unmarshal(trimmed, &list); err != nil {
			return nil, fmt.Errorf("%w: %v", weread.ErrProtocolChanged, err)
		}
		return list, nil
	case '{':
		var wrapper struct {
			Data  []catalogEntry `json:"data"`
			Books []catalogEntry `json:"books"`
			catalogEntry
		}
		if err := json.Unmarshal(trimmed, &wrapper); err != nil {
			return nil, fmt.Errorf("%w: %v", weread.ErrProtocolChanged, err)
		}
		out := wrapper.Data
		if len(out) == 0 {
			out = wrapper.Books
		}
		if len(out) == 0 && (wrapper.Updated != nil || wrapper.Chapters != nil) {
			out = []catalogEntry{wrapper.catalogEntry}
		}
		if len(out) == 0 {
			return nil, fmt.Errorf("%w: 目录响应结构未知", weread.ErrProtocolChanged)
		}
		return out, nil
	}
	return nil, fmt.Errorf("%w: 目录响应不是 JSON", weread.ErrProtocolChanged)
}

// extractMPBody pulls the article body out of a WeChat official-account page.
func extractMPBody(page string) (string, string) {
	doc, err := html.Parse(strings.NewReader(page))
	if err != nil {
		return "", ""
	}
	title := ""
	if t := findFirst(doc, func(n *html.Node) bool {
		return n.DataAtom == atom.Title
	}); t != nil && t.FirstChild != nil {
		title = strings.TrimSpace(t.FirstChild.Data)
	}

	body := findFirst(doc, func(n *html.Node) bool {
		if n.Type != html.ElementNode {
			return false
		}
		if n.DataAtom == atom.Div {
			if id := attr(n, "id"); id == "js_content" {
				return true
			}
			if cls := attr(n, "class"); strings.Contains(cls, "rich_media_content") {
				return true
			}
		}
		return false
	})
	if body == nil {
		return "", title
	}
	var buf bytes.Buffer
	for c := body.FirstChild; c != nil; c = c.NextSibling {
		if err := html.Render(&buf, c); err != nil {
			return "", title
		}
	}
	if title == "" {
		title = attr(body, "data-title")
	}
	return buf.String(), title
}

func findFirst(n *html.Node, pred func(*html.Node) bool) *html.Node {
	if pred(n) {
		return n
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if found := findFirst(c, pred); found != nil {
			return found
		}
	}
	return nil
}

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if strings.EqualFold(a.Key, key) {
			return a.Val
		}
	}
	return ""
}

// ParseTar extracts every regular file of a chapter resource archive keyed by
// base name.
func ParseTar(data []byte) (map[string][]byte, error) {
	if len(data) == 0 {
		return nil, nil
	}
	out := map[string][]byte{}
	tr := tar.NewReader(bytes.NewReader(data))
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			// Truncated archives still yield the entries read so far.
			break
		}
		if hdr.Typeflag != tar.TypeReg && hdr.Typeflag != tar.TypeRegA {
			continue
		}
		buf, err := io.ReadAll(io.LimitReader(tr, 32<<20))
		if err != nil {
			continue
		}
		name := path.Base(strings.ReplaceAll(hdr.Name, "\\", "/"))
		if name == "" || name == "." || name == "/" {
			continue
		}
		out[name] = buf
	}
	return out, nil
}

// MediaName normalises an image reference to the archive base name.
func MediaName(src string) string {
	if src == "" {
		return ""
	}
	u, err := url.Parse(src)
	if err == nil && u.Path != "" {
		src = u.Path
	}
	return path.Base(strings.ReplaceAll(src, "\\", "/"))
}

// ImageExt detects an extension from magic bytes.
func ImageExt(data []byte) string {
	switch {
	case bytes.HasPrefix(data, []byte("\x89PNG\r\n\x1a\n")):
		return ".png"
	case bytes.HasPrefix(data, []byte("\xff\xd8\xff")):
		return ".jpg"
	case bytes.HasPrefix(data, []byte("GIF87a")), bytes.HasPrefix(data, []byte("GIF89a")):
		return ".gif"
	case len(data) > 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP":
		return ".webp"
	case bytes.HasPrefix(data, []byte("BM")):
		return ".bmp"
	case bytes.HasPrefix(data, []byte("<svg")), bytes.HasPrefix(data, []byte("<?xml")):
		return ".svg"
	}
	return ".bin"
}

// ImageMime maps an extension to a MIME type.
func ImageMime(ext string) string {
	switch strings.ToLower(ext) {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".bmp":
		return "image/bmp"
	case ".svg":
		return "image/svg+xml"
	}
	return "application/octet-stream"
}
