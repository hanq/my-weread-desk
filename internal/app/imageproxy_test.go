package app

import (
	"strings"
	"testing"
)

func hasKey(keys []string, want string) bool {
	for _, key := range keys {
		if key == want {
			return true
		}
	}
	return false
}

func TestImageLookupKeysExtensionless(t *testing.T) {
	keys := imageLookupKeys("epub_43208843_44")
	if !hasKey(keys, "epub_43208843_44") {
		t.Fatalf("missing the original key: %v", keys)
	}
	if !hasKey(keys, "epub_43208843_44.png") {
		t.Fatalf("missing the tar extension variant: %v", keys)
	}
	// path.Ext would treat ".44" as the extension and produce this bogus stem.
	if hasKey(keys, "epub_43208843") {
		t.Fatalf("path.Ext-style stem leaked into the keys: %v", keys)
	}
}

func TestImageLookupKeysKnownExtension(t *testing.T) {
	keys := imageLookupKeys("epub_1_2.PNG")
	if !hasKey(keys, "epub_1_2.PNG") || !hasKey(keys, "epub_1_2") {
		t.Fatalf("keys = %v", keys)
	}
	if hasKey(keys, "epub_1") {
		t.Fatalf("extension stripping was too aggressive: %v", keys)
	}
}

func TestAllowedImageHost(t *testing.T) {
	for _, host := range []string{"res.weread.qq.com", "weread.qq.com", "cdn.weread.qq.com", "mmbiz.qpic.cn"} {
		if !allowedImageHost(host) {
			t.Fatalf("expected %q to be allowed", host)
		}
	}
	for _, host := range []string{"", "evil.com", "weread.qq.com.evil.com", "127.0.0.1"} {
		if allowedImageHost(host) {
			t.Fatalf("expected %q to be rejected", host)
		}
	}
}

func TestRemoteImageSrc(t *testing.T) {
	got, ok := remoteImageSrc("//res.weread.qq.com/wrepub/epub_43208843_44")
	if !ok || got != "https://res.weread.qq.com/wrepub/epub_43208843_44" {
		t.Fatalf("remoteImageSrc = %q, %v", got, ok)
	}
	if _, ok := remoteImageSrc("javascript:alert(1)"); ok {
		t.Fatal("javascript URL must be rejected")
	}
	if _, ok := remoteImageSrc("https://evil.com/a.png"); ok {
		t.Fatal("foreign host must be rejected")
	}
}

func TestImageProxyURL(t *testing.T) {
	u := imageProxyURL("43208843", 44, "https://res.weread.qq.com/wrepub/epub_43208843_44")
	if !strings.HasPrefix(u, "/media-proxy?") {
		t.Fatalf("url = %q", u)
	}
	for _, want := range []string{"book=43208843", "uid=44", "src=https%3A%2F%2Fres.weread.qq.com"} {
		if !strings.Contains(u, want) {
			t.Fatalf("url %q does not contain %q", u, want)
		}
	}
}

func TestImageProxyKeyStable(t *testing.T) {
	a := imageProxyKey("https://res.weread.qq.com/wrepub/epub_1")
	b := imageProxyKey("https://res.weread.qq.com/wrepub/epub_1")
	c := imageProxyKey("https://res.weread.qq.com/wrepub/epub_2")
	if a != b || a == c {
		t.Fatalf("keys = %q, %q, %q", a, b, c)
	}
}
