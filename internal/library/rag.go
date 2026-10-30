package library

import (
	"database/sql"
	"errors"
	"strings"
	"time"

	"my-weread-desk/internal/rag"
)

// RAGChunkerVersion must be bumped whenever chunking changes shape, so an index
// built by an older algorithm is rebuilt instead of silently half-matching.
const RAGChunkerVersion = 1

// RAGIndex is the indexing state of one book.
type RAGIndex struct {
	BookID         string `json:"bookId"`
	Status         string `json:"status"`
	ChunkerVersion int    `json:"chunkerVersion"`
	ChunkCount     int    `json:"chunkCount"`
	CharCount      int    `json:"charCount"`
	Error          string `json:"error"`
	UpdatedAt      int64  `json:"updatedAt"`
}

// RAGIndex returns the index row, or ErrNotFound.
func (s *Store) RAGIndex(bookID string) (*RAGIndex, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var (
		idx RAGIndex
	)
	err := s.db.QueryRow(`SELECT book_id,status,chunker_version,chunk_count,char_count,error,updated_at
		FROM rag_index WHERE book_id=?`, bookID).
		Scan(&idx.BookID, &idx.Status, &idx.ChunkerVersion, &idx.ChunkCount, &idx.CharCount, &idx.Error, &idx.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &idx, nil
}

// SetRAGStatus records a state transition without touching the stored chunks.
// It is written before and after indexing so the UI can show progress and a
// failure stays visible instead of looking like "never indexed".
func (s *Store) SetRAGStatus(bookID, status, message string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`INSERT INTO rag_index(book_id,status,chunker_version,error,updated_at)
		VALUES (?,?,?,?,?)
		ON CONFLICT(book_id) DO UPDATE SET
		 status=excluded.status, error=excluded.error, updated_at=excluded.updated_at`,
		bookID, status, RAGChunkerVersion, message, time.Now().Unix())
	return err
}

// ReplaceRAGChunks swaps a book's whole index in one transaction. Replacing
// instead of appending keeps a re-index from accumulating stale chunks when a
// chapter was edited or the chunker changed.
func (s *Store) ReplaceRAGChunks(bookID string, chunks []rag.Chunk) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(`DELETE FROM rag_chunk WHERE book_id=?`, bookID); err != nil {
		return err
	}
	stmt, err := tx.Prepare(`INSERT INTO rag_chunk
		(book_id,seq,chapter_uid,chapter_idx,chapter_title,block_start,block_end,char_count,text)
		VALUES (?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	chars := 0
	for _, c := range chunks {
		if _, err := stmt.Exec(bookID, c.Seq, c.ChapterUID, c.ChapterIdx, c.ChapterTitle,
			c.BlockStart, c.BlockEnd, c.CharCount, c.Text); err != nil {
			return err
		}
		chars += c.CharCount
	}

	now := time.Now().Unix()
	if _, err := tx.Exec(`INSERT INTO rag_index
		(book_id,status,chunker_version,chunk_count,char_count,error,updated_at)
		VALUES (?,'ready',?,?,?,'',?)
		ON CONFLICT(book_id) DO UPDATE SET
		 status='ready', chunker_version=excluded.chunker_version,
		 chunk_count=excluded.chunk_count, char_count=excluded.char_count,
		 error='', updated_at=excluded.updated_at`,
		bookID, RAGChunkerVersion, len(chunks), chars, now); err != nil {
		return err
	}
	return tx.Commit()
}

// RAGChunks loads a book's chunks in retrieval order.
func (s *Store) RAGChunks(bookID string) ([]rag.Chunk, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows, err := s.db.Query(`SELECT book_id,seq,chapter_uid,chapter_idx,chapter_title,
		block_start,block_end,char_count,text
		FROM rag_chunk WHERE book_id=? ORDER BY seq ASC`, bookID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []rag.Chunk
	for rows.Next() {
		var c rag.Chunk
		if err := rows.Scan(&c.BookID, &c.Seq, &c.ChapterUID, &c.ChapterIdx, &c.ChapterTitle,
			&c.BlockStart, &c.BlockEnd, &c.CharCount, &c.Text); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// DropRAGIndex removes both the chunks and the state row.
func (s *Store) DropRAGIndex(bookID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec(`DELETE FROM rag_chunk WHERE book_id=?`, bookID); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM rag_index WHERE book_id=?`, bookID); err != nil {
		return err
	}
	return tx.Commit()
}

// IndexedBooks reports which of the given books have a usable index. The
// assistant uses it to avoid offering a book it cannot search.
func (s *Store) IndexedBooks(bookIDs []string) (map[string]RAGIndex, error) {
	if len(bookIDs) == 0 {
		return map[string]RAGIndex{}, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	stmts := make([]string, len(bookIDs))
	args := make([]any, len(bookIDs))
	for i, id := range bookIDs {
		stmts[i] = "?"
		args[i] = id
	}
	query := `SELECT book_id,status,chunker_version,chunk_count,char_count,error,updated_at
		FROM rag_index WHERE status='ready' AND book_id IN (` + strings.Join(stmts, ",") + `)`
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[string]RAGIndex{}
	for rows.Next() {
		var idx RAGIndex
		if err := rows.Scan(&idx.BookID, &idx.Status, &idx.ChunkerVersion, &idx.ChunkCount,
			&idx.CharCount, &idx.Error, &idx.UpdatedAt); err != nil {
			return nil, err
		}
		out[idx.BookID] = idx
	}
	return out, rows.Err()
}
