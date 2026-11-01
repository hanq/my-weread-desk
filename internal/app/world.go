package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"my-weread-desk/internal/ai"
	"my-weread-desk/internal/story"
)

// WorldSheet returns the book's derived world sheet.
func (a *App) WorldSheet(bookID string) (*story.WorldSheet, error) {
	bookID = strings.TrimSpace(bookID)
	if bookID == "" {
		return nil, errors.New("缺少书籍 ID")
	}
	return a.store.WorldSheet(bookID)
}

// SaveWorldSheet replaces the sheet after the reader confirms it.
//
// The caps are enforced here too: a sheet that exceeds them would make the
// prompt unreliable, so it is rejected rather than silently truncated.
func (a *App) SaveWorldSheet(bookID string, sheet story.WorldSheet) (*story.WorldSheet, error) {
	bookID = strings.TrimSpace(bookID)
	if bookID == "" {
		return nil, errors.New("缺少书籍 ID")
	}
	if len(sheet.Stats) > story.MaxWorldStats {
		return nil, fmt.Errorf("属性最多 %d 项", story.MaxWorldStats)
	}
	if len(sheet.Items) > story.MaxWorldItems {
		return nil, fmt.Errorf("道具最多 %d 件", story.MaxWorldItems)
	}
	sheet.BookID = bookID
	if err := a.store.ReplaceWorldSheet(bookID, sheet); err != nil {
		return nil, err
	}
	return a.store.WorldSheet(bookID)
}

// SaveWorldStat stores one stat and returns the updated sheet.
func (a *App) SaveWorldStat(bookID string, def story.WorldStat) (*story.WorldSheet, error) {
	bookID = strings.TrimSpace(bookID)
	if bookID == "" {
		return nil, errors.New("缺少书籍 ID")
	}
	if err := a.store.UpsertWorldStat(bookID, def); err != nil {
		return nil, err
	}
	return a.store.WorldSheet(bookID)
}

// DeleteWorldStat removes one stat and returns the updated sheet.
func (a *App) DeleteWorldStat(bookID, key string) (*story.WorldSheet, error) {
	if err := a.store.DeleteWorldStat(bookID, key); err != nil {
		return nil, err
	}
	return a.store.WorldSheet(bookID)
}

// SaveWorldItem stores one item and returns the updated sheet.
func (a *App) SaveWorldItem(bookID string, item story.WorldItem) (*story.WorldSheet, error) {
	bookID = strings.TrimSpace(bookID)
	if bookID == "" {
		return nil, errors.New("缺少书籍 ID")
	}
	if err := a.store.UpsertWorldItem(bookID, item); err != nil {
		return nil, err
	}
	return a.store.WorldSheet(bookID)
}

// DeleteWorldItem removes one item and returns the updated sheet.
func (a *App) DeleteWorldItem(bookID, id string) (*story.WorldSheet, error) {
	if err := a.store.DeleteWorldItem(bookID, id); err != nil {
		return nil, err
	}
	return a.store.WorldSheet(bookID)
}

// DeriveWorldSheet asks the model for candidates.
//
// Nothing is saved: the sheet decides what the reader can and cannot do, so
// adopting it has to be an explicit act, exactly like adopting an anchor.
func (a *App) DeriveWorldSheet(bookID string) (*story.WorldSheet, error) {
	bookID = strings.TrimSpace(bookID)
	if bookID == "" {
		return nil, errors.New("缺少书籍 ID")
	}
	if !a.aiReady() {
		return nil, errors.New("需要先启用 AI 并确认正文片段发送")
	}
	if err := a.requireFiction(bookID); err != nil {
		return nil, err
	}
	if err := a.requireTextIndex(bookID); err != nil {
		return nil, err
	}
	if err := a.requireScenes(bookID); err != nil {
		return nil, err
	}
	scenes, err := a.store.StoryScenes(bookID)
	if err != nil {
		return nil, err
	}
	if len(scenes) == 0 {
		return nil, errors.New("请先建立时间线")
	}

	title := bookID
	if book, berr := a.store.Book(bookID); berr == nil && book != nil && book.Title != "" {
		title = book.Title
	}
	cfg := a.settings.Get().AI
	system, user := story.BuildWorldPrompt(title, scenes)
	provider := ai.NewOpenAI(ai.Config{
		BaseURL: cfg.BaseURL, APIKey: cfg.APIKey, Model: cfg.Model,
		Temperature: cfg.Temperature, MaxTokens: 900,
	})
	ctx, cancel := context.WithTimeout(a.ctxOrBackground(), 90*time.Second)
	defer cancel()
	answer, err := provider.Chat(ctx, ai.Request{
		Model: cfg.Model,
		Messages: []ai.Message{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
		Temperature: 0.2,
		MaxTokens:   900,
	}, nil)
	if err != nil {
		return nil, err
	}
	fields, err := story.ParseWorldFields(answer)
	if err != nil {
		return nil, err
	}

	sheet := &story.WorldSheet{BookID: bookID, Stats: []story.WorldStat{}, Items: []story.WorldItem{}}
	for _, f := range fields.Stats {
		sheet.Stats = append(sheet.Stats, story.WorldStat{
			Key: f.Key, Label: f.Label, Kind: f.Kind,
			Min: f.Min, Max: f.Max, Initial: f.Initial, Decay: f.Decay,
			EvidenceSeq: f.EvidenceSeq, Source: "derived",
		})
	}
	for i, f := range fields.Items {
		sheet.Items = append(sheet.Items, story.WorldItem{
			ID: fmt.Sprintf("item-%d", i+1), Name: f.Name, Kind: f.Kind,
			Owner: f.Owner, FirstSeq: f.FirstSeq, Effects: f.Effects,
			LimitPerBranch: 1, Source: "derived",
		})
	}
	return sheet, nil
}
