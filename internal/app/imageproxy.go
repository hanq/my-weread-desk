package app

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"net/url"
	"strconv"
	"strings"
	"time"

	"my-weread-desk/internal/weread/protocol"
)

// knownImageExts is also used to map an extensionless tar entry to an
// extensionless XHTML image reference. WeRead's wrepub URLs omit the suffix
// while the tar file usually has one.
var knownImageExts = []string{".png", ".jpg", ".jpeg", ".gif", ".webp", ".bmp", ".svg", ".avif"}

// imageLookupKeys returns every reasonable key for one image reference. The
// XHTML reference and the tar entry may differ only by an extension, and the
// reference may contain dots in its own name (epub_43208843_44), so path.Ext
// must not be used here: it would treat ".44" as the extension.
func imageLookupKeys(name string) []string {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	keys := make([]string, 0, 2+2*len(knownImageExts))
	seen := make(map[string]bool, len(keys))
	add := func(key string) {
		if key == "" || seen[key] {
			return
		}
		seen[key] = true
		keys = append(keys, key)
	}

	add(name)
	add(strings.ToLower(name))

	lower := strings.ToLower(name)
	matched := ""
	for _, ext := range knownImageExts {
		if strings.HasSuffix(lower, ext) {
			matched = ext
			break
		}
	}
	if matched != "" {
		stem := name[:len(name)-len(matched)]
		add(stem)
		add(strings.ToLower(stem))
		return keys
	}

	// The URL has no extension: try the extensions the archive may use.
	for _, ext := range knownImageExts {
		add(name + ext)
		add(strings.ToLower(name + ext))
	}
	return keys
}

// allowedImageHost keeps the proxy from becoming an SSRF primitive. Only the
// WeRead image CDN and the main site are accepted.
func allowedImageHost(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" {
		return false
	}
	return host == "res.weread.qq.com" ||
		host == "weread.qq.com" ||
		host == "mmbiz.qpic.cn" ||
		strings.HasSuffix(host, ".weread.qq.com")
}

func normalizeImageURL(src string) string {
	src = strings.TrimSpace(src)
	if strings.HasPrefix(src, "//") {
		return "https:" + src
	}
	return src
}

// remoteImageSrc reports whether an image reference points at an absolute
// WeRead-hosted URL that the Go backend can fetch with the account cookies.
func remoteImageSrc(src string) (string, bool) {
	src = normalizeImageURL(src)
	u, err := url.Parse(src)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") {
		return "", false
	}
	if !allowedImageHost(u.Hostname()) {
		return "", false
	}
	return src, true
}

func imageProxyURL(bookID string, chapterUID int64, src string) string {
	q := url.Values{}
	q.Set("book", bookID)
	q.Set("uid", strconv.FormatInt(chapterUID, 10))
	q.Set("src", src)
	return "/media-proxy?" + q.Encode()
}

func imageProxyKey(src string) string {
	sum := sha1.Sum([]byte(src))
	return "proxy_" + hex.EncodeToString(sum[:])
}

// ProxyImage fetches an original WeRead image through the authenticated web
// session. The chapter tar archive is the primary source; this is the fallback
// used when the archive is missing, incomplete, or cannot be downloaded. The
// renderer cannot fetch res.weread.qq.com by itself because it has no cookies.
func (a *App) proxyImage(ctx context.Context, bookID string, chapterUID int64, src string) ([]byte, string, error) {
	src = normalizeImageURL(src)
	u, err := url.Parse(src)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || !allowedImageHost(u.Hostname()) {
		return nil, "", errors.New("图片地址不在允许范围")
	}
	if strings.TrimSpace(bookID) == "" {
		return nil, "", errors.New("缺少书籍 ID")
	}

	key := imageProxyKey(src)
	if data, mime, err := a.store.Media(bookID, key); err == nil {
		return data, mime, nil
	}

	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	referer := protocol.ReaderURL(bookID, chapterUID)
	if protocol.IsMPBook(bookID) {
		referer = protocol.MPReaderURL(bookID)
	}
	data, err := a.web.DownloadResourceAs(ctx, src, referer)
	if err != nil {
		a.logf("图片代理失败：book=%s src=%s err=%v", bookID, src, err)
		return nil, "", err
	}
	if len(data) == 0 {
		a.logf("图片代理失败：book=%s src=%s err=空内容", bookID, src)
		return nil, "", errors.New("图片内容为空")
	}
	if len(data) > 24<<20 {
		return nil, "", errors.New("图片过大")
	}
	ext := imageExt(data)
	if ext == ".bin" {
		a.logf("图片代理失败：book=%s src=%s err=不是图片", bookID, src)
		return nil, "", errors.New("返回内容不是图片")
	}
	mime := imageMime(ext)
	if err := a.store.SaveMedia(bookID, key, mime, data); err != nil {
		a.logf("缓存代理图片失败：book=%s err=%v", bookID, err)
	}
	return data, mime, nil
}

// ProxyImage is the asset-server entry point. Keeping it a package function
// (instead of an App method) stops Wails from exposing it as a JS binding.
func ProxyImage(ctx context.Context, application *App, bookID string, chapterUID int64, src string) ([]byte, string, error) {
	return application.proxyImage(ctx, bookID, chapterUID, src)
}
