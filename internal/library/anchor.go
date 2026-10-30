package library

import (
	"database/sql"
	"errors"
	"time"

	"my-weread-desk/internal/story"
)

// ReplaceAnchors swaps a book's whole anchor set in one transaction.
func (s *Store) ReplaceAnchors(bookID string, anchors []story.CanonAnchor) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(`DELETE FROM canon_anchor WHERE book_id=?`, bookID); err != nil {
		return err
	}
	stmt, err := tx.Prepare(`INSERT INTO canon_anchor
		(id,book_id,scene_seq,deadline_seq,kind,summary,weight,source,created_at)
		VALUES (?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	now := time.Now().Unix()
	for _, a := range anchors {
		a = normaliseAnchor(a)
		if a.ID == "" {
			continue
		}
		if _, err := stmt.Exec(a.ID, bookID, a.SceneSeq, a.DeadlineSeq, a.Kind,
			a.Summary, a.Weight, a.Source, now); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// UpsertAnchor stores one anchor. created_at is preserved on update so the
// original ordering is not lost when the user edits a summary.
func (s *Store) UpsertAnchor(a story.CanonAnchor) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	a = normaliseAnchor(a)
	if a.ID == "" || a.BookID == "" {
		return errors.New("library: 锚点缺少 ID 或书籍")
	}
	_, err := s.db.Exec(`INSERT INTO canon_anchor
		(id,book_id,scene_seq,deadline_seq,kind,summary,weight,source,created_at)
		VALUES (?,?,?,?,?,?,?,?,?)
		ON CONFLICT(book_id,id) DO UPDATE SET
		 scene_seq=excluded.scene_seq, deadline_seq=excluded.deadline_seq,
		 kind=excluded.kind, summary=excluded.summary,
		 weight=excluded.weight, source=excluded.source`,
		a.ID, a.BookID, a.SceneSeq, a.DeadlineSeq, a.Kind, a.Summary, a.Weight, a.Source,
		time.Now().Unix())
	return err
}

// Anchors returns a book's anchors ordered by urgency.
func (s *Store) Anchors(bookID string) ([]story.CanonAnchor, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows, err := s.db.Query(`SELECT id,book_id,scene_seq,deadline_seq,kind,summary,weight,source
		FROM canon_anchor WHERE book_id=? ORDER BY deadline_seq ASC, scene_seq ASC, id ASC`, bookID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []story.CanonAnchor
	for rows.Next() {
		var a story.CanonAnchor
		if err := rows.Scan(&a.ID, &a.BookID, &a.SceneSeq, &a.DeadlineSeq, &a.Kind,
			&a.Summary, &a.Weight, &a.Source); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// Anchor returns one anchor.
func (s *Store) Anchor(bookID, id string) (*story.CanonAnchor, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var a story.CanonAnchor
	err := s.db.QueryRow(`SELECT id,book_id,scene_seq,deadline_seq,kind,summary,weight,source
		FROM canon_anchor WHERE book_id=? AND id=?`, bookID, id).
		Scan(&a.ID, &a.BookID, &a.SceneSeq, &a.DeadlineSeq, &a.Kind,
			&a.Summary, &a.Weight, &a.Source)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// DeleteAnchor removes one anchor.
func (s *Store) DeleteAnchor(bookID, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	res, err := s.db.Exec(`DELETE FROM canon_anchor WHERE book_id=? AND id=?`, bookID, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// AnchorCount reports how many anchors a book has.
func (s *Store) AnchorCount(bookID string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM canon_anchor WHERE book_id=?`, bookID).Scan(&n)
	return n, err
}

func normaliseAnchor(a story.CanonAnchor) story.CanonAnchor {
	if a.Kind == "" {
		a.Kind = story.AnchorBeat
	}
	if a.Weight <= 0 || a.Weight > 1 {
		a.Weight = 0.8
	}
	if a.Source == "" {
		a.Source = "manual"
	}
	if a.DeadlineSeq < a.SceneSeq {
		// A deadline before the canon scene would make the anchor impossible to
		// satisfy by its own route, so it is widened to the scene instead.
		a.DeadlineSeq = a.SceneSeq
	}
	return a
}
