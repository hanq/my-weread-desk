package library

import (
	"database/sql"
	"errors"
	"strings"
	"time"
)

// BookAI is the persisted AI classification of one book. User corrections use
// KindSource="user" and survive later shelf syncs; automatic guesses use
// KindSource="auto".
type BookAI struct {
	BookID     string `json:"bookId"`
	Kind       string `json:"kind"`
	KindSource string `json:"kindSource"`
	UpdatedAt  int64  `json:"updatedAt"`
}

// BookAI returns the classification row, or ErrNotFound.
func (s *Store) BookAI(bookID string) (*BookAI, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var out BookAI
	err := s.db.QueryRow(`SELECT book_id,kind,kind_source,updated_at
		FROM book_ai WHERE book_id=?`, bookID).
		Scan(&out.BookID, &out.Kind, &out.KindSource, &out.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// BookAIMap loads classifications for a batch of books. Missing rows are simply
// absent from the map, so the caller can classify and persist them lazily.
func (s *Store) BookAIMap(bookIDs []string) (map[string]BookAI, error) {
	out := map[string]BookAI{}
	if len(bookIDs) == 0 {
		return out, nil
	}

	seen := map[string]bool{}
	ph := make([]string, 0, len(bookIDs))
	args := make([]any, 0, len(bookIDs))
	for _, id := range bookIDs {
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		ph = append(ph, "?")
		args = append(args, id)
	}
	if len(ph) == 0 {
		return out, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	rows, err := s.db.Query(`SELECT book_id,kind,kind_source,updated_at
		FROM book_ai WHERE book_id IN (`+strings.Join(ph, ",")+`)`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var ai BookAI
		if err := rows.Scan(&ai.BookID, &ai.Kind, &ai.KindSource, &ai.UpdatedAt); err != nil {
			return nil, err
		}
		out[ai.BookID] = ai
	}
	return out, rows.Err()
}

// UpsertBookAI stores one classification. The caller decides whether the value
// is an automatic guess or a user correction.
func (s *Store) UpsertBookAI(bookID, kind, source string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`INSERT INTO book_ai(book_id,kind,kind_source,updated_at)
		VALUES (?,?,?,?)
		ON CONFLICT(book_id) DO UPDATE SET
		 kind=excluded.kind, kind_source=excluded.kind_source,
		 updated_at=excluded.updated_at`,
		bookID, kind, source, time.Now().Unix())
	return err
}
