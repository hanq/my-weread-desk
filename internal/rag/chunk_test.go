package rag

import (
	"strings"
	"testing"

	"my-weread-desk/internal/weread"
)

func para(texts ...string) []weread.Block {
	out := make([]weread.Block, 0, len(texts))
	for _, t := range texts {
		out = append(out, weread.Block{Kind: "para", Text: t})
	}
	return out
}

func TestChunkChapterCoversEveryBlock(t *testing.T) {
	body := strings.Repeat("窗外下着雨，屋里的灯忽然暗了一下。", 8)
	blocks := para(body, body, body, body, body, body)
	chunks := ChunkChapter("book", 11, 2, "第二章", blocks, Options{TargetChars: 300, OverlapBlocks: 1, MinChars: 60})
	if len(chunks) < 2 {
		t.Fatalf("expected multiple chunks, got %d", len(chunks))
	}
	if chunks[0].BlockStart != 0 {
		t.Fatalf("first chunk must start at block 0, got %d", chunks[0].BlockStart)
	}
	if last := chunks[len(chunks)-1]; last.BlockEnd != len(blocks)-1 {
		t.Fatalf("last chunk must end at block %d, got %d", len(blocks)-1, last.BlockEnd)
	}
	for i := 1; i < len(chunks); i++ {
		if chunks[i].BlockStart > chunks[i-1].BlockEnd+1 {
			t.Fatalf("gap between chunk %d (ends %d) and %d (starts %d)",
				i-1, chunks[i-1].BlockEnd, i, chunks[i].BlockStart)
		}
	}
	for _, c := range chunks {
		if c.CharCount == 0 || c.Text == "" {
			t.Fatal("empty chunk produced")
		}
		if c.ChapterUID != 11 || c.ChapterIdx != 2 || c.ChapterTitle != "第二章" {
			t.Fatalf("chapter metadata lost: %+v", c)
		}
		if c.Seq != 0 {
			t.Fatal("ChunkChapter must leave Seq to the caller")
		}
	}
}

func TestChunkChapterSkipsNonTextBlocks(t *testing.T) {
	blocks := []weread.Block{
		{Kind: "heading", Text: "第一章"},
		{Kind: "image", Src: "/media/x.png"},
		{Kind: "para", Text: "他推开门，雨声一下子涌了进来。"},
		{Kind: "divider"},
		{Kind: "para", Text: "屋里没有人。"},
	}
	chunks := ChunkChapter("b", 1, 1, "第一章", blocks, DefaultOptions())
	if len(chunks) != 1 {
		t.Fatalf("expected one chunk, got %d", len(chunks))
	}
	if chunks[0].BlockStart != 0 || chunks[0].BlockEnd != 4 {
		t.Fatalf("block span must cover the whole chapter, got %d..%d",
			chunks[0].BlockStart, chunks[0].BlockEnd)
	}
	if !strings.Contains(chunks[0].Text, "雨声") {
		t.Fatal("text was lost")
	}
	if strings.Contains(chunks[0].Text, "图片") {
		t.Fatal("image placeholder leaked into the index")
	}
}

func TestChunkChapterDropsBoilerplate(t *testing.T) {
	blocks := para("正文一。", "微信读书 · 本书由某某出版社授权", "正文二。")
	chunks := ChunkChapter("b", 1, 1, "", blocks, DefaultOptions())
	if len(chunks) != 1 {
		t.Fatalf("expected one chunk, got %d", len(chunks))
	}
	if strings.Contains(chunks[0].Text, "微信读书") {
		t.Fatal("boilerplate was indexed")
	}
	if !strings.Contains(chunks[0].Text, "正文一") || !strings.Contains(chunks[0].Text, "正文二") {
		t.Fatalf("real text was dropped: %q", chunks[0].Text)
	}
}

func TestChunkChapterMergesTinyTail(t *testing.T) {
	long := strings.Repeat("这是一段长度足够触发换块的中文正文。", 10)
	blocks := para(long, long, "短。")
	chunks := ChunkChapter("b", 1, 1, "", blocks, Options{TargetChars: 200, OverlapBlocks: 0, MinChars: 40})
	found := false
	for _, c := range chunks {
		if strings.Contains(c.Text, "短。") {
			found = true
		}
	}
	if !found {
		t.Fatal("trailing text was lost instead of merged")
	}
}

func TestChunkBookAssignsSequentialSeq(t *testing.T) {
	chapters := []weread.Chapter{{UID: 1, Index: 1, Title: "一"}, {UID: 2, Index: 2, Title: "二"}}
	blocks := map[int64][]weread.Block{1: para("甲甲甲甲甲甲甲甲"), 2: para("乙乙乙乙乙乙乙乙")}
	chunks := ChunkBook("b", chapters, blocks, DefaultOptions())
	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks, got %d", len(chunks))
	}
	if chunks[0].Seq != 0 || chunks[1].Seq != 1 {
		t.Fatalf("seq not sequential: %d, %d", chunks[0].Seq, chunks[1].Seq)
	}
	if chunks[1].ChapterTitle != "二" {
		t.Fatalf("chapter order lost: %q", chunks[1].ChapterTitle)
	}
}
