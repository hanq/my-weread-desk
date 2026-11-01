package app

import (
	"testing"

	"my-weread-desk/internal/weread"
)

func TestIsFrontMatterChapter(t *testing.T) {
	cases := map[string]bool{
		"封面":         true,
		"版权信息":       true,
		"文前":         true,
		"内容提要":       true,
		"推荐一":        true,
		"推荐序":        true,
		"推荐语":        true,
		"目录":         true,
		"推荐系统实战":     false,
		"第一章 能量守恒理论": false,
		"前言":         false,
	}
	for title, want := range cases {
		if got := isFrontMatterChapter(title); got != want {
			t.Fatalf("isFrontMatterChapter(%q) = %v, want %v", title, got, want)
		}
	}
}

func TestPickStartChapterSkipsFrontMatter(t *testing.T) {
	chapters := []weread.Chapter{
		{UID: 1, Title: "封面"},
		{UID: 2, Title: "版权信息"},
		{UID: 3, Title: "推荐一"},
		{UID: 4, Title: "第一章 能量守恒理论"},
	}
	got, ok := pickStartChapter(chapters)
	if !ok || got.UID != 4 {
		t.Fatalf("pickStartChapter = %+v, %v", got, ok)
	}

	// An explicit user choice is still respected, even when it is a cover.
	explicit, ok := pickChapter(chapters, 1)
	if !ok || explicit.UID != 1 {
		t.Fatalf("pickChapter explicit = %+v, %v", explicit, ok)
	}

	allFront := []weread.Chapter{{UID: 1, Title: "封面"}, {UID: 2, Title: "版权信息"}}
	fallback, ok := pickStartChapter(allFront)
	if !ok || fallback.UID != 1 {
		t.Fatalf("all-front fallback = %+v, %v", fallback, ok)
	}
}
