package library

import (
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"my-weread-desk/internal/story"
)

const branchColumns = `id,book_id,fork_seq,rejoin_seq,title,status,summary,state_json,memory_md,created_at,updated_at`

// UpsertBranch stores a world line including its constitution and state.
func (s *Store) UpsertBranch(b story.Branch) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	b.State.Normalize()
	state, err := json.Marshal(b.State)
	if err != nil {
		return err
	}
	now := time.Now().Unix()
	if b.CreatedAt == 0 {
		b.CreatedAt = now
	}
	if b.Status == "" {
		b.Status = story.BranchActive
	}
	_, err = s.db.Exec(`INSERT INTO story_branch
		(id,book_id,fork_seq,rejoin_seq,title,status,summary,state_json,memory_md,created_at,updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(id) DO UPDATE SET
		 rejoin_seq=excluded.rejoin_seq, title=excluded.title, status=excluded.status,
		 summary=excluded.summary, state_json=excluded.state_json,
		 memory_md=excluded.memory_md, updated_at=excluded.updated_at`,
		b.ID, b.BookID, b.ForkSeq, b.RejoinSeq, b.Title, b.Status, b.Summary,
		string(state), b.MemoryMD, b.CreatedAt, now)
	return err
}

// Branch returns one world line.
func (s *Store) Branch(id string) (*story.Branch, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	row := s.db.QueryRow(`SELECT `+branchColumns+` FROM story_branch WHERE id=?`, id)
	return scanBranch(row)
}

// Branches returns a book's world lines in creation order.
func (s *Store) Branches(bookID string) ([]story.Branch, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows, err := s.db.Query(`SELECT `+branchColumns+` FROM story_branch
		WHERE book_id=? ORDER BY created_at ASC, id ASC`, bookID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []story.Branch
	for rows.Next() {
		b, err := scanBranch(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *b)
	}
	return out, rows.Err()
}

// DeleteBranch removes a world line and its scenes.
func (s *Store) DeleteBranch(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec(`DELETE FROM story_branch_scene WHERE branch_id=?`, id); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM story_branch WHERE id=?`, id); err != nil {
		return err
	}
	return tx.Commit()
}

func scanBranch(row interface{ Scan(...any) error }) (*story.Branch, error) {
	var (
		b     story.Branch
		state string
	)
	err := row.Scan(&b.ID, &b.BookID, &b.ForkSeq, &b.RejoinSeq, &b.Title, &b.Status,
		&b.Summary, &state, &b.MemoryMD, &b.CreatedAt, &b.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if state != "" {
		_ = json.Unmarshal([]byte(state), &b.State)
	}
	b.State.Normalize()
	return &b, nil
}

// ---------------------------------------------------------------- scenes

// AppendBranchScene stores one generated scene. Regenerating the same sequence
// replaces the row so a retry cannot fork the branch's own history.
func (s *Store) AppendBranchScene(sc story.BranchScene) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	verdicts, err := json.Marshal(sc.Verdicts)
	if err != nil {
		return err
	}
	choices, err := json.Marshal(sc.Choices)
	if err != nil {
		return err
	}
	now := sc.CreatedAt
	if now == 0 {
		now = time.Now().Unix()
	}
	_, err = s.db.Exec(`INSERT INTO story_branch_scene
		(branch_id,seq,canon_seq,text,summary,verdict_json,choices_json,model,created_at)
		VALUES (?,?,?,?,?,?,?,?,?)
		ON CONFLICT(branch_id,seq) DO UPDATE SET
		 canon_seq=excluded.canon_seq, text=excluded.text, summary=excluded.summary,
		 verdict_json=excluded.verdict_json, choices_json=excluded.choices_json,
		 model=excluded.model, created_at=excluded.created_at`,
		sc.BranchID, sc.Seq, sc.CanonSeq, sc.Text, sc.Summary,
		string(verdicts), string(choices), sc.Model, now)
	return err
}

// BranchScenes returns the generated scenes in order.
func (s *Store) BranchScenes(branchID string) ([]story.BranchScene, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows, err := s.db.Query(`SELECT branch_id,seq,canon_seq,text,summary,verdict_json,choices_json,model,created_at
		FROM story_branch_scene WHERE branch_id=? ORDER BY seq ASC`, branchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []story.BranchScene
	for rows.Next() {
		var (
			sc       story.BranchScene
			verdicts string
			choices  string
		)
		if err := rows.Scan(&sc.BranchID, &sc.Seq, &sc.CanonSeq, &sc.Text, &sc.Summary,
			&verdicts, &choices, &sc.Model, &sc.CreatedAt); err != nil {
			return nil, err
		}
		if verdicts != "" {
			_ = json.Unmarshal([]byte(verdicts), &sc.Verdicts)
		}
		if choices != "" {
			_ = json.Unmarshal([]byte(choices), &sc.Choices)
		}
		out = append(out, sc)
	}
	return out, rows.Err()
}

// LastBranchScene returns the most recent scene, or ErrNotFound.
func (s *Store) LastBranchScene(branchID string) (*story.BranchScene, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var (
		sc       story.BranchScene
		verdicts string
		choices  string
	)
	err := s.db.QueryRow(`SELECT branch_id,seq,canon_seq,text,summary,verdict_json,choices_json,model,created_at
		FROM story_branch_scene WHERE branch_id=? ORDER BY seq DESC LIMIT 1`, branchID).
		Scan(&sc.BranchID, &sc.Seq, &sc.CanonSeq, &sc.Text, &sc.Summary,
			&verdicts, &choices, &sc.Model, &sc.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if verdicts != "" {
		_ = json.Unmarshal([]byte(verdicts), &sc.Verdicts)
	}
	if choices != "" {
		_ = json.Unmarshal([]byte(choices), &sc.Choices)
	}
	return &sc, nil
}

// BranchSceneCount reports how many scenes a world line has.
func (s *Store) BranchSceneCount(branchID string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM story_branch_scene WHERE branch_id=?`, branchID).Scan(&n)
	return n, err
}

// BranchCount reports how many world lines a book has.
func (s *Store) BranchCount(bookID string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM story_branch WHERE book_id=?`, bookID).Scan(&n)
	return n, err
}
