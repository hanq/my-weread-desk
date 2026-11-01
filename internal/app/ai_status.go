package app

import (
	"fmt"
	"strings"

	"my-weread-desk/internal/library"
)

// AI readiness values. The UI uses them to explain why a capability is
// disabled, and the bindings use them to refuse work that would waste a model
// call on an unindexed book.
const (
	AIStageNone    = "none"
	AIStageRunning = "running"
	AIStagePartial = "partial"
	AIStageReady   = "ready"
	AIStageFailed  = "failed"
	AIStageStale   = "stale"
)

// AIStage is one preparation step of a book.
type AIStage struct {
	Status    string `json:"status"`
	Detail    string `json:"detail,omitempty"`
	Count     int    `json:"count,omitempty"`
	UpdatedAt int64  `json:"updatedAt,omitempty"`
}

// AICapabilities is the final answer to "what can I do with this book now".
type AICapabilities struct {
	Ask     bool `json:"ask"`
	Immerse bool `json:"immerse"`
	Rewrite bool `json:"rewrite"`
}

// AIStatus bundles the book kind with the three preparation stages.
//
// Text indexing is local and mandatory for every AI feature. Scene extraction
// is required for immersion and rewriting. A world sheet is required only for
// rewriting, because that is the mode that changes state.
type AIStatus struct {
	BookID       string         `json:"bookId"`
	Kind         string         `json:"kind"`
	KindSource   string         `json:"kindSource"`
	Text         AIStage        `json:"text"`
	Scenes       AIStage        `json:"scenes"`
	World        AIStage        `json:"world"`
	Capabilities AICapabilities `json:"capabilities"`
}

// AIStatus reports the preparation state without touching the network.
func (a *App) AIStatus(bookID string) (AIStatus, error) {
	bookID = strings.TrimSpace(bookID)
	if bookID == "" {
		return AIStatus{}, fmt.Errorf("缺少书籍 ID")
	}

	kind, err := a.BookKind(bookID)
	if err != nil {
		return AIStatus{}, err
	}
	source := "auto"
	if ai, err := a.store.BookAI(bookID); err == nil {
		source = ai.KindSource
	}

	text := a.textStage(bookID)
	scenes := a.sceneStage(bookID)
	world := a.worldStage(bookID)

	fiction := kind == string(library.BookKindFiction)
	textReady := text.Status == AIStageReady
	scenesReady := scenes.Status == AIStageReady
	worldReady := world.Status == AIStageReady

	return AIStatus{
		BookID:     bookID,
		Kind:       kind,
		KindSource: source,
		Text:       text,
		Scenes:     scenes,
		World:      world,
		Capabilities: AICapabilities{
			Ask:     textReady,
			Immerse: fiction && textReady && scenesReady,
			Rewrite: fiction && textReady && scenesReady && worldReady,
		},
	}, nil
}

func (a *App) textStage(bookID string) AIStage {
	meta, err := a.store.RAGIndex(bookID)
	if err != nil || meta == nil {
		return AIStage{Status: AIStageNone}
	}
	stage := AIStage{Count: meta.ChunkCount, UpdatedAt: meta.UpdatedAt}
	switch {
	case meta.ChunkerVersion != library.RAGChunkerVersion:
		stage.Status = AIStageStale
		stage.Detail = "分块版本已更新，请重建索引"
	case meta.Status == "ready" && meta.ChunkCount > 0:
		stage.Status = AIStageReady
		stage.Detail = fmt.Sprintf("%d 块 · %d 字", meta.ChunkCount, meta.CharCount)
	case meta.Status == "indexing":
		stage.Status = AIStageRunning
		stage.Detail = "正在建立文本索引"
	case meta.Status == "failed":
		stage.Status = AIStageFailed
		stage.Detail = meta.Error
	default:
		stage.Status = AIStageNone
	}
	return stage
}

func (a *App) sceneStage(bookID string) AIStage {
	count, _ := a.store.SceneCount(bookID)
	job, err := a.store.StoryJob(bookID, library.StoryJobSceneExtract)
	stage := AIStage{Status: AIStageNone, Count: count}
	if err != nil || job == nil {
		if count > 0 {
			stage.Status = AIStageReady
			stage.Detail = fmt.Sprintf("%d 幕", count)
		}
		return stage
	}

	stage.UpdatedAt = job.UpdatedAt
	switch {
	case job.Status == "running":
		stage.Status = AIStageRunning
		stage.Detail = fmt.Sprintf("正在分析剧情 · %d/%d 章", job.Done, job.Total)
	case job.Status == "failed" && count == 0:
		stage.Status = AIStageFailed
		stage.Detail = job.Error
	case count > 0 && (job.Status == "done" || job.Status == ""):
		stage.Status = AIStageReady
		stage.Detail = fmt.Sprintf("%d 幕", count)
	case count > 0:
		stage.Status = AIStagePartial
		stage.Detail = fmt.Sprintf("%d 幕 · %d/%d 章", count, job.Done, job.Total)
	default:
		stage.Status = AIStageNone
	}
	return stage
}

func (a *App) worldStage(bookID string) AIStage {
	anchors, _ := a.store.Anchors(bookID)
	stats, items, _ := a.store.WorldSheetCounts(bookID)
	stage := AIStage{Status: AIStageNone, Count: len(anchors) + stats + items}
	switch {
	case stats > 0 || items > 0:
		stage.Status = AIStageReady
		stage.Detail = fmt.Sprintf("%d 项属性 · %d 件道具 · %d 个锚点", stats, items, len(anchors))
	case len(anchors) > 0:
		stage.Status = AIStagePartial
		stage.Detail = fmt.Sprintf("%d 个锚点 · 还没有世界表", len(anchors))
	}
	return stage
}

// requireTextIndex is the hard gate behind every AI entry point.
func (a *App) requireTextIndex(bookID string) error {
	status, err := a.AIStatus(bookID)
	if err != nil {
		return err
	}
	if status.Text.Status == AIStageReady {
		return nil
	}
	return fmt.Errorf("《%s》还没有建立文本索引，请先在 AI 书房完成「准备本书」", a.bookTitle(bookID))
}

func (a *App) requireScenes(bookID string) error {
	status, err := a.AIStatus(bookID)
	if err != nil {
		return err
	}
	if status.Scenes.Status == AIStageReady {
		return nil
	}
	detail := ""
	if status.Scenes.Detail != "" {
		detail = "（" + status.Scenes.Detail + "）"
	}
	return fmt.Errorf("《%s》还没有建立剧情索引%s，请先在 AI 书房完成「剧情索引」", a.bookTitle(bookID), detail)
}

func (a *App) requireWorld(bookID string) error {
	status, err := a.AIStatus(bookID)
	if err != nil {
		return err
	}
	if status.World.Status == AIStageReady {
		return nil
	}
	return fmt.Errorf("《%s》还没有建立世界设定，请先在穿越页完成「世界表」", a.bookTitle(bookID))
}

func (a *App) requireFiction(bookID string) error {
	kind, err := a.BookKind(bookID)
	if err != nil {
		return err
	}
	if kind != string(library.BookKindFiction) {
		return fmt.Errorf("这本书被识别为「%s」，故事穿越仅支持虚构叙事；可在书籍详情中更正类型", library.BookKindLabel(kind))
	}
	return nil
}

func (a *App) bookTitle(bookID string) string {
	if book, err := a.store.Book(bookID); err == nil && book != nil && strings.TrimSpace(book.Title) != "" {
		return book.Title
	}
	return "这本书"
}
