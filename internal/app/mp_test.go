package app

import (
	"strings"
	"testing"

	"my-weread-desk/internal/weread"
)

func TestMPAccountID(t *testing.T) {
	got := mpAccountID("MP_WXS_3286016687_9gm5eWle7VrEYNiwBGaaOQ")
	if got != "MP_WXS_3286016687" {
		t.Fatalf("mpAccountID = %q", got)
	}
	if mpAccountID("907755") != "" {
		t.Fatal("regular book should not have an MP account")
	}
}

func TestRewriteMPImageSources(t *testing.T) {
	blocks := []weread.Block{
		{Kind: "image", Src: "https://mmbiz.qpic.cn/a.jpg"},
		{Kind: "image", Src: "/media/MP_WXS_1/cover.jpg"},
		{Kind: "para", Text: "正文"},
	}
	rewriteMPImageSources(blocks, "MP_WXS_1")
	if !strings.HasPrefix(blocks[0].Src, "/media-proxy?") {
		t.Fatalf("image source not proxied: %q", blocks[0].Src)
	}
	if blocks[1].Src != "/media/MP_WXS_1/cover.jpg" {
		t.Fatalf("existing local image changed: %q", blocks[1].Src)
	}
}

func TestMPBlocksNeedRefresh(t *testing.T) {
	if !mpBlocksNeedRefresh([]weread.Block{{Kind: "image", Src: "data:image/gif;base64,AA"}}) {
		t.Fatal("data URI placeholder should be refreshed")
	}
	if mpBlocksNeedRefresh([]weread.Block{{Kind: "image", Src: "https://mmbiz.qpic.cn/a.jpg"}}) {
		t.Fatal("direct remote image can be rewritten without refetching")
	}
}
