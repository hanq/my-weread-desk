package library

import (
	"strings"

	"my-weread-desk/internal/weread"
)

// BookKind is the AI profile of a book. It decides which AI modes are allowed:
// story traversal and world-line rewriting only make sense for fiction.
type BookKind string

const (
	BookKindFiction    BookKind = "fiction"
	BookKindNonfiction BookKind = "nonfiction"
	BookKindTextbook   BookKind = "textbook"
	BookKindPeriodical BookKind = "periodical"
	BookKindMedical    BookKind = "medical"
	BookKindImaging    BookKind = "imaging"
	BookKindOther      BookKind = "other"
)

// Valid reports whether kind is one of the stored values.
func (k BookKind) Valid() bool {
	switch k {
	case BookKindFiction, BookKindNonfiction, BookKindTextbook, BookKindPeriodical,
		BookKindMedical, BookKindImaging, BookKindOther:
		return true
	}
	return false
}

// BookKindLabel is the user-facing name shown in errors and the UI.
func BookKindLabel(kind string) string {
	switch BookKind(kind) {
	case BookKindFiction:
		return "小说"
	case BookKindNonfiction:
		return "非虚构"
	case BookKindTextbook:
		return "教材"
	case BookKindPeriodical:
		return "期刊"
	case BookKindMedical:
		return "医学"
	case BookKindImaging:
		return "影像"
	default:
		return "其他"
	}
}

// ClassifyBook infers a kind from the metadata WeRead returns.
//
// The category string is free-form, so this is deliberately conservative: a
// false negative ("nonfiction" for a novel) only hides an optional feature,
// while a false positive would let the story engine rewrite a textbook. The
// reader can always correct the kind in the book detail page.
func ClassifyBook(b weread.Book) BookKind {
	if b.IsMP || b.IsAudio {
		return BookKindOther
	}

	category := strings.ToLower(strings.TrimSpace(b.Category))
	text := strings.ToLower(strings.Join([]string{b.Category, b.Title, b.Intro}, " "))

	// A category that explicitly says "novel" is stronger than anything in the
	// blurb. This keeps a medical romance out of the medical bucket.
	if containsAny(category, fictionTerms) {
		return BookKindFiction
	}
	if containsAny(text, imagingTerms) {
		return BookKindImaging
	}
	if containsAny(text, medicalTerms) {
		return BookKindMedical
	}
	if containsAny(text, textbookTerms) {
		return BookKindTextbook
	}
	if containsAny(text, periodicalTerms) {
		return BookKindPeriodical
	}
	if containsAny(text, fictionTerms) {
		return BookKindFiction
	}
	if containsAny(text, nonfictionTerms) {
		return BookKindNonfiction
	}
	return BookKindOther
}

func containsAny(text string, terms []string) bool {
	for _, term := range terms {
		if strings.Contains(text, term) {
			return true
		}
	}
	return false
}

var (
	imagingTerms = []string{
		"影像", "放射", "超声", "核磁", "造影", "介入", "x线", "x光",
	}
	medicalTerms = []string{
		"医学", "临床", "病理", "药理", "药学", "诊断", "护理", "解剖",
		"生理", "内科", "外科", "儿科", "妇产", "中医",
	}
	textbookTerms = []string{
		"教材", "教辅", "教科书", "习题", "练习册", "考试", "高考", "考研",
		"辅导", "教程", "课本", "同步练习", "真题", "题库", "课程",
	}
	periodicalTerms = []string{
		"期刊", "杂志", "月刊", "周刊", "年刊", "年鉴", "文摘", "连续出版",
	}
	fictionTerms = []string{
		"小说", "科幻", "奇幻", "武侠", "言情", "悬疑", "推理", "恐怖",
		"轻小说", "网文", "故事集", "寓言", "童话", "虚构",
	}
	nonfictionTerms = []string{
		"历史", "传记", "社科", "哲学", "经济", "管理", "心理", "科普",
		"政治", "法律", "艺术", "文化", "教育", "文学", "散文", "诗歌",
		"纪实", "评论",
	}
)
