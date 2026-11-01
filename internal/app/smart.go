package app

import (
	"sort"

	"my-weread-desk/internal/weread"
)

// SmartBook is one entry on the smart shelf: a shelf book that already has AI
// artifacts, plus the capabilities those artifacts unlock.
type SmartBook struct {
	Book         weread.Book    `json:"book"`
	Kind         string         `json:"kind"`
	KindSource   string         `json:"kindSource"`
	Text         AIStage        `json:"text"`
	Scenes       AIStage        `json:"scenes"`
	World        AIStage        `json:"world"`
	Capabilities AICapabilities `json:"capabilities"`
	BranchCount  int            `json:"branchCount"`
	UpdatedAt    int64          `json:"updatedAt"`
}

// SmartBooks lists the smart shelf. It is intentionally data-driven: a book
// appears only after it has an index, a timeline, a world sheet or a world
// line, so the shelf is a record of work that actually exists.
func (a *App) SmartBooks() ([]SmartBook, error) {
	cached, err := a.store.Books()
	if err != nil {
		return nil, err
	}
	books := a.enrichBookKinds(cached)
	out := make([]SmartBook, 0, len(books))
	for _, b := range books {
		if b.ID == "" || b.IsAudio || b.IsMP {
			continue
		}
		status, err := a.AIStatus(b.ID)
		if err != nil {
			continue
		}
		branches, _ := a.store.Branches(b.ID)
		if status.Text.Status == AIStageNone && status.Scenes.Status == AIStageNone &&
			status.World.Status == AIStageNone && len(branches) == 0 {
			continue
		}

		updated := status.Text.UpdatedAt
		if status.Scenes.UpdatedAt > updated {
			updated = status.Scenes.UpdatedAt
		}
		if status.World.UpdatedAt > updated {
			updated = status.World.UpdatedAt
		}
		for _, branch := range branches {
			if branch.UpdatedAt > updated {
				updated = branch.UpdatedAt
			}
		}

		b.Kind = status.Kind
		b.KindSource = status.KindSource
		out = append(out, SmartBook{
			Book:         b,
			Kind:         status.Kind,
			KindSource:   status.KindSource,
			Text:         status.Text,
			Scenes:       status.Scenes,
			World:        status.World,
			Capabilities: status.Capabilities,
			BranchCount:  len(branches),
			UpdatedAt:    updated,
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].UpdatedAt != out[j].UpdatedAt {
			return out[i].UpdatedAt > out[j].UpdatedAt
		}
		return out[i].Book.Title < out[j].Book.Title
	})
	return out, nil
}
