package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"my-weread-desk/internal/ai"
	"my-weread-desk/internal/story"
)

// BranchDetail bundles a world line with its generated scenes.
type BranchDetail struct {
	Branch story.Branch        `json:"branch"`
	Scenes []story.BranchScene `json:"scenes"`
}

// StartBranch forks a world line at a scene.
//
// No model call happens here: the constitution is built locally, so a reader can
// set up a world line before configuring a provider.
func (a *App) StartBranch(bookID string, forkSeq int, intent string) (*story.Branch, error) {
	bookID = strings.TrimSpace(bookID)
	if bookID == "" {
		return nil, errors.New("缺少书籍 ID")
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
	if err := a.requireWorld(bookID); err != nil {
		return nil, err
	}

	scenes, err := a.store.StoryScenes(bookID)
	if err != nil {
		return nil, err
	}
	if len(scenes) == 0 {
		return nil, errors.New("请先建立时间线")
	}

	var fork story.Scene
	found := false
	maxSeq := 0
	for _, sc := range scenes {
		if sc.Seq > maxSeq {
			maxSeq = sc.Seq
		}
		if sc.Seq == forkSeq {
			fork = sc
			found = true
		}
	}
	if !found {
		return nil, errors.New("分叉点不在时间线内")
	}

	title := bookID
	author := ""
	if book, berr := a.store.Book(bookID); berr == nil && book != nil {
		if book.Title != "" {
			title = book.Title
		}
		author = book.Author
	}

	id := fmt.Sprintf("branch-%d", time.Now().UnixNano())
	mem := story.NewMemory(story.BookRef{Title: title, Author: author, EditionHash: story.BookKey(title, author)})
	mem.BranchID = id

	intent = strings.TrimSpace(intent)
	if intent != "" {
		if err := mem.AddUserRule(intent); err != nil {
			return nil, err
		}
	}
	memMD, err := mem.Render()
	if err != nil {
		return nil, err
	}

	state := story.NewState()
	if intent != "" {
		state.Assumptions = []string{intent}
	}
	// Seed the book's initial numbers so the very first scene already carries
	// resources instead of starting from an empty sheet.
	if sheet, serr := a.store.WorldSheet(bookID); serr == nil {
		sheet.InitState(state)
	}

	b := story.Branch{
		ID:        id,
		BookID:    bookID,
		ForkSeq:   fork.Seq,
		RejoinSeq: story.EstimateRejoin(fork.Seq, maxSeq, 4),
		Title:     fmt.Sprintf("世界线 · 第 %d 幕", fork.Seq+1),
		Status:    story.BranchActive,
		State:     *state,
		MemoryMD:  memMD,
	}
	if err := a.store.UpsertBranch(b); err != nil {
		return nil, err
	}
	return &b, nil
}

// Branches lists a book's world lines.
func (a *App) Branches(bookID string) ([]story.Branch, error) {
	branches, err := a.store.Branches(bookID)
	if err != nil {
		return nil, err
	}
	if branches == nil {
		return []story.Branch{}, nil
	}
	return branches, nil
}

// BranchDetail returns a world line with its scenes.
func (a *App) BranchDetail(branchID string) (*BranchDetail, error) {
	b, err := a.store.Branch(branchID)
	if err != nil {
		return nil, err
	}
	scenes, err := a.store.BranchScenes(branchID)
	if err != nil {
		return nil, err
	}
	if scenes == nil {
		scenes = []story.BranchScene{}
	}
	return &BranchDetail{Branch: *b, Scenes: scenes}, nil
}

// ContinueBranch generates the next scene of a world line.
func (a *App) ContinueBranch(branchID, intent string) (*story.BranchScene, error) {
	if !a.aiReady() {
		return nil, errors.New("需要先启用 AI 并确认正文片段发送")
	}
	b, err := a.store.Branch(branchID)
	if err != nil {
		return nil, err
	}
	if b.Status != story.BranchActive {
		return nil, errors.New("这条世界线已经收束")
	}
	if err := a.requireFiction(b.BookID); err != nil {
		return nil, err
	}
	if err := a.requireTextIndex(b.BookID); err != nil {
		return nil, err
	}
	if err := a.requireScenes(b.BookID); err != nil {
		return nil, err
	}
	if err := a.requireWorld(b.BookID); err != nil {
		return nil, err
	}

	mem, err := story.ParseMemory(b.MemoryMD)
	if err != nil {
		return nil, fmt.Errorf("世界线宪法无法读取：%w", err)
	}
	scenes, err := a.store.StoryScenes(b.BookID)
	if err != nil {
		return nil, err
	}
	if len(scenes) == 0 {
		return nil, errors.New("时间线已不存在，请重建")
	}

	existing, err := a.store.BranchScenes(branchID)
	if err != nil {
		return nil, err
	}

	maxSeq := scenes[len(scenes)-1].Seq
	pos := b.ForkSeq + len(existing)
	if pos > maxSeq {
		pos = maxSeq
	}
	canon, ok := findScene(scenes, pos)
	if !ok {
		canon = scenes[len(scenes)-1]
	}

	anchors, _ := a.store.Anchors(b.BookID)
	sat := story.Satisfaction{}
	for _, id := range b.State.SatisfiedAnchors {
		sat[id] = true
	}
	divergence := story.Divergence(pos, anchors, sat, 0, 0)
	pressure := story.PlanPressure(pos, anchors, sat, divergence)

	previous := make([]string, 0, 2)
	for _, sc := range existing {
		previous = append(previous, sc.Text)
	}
	if len(previous) > 2 {
		previous = previous[len(previous)-2:]
	}

	// Compile the free-form request before generating. A request that breaks a
	// hard rule is turned down with alternatives instead of being attempted and
	// rejected by validation after paying for a full scene.
	intentGoal := ""
	var patch story.StatePatch
	if raw := strings.TrimSpace(intent); raw != "" {
		compiled, cerr := a.compileIntent(mem, &b.State, raw)
		if cerr != nil {
			return nil, cerr
		}
		if compiled.Feasibility == story.IntentViolates {
			return nil, fmt.Errorf("%s。可选：%s", compiled.Reason, strings.Join(compiled.Alternatives, " / "))
		}
		intentGoal = compiled.Goal
		if compiled.Feasibility == story.IntentNeedsSetup {
			intentGoal = "先做铺垫：" + intentGoal
		}
		patch = compiled.StatePatch
	}

	sheet, serr := a.store.WorldSheet(b.BookID)
	if serr != nil {
		sheet = &story.WorldSheet{}
	}

	text, verdicts, err := a.generateScene(mem, &b.State, sheet, canon, pressure, intentGoal, previous)
	if err != nil {
		return nil, err
	}

	if vs := story.ValidatePatch(patch, &b.State); story.HasHardViolation(vs) {
		// A malformed or oversized patch is dropped rather than partially
		// applied: half a change is harder to reason about than none.
		a.logf("状态补丁被拒绝：%+v", vs)
		patch = story.StatePatch{}
	}
	// World deltas are validated and applied by the engine, then stripped so
	// State.Apply cannot double-apply them.
	if vs := story.ApplyWorld(sheet, &b.State, patch); story.HasHardViolation(vs) {
		a.logf("世界变更被拒绝：%+v", vs)
	}
	sheet.Tick(&b.State)
	patch.StatDelta = nil
	patch.ItemDelta = nil
	b.State.Apply(patch)
	if satisfied := a.judgeAnchors(&b.State, anchors, text, pos); len(satisfied) > 0 {
		b.State.Apply(story.StatePatch{SatisfyAnchors: satisfied})
	}

	scene := story.BranchScene{
		BranchID:  branchID,
		Seq:       len(existing),
		CanonSeq:  canon.Seq,
		Text:      text,
		Summary:   shortenRunes(text, 80),
		Verdicts:  verdicts,
		Choices:   a.suggestChoices(mem.Book.Title, text),
		Model:     a.settings.Get().AI.Model,
		CreatedAt: time.Now().Unix(),
	}
	if err := a.store.AppendBranchScene(scene); err != nil {
		return nil, err
	}

	mem.Engine.Divergence = divergence
	mem.Engine.SatisfiedAnchors = b.State.SatisfiedAnchors
	mem.Engine.UpdatedAt = time.Now().Unix()
	mem.Engine.Summary = scene.Summary
	if md, rerr := mem.Render(); rerr == nil {
		b.MemoryMD = md
	}
	b.State.Facts["最近一幕"] = scene.Summary
	b.UpdatedAt = time.Now().Unix()
	if pos >= b.RejoinSeq {
		b.Status = story.BranchJoined
	}
	if err := a.store.UpsertBranch(*b); err != nil {
		return nil, err
	}
	return &scene, nil
}

// generateScene runs the model and retries once with the violations fed back.
// Two attempts is the cost ceiling: a scene that keeps breaking hard rules is
// better refused than silently shipped.
func (a *App) generateScene(mem *story.Memory, state *story.State, sheet *story.WorldSheet, canon story.Scene,
	pressure *story.Pressure, intent string, previous []string) (string, []story.Violation, error) {

	cfg := a.settings.Get().AI
	provider := ai.NewOpenAI(ai.Config{
		BaseURL: cfg.BaseURL, APIKey: cfg.APIKey, Model: cfg.Model,
		Temperature: cfg.Temperature, MaxTokens: cfg.MaxTokens,
	})
	ctx, cancel := context.WithTimeout(a.ctxOrBackground(), 120*time.Second)
	defer cancel()

	canonText := a.sceneText(canon)
	title := mem.Book.Title

	var verdicts []story.Violation
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		system, user := story.BuildSceneGenPrompt(story.SceneGenInput{
			BookTitle:  title,
			CanonTitle: canon.ChapterTitle,
			CanonText:  canonText,
			Memory:     mem,
			State:      state,
			World:      sheet,
			Pressure:   pressure,
			Intent:     intent,
			Previous:   previous,
			Violations: verdicts,
		})
		text, err := provider.Chat(ctx, ai.Request{
			Model: cfg.Model,
			Messages: []ai.Message{
				{Role: "system", Content: system},
				{Role: "user", Content: user},
			},
			Temperature: cfg.Temperature,
			MaxTokens:   cfg.MaxTokens,
		}, nil)
		if err != nil {
			return "", nil, err
		}
		verdicts = story.ValidateScene(text, mem, state)
		if !story.HasHardViolation(verdicts) {
			return strings.TrimSpace(text), verdicts, nil
		}
		lastErr = fmt.Errorf("校验未通过：%s（%s）", verdicts[0].Rule, verdicts[0].Evidence)
	}
	return "", verdicts, lastErr
}

// sceneText returns the original text of a scene, clamped to its block span.
func (a *App) sceneText(sc story.Scene) string {
	content, err := a.store.ChapterContent(sc.BookID, sc.ChapterUID)
	if err != nil || len(content.Blocks) == 0 {
		return ""
	}
	blocks := content.Blocks
	start, end := sc.BlockStart, sc.BlockEnd
	if start < 0 {
		start = 0
	}
	if end >= len(blocks) {
		end = len(blocks) - 1
	}
	if start > end {
		return ""
	}
	return story.PlainText(blocks[start : end+1])
}

// RejoinBranch closes a world line: it summarises what diverged and marks the
// branch joined. The summary is what lets the reader carry the consequences
// back without regenerating every later chapter.
func (a *App) RejoinBranch(branchID string) (*story.Branch, error) {
	b, err := a.store.Branch(branchID)
	if err != nil {
		return nil, err
	}
	scenes, err := a.store.BranchScenes(branchID)
	if err != nil {
		return nil, err
	}

	summary := ""
	if len(scenes) > 0 {
		summary = a.summariseBranch(b, scenes)
	}
	b.Summary = summary
	b.Status = story.BranchJoined
	b.UpdatedAt = time.Now().Unix()
	if err := a.store.UpsertBranch(*b); err != nil {
		return nil, err
	}
	return b, nil
}

func (a *App) summariseBranch(b *story.Branch, scenes []story.BranchScene) string {
	if a.aiReady() {
		cfg := a.settings.Get().AI
		provider := ai.NewOpenAI(ai.Config{
			BaseURL: cfg.BaseURL, APIKey: cfg.APIKey, Model: cfg.Model,
			Temperature: cfg.Temperature, MaxTokens: 400,
		})
		system, user := story.BuildBranchSummaryPrompt(b.Title, scenes)
		ctx, cancel := context.WithTimeout(a.ctxOrBackground(), 60*time.Second)
		defer cancel()
		if answer, err := provider.Chat(ctx, ai.Request{
			Model: cfg.Model,
			Messages: []ai.Message{
				{Role: "system", Content: system},
				{Role: "user", Content: user},
			},
			Temperature: 0.2,
			MaxTokens:   400,
		}, nil); err == nil {
			return strings.TrimSpace(answer)
		}
	}

	// Offline fallback: the scene summaries already describe the divergence.
	parts := make([]string, 0, len(scenes))
	for _, sc := range scenes {
		if sc.Summary != "" {
			parts = append(parts, sc.Summary)
		}
	}
	return shortenRunes(strings.Join(parts, "；"), 150)
}

// DeleteBranch removes a world line and its scenes.
func (a *App) DeleteBranch(branchID string) error {
	return a.store.DeleteBranch(branchID)
}

// ExportBranch serialises a world line for sharing.
//
// The package contains no original book text: only generated scenes and canon
// sequence references, so sharing it does not distribute the novel.
func (a *App) ExportBranch(branchID string) (string, error) {
	b, err := a.store.Branch(branchID)
	if err != nil {
		return "", err
	}
	scenes, err := a.store.BranchScenes(branchID)
	if err != nil {
		return "", err
	}
	pkg := story.BuildBranchPackage(*b, scenes, story.PackageAuthor{})
	out, err := json.MarshalIndent(pkg, "", "  ")
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// ImportBranch validates and stores a shared world line.
//
// Package text is treated as data, never as instructions: it is stored as scene
// content and later injected into the user turn of the generation prompt, while
// the constitution stays the only system-level ruleset.
func (a *App) ImportBranch(bookID, payload string) (*story.Branch, error) {
	bookID = strings.TrimSpace(bookID)
	if bookID == "" {
		return nil, errors.New("缺少书籍 ID")
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
	pkg, err := story.ParseBranchPackage(payload)
	if err != nil {
		return nil, err
	}

	title, author := bookID, ""
	if book, berr := a.store.Book(bookID); berr == nil && book != nil {
		if book.Title != "" {
			title = book.Title
		}
		author = book.Author
	}
	if !pkg.MatchesBook(title, author) {
		return nil, fmt.Errorf("这份世界线属于《%s》，与当前书籍不匹配", pkg.Book.Title)
	}

	id := fmt.Sprintf("branch-%d", time.Now().UnixNano())
	b := story.Branch{
		ID:        id,
		BookID:    bookID,
		ForkSeq:   pkg.ForkSeq,
		RejoinSeq: pkg.RejoinSeq,
		Title:     strings.TrimSpace("导入 · " + pkg.Title),
		Status:    pkg.Status,
		Summary:   pkg.Summary,
		State:     pkg.State,
		MemoryMD:  pkg.MemoryMD,
	}
	if strings.TrimSpace(pkg.Title) == "" {
		b.Title = "导入的世界线"
	}
	if b.Status == "" {
		b.Status = story.BranchActive
	}

	// Keep the constitution's branch id in sync with the local row.
	if mem, merr := story.ParseMemory(pkg.MemoryMD); merr == nil {
		mem.BranchID = id
		if md, rerr := mem.Render(); rerr == nil {
			b.MemoryMD = md
		}
	}
	if err := a.store.UpsertBranch(b); err != nil {
		return nil, err
	}
	for i, sc := range pkg.Scenes {
		sc.BranchID = id
		sc.Seq = i
		if sc.CreatedAt == 0 {
			sc.CreatedAt = time.Now().Unix()
		}
		if err := a.store.AppendBranchScene(sc); err != nil {
			return nil, err
		}
	}
	return &b, nil
}

// ---------------------------------------------------------------- helpers

func findScene(scenes []story.Scene, seq int) (story.Scene, bool) {
	for _, sc := range scenes {
		if sc.Seq == seq {
			return sc, true
		}
	}
	return story.Scene{}, false
}

func shortenRunes(s string, n int) string {
	clean := strings.Join(strings.Fields(s), " ")
	runes := []rune(clean)
	if len(runes) <= n {
		return clean
	}
	return string(runes[:n]) + "…"
}

// CompileIntent classifies a free-form request before generation, so a conflict
// can be shown as a choice instead of surfacing as a failed scene.
func (a *App) CompileIntent(branchID, intent string) (*story.Intent, error) {
	b, err := a.store.Branch(branchID)
	if err != nil {
		return nil, err
	}
	mem, err := story.ParseMemory(b.MemoryMD)
	if err != nil {
		return nil, fmt.Errorf("世界线宪法无法读取：%w", err)
	}
	return a.compileIntent(mem, &b.State, intent)
}

func (a *App) compileIntent(mem *story.Memory, st *story.State, raw string) (*story.Intent, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return &story.Intent{Feasibility: story.IntentOK}, nil
	}
	if vs := story.PrecheckIntent(raw, mem, st); len(vs) > 0 {
		rules := make([]string, 0, len(vs))
		for _, v := range vs {
			rules = append(rules, v.Rule)
		}
		return &story.Intent{
			Raw:          raw,
			Feasibility:  story.IntentViolates,
			Reason:       vs[0].Rule + "：" + vs[0].Evidence,
			BlockedBy:    rules,
			Alternatives: story.AlternativesFor(vs),
		}, nil
	}
	if !a.aiReady() {
		return nil, errors.New("需要先启用 AI 并确认正文片段发送")
	}

	cfg := a.settings.Get().AI
	provider := ai.NewOpenAI(ai.Config{
		BaseURL: cfg.BaseURL, APIKey: cfg.APIKey, Model: cfg.Model,
		Temperature: cfg.Temperature, MaxTokens: 400,
	})
	system, user := story.BuildIntentPrompt(mem, st, raw)
	ctx, cancel := context.WithTimeout(a.ctxOrBackground(), 45*time.Second)
	defer cancel()
	answer, err := provider.Chat(ctx, ai.Request{
		Model: cfg.Model,
		Messages: []ai.Message{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
		Temperature: 0.1,
		MaxTokens:   400,
	}, nil)
	if err != nil {
		return nil, err
	}
	out, err := story.ParseIntent(answer)
	if err != nil {
		return nil, err
	}
	out.Raw = raw
	if out.Goal == "" {
		out.Goal = raw
	}
	return &out, nil
}

// judgeAnchors asks whether the scene realised any near-due anchor. Only anchors
// within two scenes of their deadline are checked, which bounds the cost to at
// most three short calls per scene.
func (a *App) judgeAnchors(st *story.State, anchors []story.CanonAnchor, text string, pos int) []string {
	if len(anchors) == 0 || !a.aiReady() {
		return nil
	}
	sat := story.Satisfaction{}
	for _, id := range st.SatisfiedAnchors {
		sat[id] = true
	}
	cfg := a.settings.Get().AI
	provider := ai.NewOpenAI(ai.Config{
		BaseURL: cfg.BaseURL, APIKey: cfg.APIKey, Model: cfg.Model,
		Temperature: cfg.Temperature, MaxTokens: 16,
	})
	ctx, cancel := context.WithTimeout(a.ctxOrBackground(), 60*time.Second)
	defer cancel()

	var out []string
	checked := 0
	for _, anc := range anchors {
		if sat[anc.ID] || anc.DeadlineSeq > pos+2 {
			continue
		}
		if checked >= 3 {
			break
		}
		checked++
		system, user := story.BuildAnchorCheckPrompt(anc, text)
		answer, err := provider.Chat(ctx, ai.Request{
			Model: cfg.Model,
			Messages: []ai.Message{
				{Role: "system", Content: system},
				{Role: "user", Content: user},
			},
			Temperature: 0,
			MaxTokens:   16,
		}, nil)
		if err != nil {
			continue
		}
		if ok, _ := story.ParseAnchorCheck(answer); ok {
			out = append(out, anc.ID)
		}
	}
	return out
}

// suggestChoices asks the model for concrete options at the end of a scene.
// Failure is non-fatal: the reader can always type an intent directly.
func (a *App) suggestChoices(bookTitle, sceneText string) []story.Choice {
	if !a.aiReady() {
		return nil
	}
	cfg := a.settings.Get().AI
	provider := ai.NewOpenAI(ai.Config{
		BaseURL: cfg.BaseURL, APIKey: cfg.APIKey, Model: cfg.Model,
		Temperature: cfg.Temperature, MaxTokens: 200,
	})
	system, user := story.BuildChoicesPrompt(bookTitle, sceneText)
	ctx, cancel := context.WithTimeout(a.ctxOrBackground(), 45*time.Second)
	defer cancel()
	answer, err := provider.Chat(ctx, ai.Request{
		Model: cfg.Model,
		Messages: []ai.Message{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
		Temperature: 0.3,
		MaxTokens:   200,
	}, nil)
	if err != nil {
		return nil
	}
	choices, err := story.ParseChoices(answer)
	if err != nil {
		return nil
	}
	return choices
}
