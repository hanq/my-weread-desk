package content

import (
	"testing"
)

func TestParseBasicBlocks(t *testing.T) {
	html := `<h1>第一章　风起</h1>
		<p>　　第一段，包含 <i>斜体</i> 和 <b>粗体</b>。</p>
		<p class="center">居中一行</p>
		<blockquote>引用的话</blockquote>
		<hr/>
		<p>第二段<br/>换行后</p>`

	blocks := Parse(html, nil)
	if len(blocks) < 6 {
		t.Fatalf("expected at least 6 blocks, got %d: %+v", len(blocks), blocks)
	}
	if blocks[0].Kind != "heading" || blocks[0].Level != 1 || blocks[0].Text != "第一章 风起" {
		t.Errorf("heading = %+v", blocks[0])
	}
	if blocks[1].Kind != "para" || blocks[1].Text != "第一段，包含 斜体 和 粗体。" {
		t.Errorf("para = %+v", blocks[1])
	}
	if blocks[2].Class != "center" {
		t.Errorf("class hint = %q", blocks[2].Class)
	}
	if blocks[3].Kind != "quote" {
		t.Errorf("quote = %+v", blocks[3])
	}
	if blocks[4].Kind != "divider" {
		t.Errorf("divider = %+v", blocks[4])
	}
	if blocks[5].Text != "第二段 换行后" {
		t.Errorf("br handling = %q", blocks[5].Text)
	}
}

func TestParseImageResolution(t *testing.T) {
	html := `<p>看图：</p><img src="https://res.weread.qq.com/wrepub/epub_1_2" alt="插图"/>`
	blocks := Parse(html, func(src string) (string, bool) {
		if src == "https://res.weread.qq.com/wrepub/epub_1_2" {
			return "/media/b/epub_1_2.png", true
		}
		return "", false
	})
	if len(blocks) != 2 {
		t.Fatalf("blocks = %+v", blocks)
	}
	if blocks[1].Kind != "image" || blocks[1].Src != "/media/b/epub_1_2.png" || blocks[1].Alt != "插图" {
		t.Errorf("image = %+v", blocks[1])
	}
}

func TestParseUnresolvedImageKeepsPlaceholder(t *testing.T) {
	blocks := Parse(`<img src="https://res.weread.qq.com/wrepub/x" alt="图1"/>`, nil)
	if len(blocks) != 1 || blocks[0].Kind != "image" || blocks[0].Class != "missing" {
		t.Fatalf("blocks = %+v", blocks)
	}
}

func TestParseImageInsideParagraphWithPlaceholder(t *testing.T) {
	html := `<p>[图片]<img src="https://res.weread.qq.com/wrepub/epub_1_2" alt="插图"/></p>`
	blocks := Parse(html, func(src string) (string, bool) {
		if src == "https://res.weread.qq.com/wrepub/epub_1_2" {
			return "/media/b/epub_1_2.png", true
		}
		return "", false
	})
	if len(blocks) != 1 || blocks[0].Kind != "image" || blocks[0].Src != "/media/b/epub_1_2.png" {
		t.Fatalf("blocks = %+v", blocks)
	}
}

func TestParseCaptionAndImage(t *testing.T) {
	blocks := Parse(`<p>看图：<img src="x" alt="插图"/></p>`, func(string) (string, bool) {
		return "/media/x.png", true
	})
	if len(blocks) != 2 || blocks[0].Text != "看图：" || blocks[1].Kind != "image" {
		t.Fatalf("blocks = %+v", blocks)
	}
}

func TestParseSrcsetImage(t *testing.T) {
	blocks := Parse(`<img srcset="https://a/1.png 1x, https://a/2.png 2x" alt="图"/>`, func(src string) (string, bool) {
		if src == "https://a/1.png" {
			return "/media/1.png", true
		}
		return "", false
	})
	if len(blocks) != 1 || blocks[0].Src != "/media/1.png" {
		t.Fatalf("blocks = %+v", blocks)
	}
}

func TestParseListsAndTables(t *testing.T) {
	html := `<ul><li>甲</li><li>乙</li></ul>
		<table><tr><td>1990</td><td>3.8</td></tr><tr><th>年份</th><th>增速</th></tr></table>`
	blocks := Parse(html, nil)
	if len(blocks) != 4 {
		t.Fatalf("blocks = %+v", blocks)
	}
	if blocks[0].Text != "• 甲" || blocks[1].Text != "• 乙" {
		t.Errorf("list = %+v", blocks[:2])
	}
	if blocks[2].Text != "1990 · 3.8" || blocks[3].Text != "年份 · 增速" {
		t.Errorf("table = %+v", blocks[2:])
	}
}

func TestParseNestedDivWithoutBlocksBecomesParagraph(t *testing.T) {
	blocks := Parse(`<div><span>只有行内内容</span></div>`, nil)
	if len(blocks) != 1 || blocks[0].Text != "只有行内内容" {
		t.Fatalf("blocks = %+v", blocks)
	}
}

func TestParseSkipsScriptAndStyle(t *testing.T) {
	blocks := Parse(`<style>p{color:red}</style><p>正文</p><script>alert(1)</script>`, nil)
	if len(blocks) != 1 || blocks[0].Text != "正文" {
		t.Fatalf("blocks = %+v", blocks)
	}
}

func TestParseEntitiesAndNBSP(t *testing.T) {
	blocks := Parse(`<p>a&nbsp;b &amp; c &#8212; d</p>`, nil)
	if len(blocks) != 1 {
		t.Fatalf("blocks = %+v", blocks)
	}
	if blocks[0].Text != "a b & c — d" {
		t.Errorf("entities = %q", blocks[0].Text)
	}
}
