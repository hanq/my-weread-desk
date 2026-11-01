package app

import (
	"errors"
	"strings"

	"my-weread-desk/internal/library"
	"my-weread-desk/internal/weread"
)

// enrichBookKinds fills the AI classification for a batch of shelf entries.
// Missing rows are classified once and persisted, so repeated shelf loads do
// not repeat the heuristic and a user correction is never overwritten.
func (a *App) enrichBookKinds(books []weread.Book) []weread.Book {
	if len(books) == 0 {
		return books
	}

	ids := make([]string, 0, len(books))
	for _, b := range books {
		if b.ID != "" {
			ids = append(ids, b.ID)
		}
	}
	known, err := a.store.BookAIMap(ids)
	if err != nil {
		a.logf("读取书籍 AI 类型失败：%v", err)
		known = map[string]library.BookAI{}
	}

	for i := range books {
		if ai, ok := known[books[i].ID]; ok && ai.Kind != "" {
			books[i].Kind = ai.Kind
			books[i].KindSource = ai.KindSource
			continue
		}
		kind := string(library.ClassifyBook(books[i]))
		books[i].Kind = kind
		books[i].KindSource = "auto"
		if err := a.store.UpsertBookAI(books[i].ID, kind, "auto"); err != nil {
			a.logf("保存书籍 AI 类型失败：book=%s err=%v", books[i].ID, err)
		}
	}
	return books
}

// enrichBookKind is the single-book variant used by the detail page.
func (a *App) enrichBookKind(b weread.Book) weread.Book {
	if b.ID == "" {
		return b
	}
	if ai, err := a.store.BookAI(b.ID); err == nil && ai.Kind != "" {
		b.Kind = ai.Kind
		b.KindSource = ai.KindSource
		return b
	}
	kind := string(library.ClassifyBook(b))
	b.Kind = kind
	b.KindSource = "auto"
	if err := a.store.UpsertBookAI(b.ID, kind, "auto"); err != nil {
		a.logf("保存书籍 AI 类型失败：book=%s err=%v", b.ID, err)
	}
	return b
}

// BookKind returns the stored classification, classifying and persisting it on
// first use. A missing book is an error rather than a silent "other".
func (a *App) BookKind(bookID string) (string, error) {
	book, err := a.store.Book(bookID)
	if err != nil {
		return "", err
	}
	return a.enrichBookKind(*book).Kind, nil
}

// SetBookKind records a user correction. It is the escape hatch for the
// heuristic: category strings are free-form, so "文学" can mean a novel or an
// essay collection.
func (a *App) SetBookKind(bookID, kind string) (string, error) {
	bookID = strings.TrimSpace(bookID)
	kind = strings.TrimSpace(kind)
	if bookID == "" {
		return "", errors.New("缺少书籍 ID")
	}
	if !library.BookKind(kind).Valid() {
		return "", errors.New("不支持的书籍类型")
	}
	if _, err := a.store.Book(bookID); err != nil {
		return "", errors.New("这本书不在本地缓存中")
	}
	if err := a.store.UpsertBookAI(bookID, kind, "user"); err != nil {
		return "", err
	}
	return kind, nil
}
