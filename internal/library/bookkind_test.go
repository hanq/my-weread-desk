package library

import (
	"testing"

	"my-weread-desk/internal/weread"
)

func TestClassifyBook(t *testing.T) {
	cases := []struct {
		name string
		book weread.Book
		want BookKind
	}{
		{"novel category", weread.Book{Category: "小说", Title: "三体"}, BookKindFiction},
		{"science fiction", weread.Book{Category: "科幻", Title: "银河帝国"}, BookKindFiction},
		{"literary essay collection", weread.Book{Category: "文学", Title: "我们仨", Intro: "散文集"}, BookKindNonfiction},
		{"fiction from intro", weread.Book{Title: "三体", Intro: "一部科幻小说"}, BookKindFiction},
		{"textbook", weread.Book{Category: "教材", Title: "高等数学"}, BookKindTextbook},
		{"exam prep", weread.Book{Category: "考试", Title: "考研英语真题"}, BookKindTextbook},
		{"medical", weread.Book{Category: "医学", Title: "内科学"}, BookKindMedical},
		{"medical imaging wins over medical", weread.Book{Category: "医学", Title: "医学影像学"}, BookKindImaging},
		{"periodical", weread.Book{Category: "期刊", Title: "读书"}, BookKindPeriodical},
		{"history", weread.Book{Category: "历史", Title: "万历十五年"}, BookKindNonfiction},
		{"mp is other", weread.Book{IsMP: true, Category: "小说"}, BookKindOther},
		{"audio is other", weread.Book{IsAudio: true, Category: "小说"}, BookKindOther},
		{"unknown", weread.Book{Title: "未分类的书"}, BookKindOther},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ClassifyBook(tc.book); got != tc.want {
				t.Fatalf("ClassifyBook() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestBookKindValidAndLabel(t *testing.T) {
	for _, kind := range []BookKind{
		BookKindFiction, BookKindNonfiction, BookKindTextbook, BookKindPeriodical,
		BookKindMedical, BookKindImaging, BookKindOther,
	} {
		if !kind.Valid() {
			t.Fatalf("%q should be valid", kind)
		}
		if BookKindLabel(string(kind)) == "" {
			t.Fatalf("%q has no label", kind)
		}
	}
	if BookKind("novel").Valid() {
		t.Fatal("unknown kind should be invalid")
	}
	if got := BookKindLabel(string(BookKindFiction)); got != "小说" {
		t.Fatalf("fiction label = %q", got)
	}
}
