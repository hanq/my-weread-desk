// Command audio_probe inspects the audiobook endpoints with the credentials the
// desktop app already stored.
//
// It prints response shapes and counts only: tokens, cookies and book text never
// reach stdout. The goal is to answer one question before any player work
// starts — can a track be played, and does it come with a transcript that could
// be indexed for retrieval.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"my-weread-desk/internal/config"
	"my-weread-desk/internal/weread/auth"
	"my-weread-desk/internal/weread/eink"
	"my-weread-desk/internal/weread/session"
	"my-weread-desk/internal/weread/web"
)

func main() {
	var (
		albumID = flag.String("album", "", "专辑 ID；留空则从书架取第一本有声书")
		trackID = flag.String("track", "", "单集 trackId；留空则从专辑曲目取第一条")
		bookID  = flag.String("book", "", "电子书 ID，用于 /book/album")
		search  = flag.String("search", "", "书架没有有声书时，用该关键词搜索商店的音频分类")
		scope   = flag.String("scope", "audio", "商店搜索分类：audio 或 all")
		timeout = flag.Duration("timeout", 25*time.Second, "单次请求超时")
	)
	flag.Parse()

	dataDir, err := config.DataDir()
	if err != nil {
		fail("读取数据目录失败: %v", err)
	}
	credStore, err := auth.NewStore(filepath.Join(dataDir, "account"))
	if err != nil {
		fail("打开凭据库失败: %v", err)
	}
	sess := session.New(credStore)
	if !sess.LoggedIn() {
		fmt.Println("未登录：请先在应用里扫码登录后再运行。")
		return
	}
	client := eink.New(sess)
	webClient := web.New(sess)

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	if *albumID == "" || *bookID == "" {
		books, groups, err := client.ShelfSnapshot(ctx)
		if err != nil {
			fmt.Printf("书架读取失败: %v\n", err)
		}
		fmt.Printf("书架: %d 项，分组: %d 个\n", len(books), len(groups))
		for _, g := range groups {
			fmt.Printf("  分组 %q source=%s books=%d\n", g.Name, g.Source, len(g.BookIDs))
		}
		audio := 0
		for _, b := range books {
			if b.IsAudio {
				audio++
				if *albumID == "" {
					*albumID = b.ID
				}
			}
		}
		fmt.Printf("有声书: %d 本\n", audio)
		if *bookID == "" {
			for _, b := range books {
				if !b.IsAudio && !b.IsMP {
					*bookID = b.ID
					break
				}
			}
		}
	}

	// An account can have no audiobooks on its shelf while the store still sells
	// them. Searching scope=audio yields an album id without touching the shelf.
	if *albumID == "" && *search != "" {
		res, err := client.Search(ctx, *search, *scope, 0, 20)
		if err != nil {
			fmt.Printf("商店搜索失败: %v\n", err)
		} else {
			fmt.Printf("商店搜索 %q scope=%s: %d 条\n", *search, *scope, len(res.Books))
			for i, b := range res.Books {
				if i >= 5 {
					break
				}
				fmt.Printf("  [%d] id=%s title=%q author=%q\n", i, b.ID, b.Title, b.Author)
			}
			if len(res.Books) > 0 {
				*albumID = res.Books[0].ID
			}
		}
		// The E-Ink store index is frequently empty for accounts that logged in
		// through the browser flow. The web store serves the same catalogue and
		// is the documented fallback elsewhere in the app.
		if len(res.Books) == 0 && sess.Credentials().HasWebSession() {
			if webRes, werr := webClient.Search(ctx, *search, 0, 20); werr != nil {
				fmt.Printf("网页商店搜索失败: %v\n", werr)
			} else {
				fmt.Printf("网页商店搜索: %d 条\n", len(webRes.Books))
				for i, b := range webRes.Books {
					if i >= 5 {
						break
					}
					fmt.Printf("  [%d] id=%s title=%q author=%q\n", i, b.ID, b.Title, b.Author)
				}
			}
		}
	}

	fmt.Println()
	verdict := map[string]int{}

	if *albumID != "" {
		fmt.Printf("专辑: %s\n", *albumID)
		_, _ = run(ctx, client, "album/info", "/album/info", url.Values{"albumId": {*albumID}})
		_, list := run(ctx, client, "album/list", "/album/list",
			url.Values{"albumId": {*albumID}, "synckey": {"0"}})
		if *trackID == "" && list != nil {
			var found []string
			walkStrings(list, map[string]bool{"trackId": true, "track_id": true, "audioId": true}, &found)
			if len(found) > 0 {
				*trackID = found[0]
			}
		}
	} else {
		fmt.Println("书架里没有有声书，可用 -album 指定一个专辑 ID。")
	}

	if *trackID != "" {
		fmt.Printf("\n单集: %s\n", *trackID)
		verdict["track/info"], _ = run(ctx, client, "track/info", "/track/info",
			url.Values{"trackId": {*trackID}})
		verdict["track/text"], _ = run(ctx, client, "track/text", "/track/text",
			url.Values{"trackId": {*trackID}})
		verdict["audio/geturl"], _ = run(ctx, client, "audio/geturl", "/audio/geturl",
			url.Values{"audioId": {*trackID}, "type": {"1"}, "quality": {"1"}})
	}

	if *bookID != "" {
		fmt.Printf("\n电子书: %s\n", *bookID)
		run(ctx, client, "book/album", "/book/album", url.Values{"bookId": {*bookID}})
	}

	fmt.Println("\n结论")
	if code, ok := verdict["track/text"]; ok && code == 200 {
		fmt.Println("  /track/text 返回 200：有声书可能带文字稿，可进入 RAG。")
	} else {
		fmt.Println("  /track/text 未返回可用正文：有声书与 RAG 无缘，只做元数据展示。")
	}
	if code, ok := verdict["audio/geturl"]; ok && code == 200 {
		fmt.Println("  /audio/geturl 返回 200：播放链路值得进一步验证防盗链与格式。")
	} else {
		fmt.Println("  /audio/geturl 不可用：不要投入内嵌播放器。")
	}
}

// run performs one probe, prints its shape and returns the HTTP status together
// with the decoded body so the caller can pull an id out of a listing.
func run(ctx context.Context, c *eink.Client, name, path string, q url.Values) (int, any) {
	start := time.Now()
	resp, err := c.ProbeRaw(ctx, path, q)
	cost := time.Since(start).Milliseconds()
	if err != nil {
		fmt.Printf("  %-14s 请求失败 (%dms): %v\n", name, cost, err)
		return 0, nil
	}
	var decoded any
	_ = json.Unmarshal(resp.Body, &decoded)
	fmt.Printf("  %-14s HTTP %d  %dB  %dms\n", name, resp.Status, len(resp.Body), cost)
	if decoded != nil {
		fmt.Printf("  %-14s shape: %s\n", "", shape(decoded, 3))
		if code, msg, ok := errEnvelope(decoded); ok {
			fmt.Printf("  %-14s errcode=%d %s\n", "", code, msg)
		}
	}
	return resp.Status, decoded
}

// shape describes a decoded JSON value without printing its content. String
// values are reported as lengths so a transcript body can be recognised without
// leaking a single word of it.
func shape(v any, depth int) string {
	if depth <= 0 {
		return "…"
	}
	switch t := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		if len(keys) > 40 {
			keys = keys[:40]
		}
		parts := make([]string, 0, len(keys))
		for _, k := range keys {
			parts = append(parts, k+":"+shape(t[k], depth-1))
		}
		return "{" + strings.Join(parts, ", ") + "}"
	case []any:
		if len(t) == 0 {
			return "[]"
		}
		return fmt.Sprintf("[%s ×%d]", shape(t[0], depth-1), len(t))
	case string:
		return fmt.Sprintf("string(%d)", len([]rune(t)))
	case float64:
		return "number"
	case bool:
		return "bool"
	case nil:
		return "null"
	default:
		return fmt.Sprintf("%T", v)
	}
}

func errEnvelope(v any) (int, string, bool) {
	m, ok := v.(map[string]any)
	if !ok {
		return 0, "", false
	}
	raw, exists := m["errcode"]
	if !exists {
		return 0, "", false
	}
	code, ok := raw.(float64)
	if !ok || code == 0 {
		return 0, "", false
	}
	msg, _ := m["errmsg"].(string)
	return int(code), msg, true
}

func walkStrings(v any, want map[string]bool, out *[]string) {
	switch t := v.(type) {
	case map[string]any:
		for k, val := range t {
			if want[k] {
				if s, ok := val.(string); ok && s != "" {
					*out = append(*out, s)
				}
			}
			walkStrings(val, want, out)
		}
	case []any:
		for _, item := range t {
			walkStrings(item, want, out)
		}
	}
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
