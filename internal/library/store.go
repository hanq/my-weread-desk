// Package library is the local cache: shelf snapshot, table of contents,
// decoded chapters, images, reading positions and bookkeeping flags.
package library

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	_ "modernc.org/sqlite"
)

const schemaVersion = 3

// Store wraps the SQLite cache. It intentionally uses a single connection:
// the app is a single user desktop client and serialising writes removes every
// lock-contention failure mode at a negligible cost.
type Store struct {
	mu   sync.Mutex
	db   *sql.DB
	path string
}

// Open creates or migrates the database at path.
func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	s := &Store{db: db, path: path}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

// Path returns the database file location.
func (s *Store) Path() string { return s.path }

// Close releases the database.
func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS meta (key TEXT PRIMARY KEY, value TEXT)`,
		`CREATE TABLE IF NOT EXISTS book (
			id TEXT PRIMARY KEY,
			title TEXT NOT NULL DEFAULT '',
			author TEXT NOT NULL DEFAULT '',
			cover TEXT NOT NULL DEFAULT '',
			category TEXT NOT NULL DEFAULT '',
			intro TEXT NOT NULL DEFAULT '',
			rating REAL NOT NULL DEFAULT 0,
			rating_count INTEGER NOT NULL DEFAULT 0,
			word_count INTEGER NOT NULL DEFAULT 0,
			finished INTEGER NOT NULL DEFAULT 0,
			secret INTEGER NOT NULL DEFAULT 0,
			is_mp INTEGER NOT NULL DEFAULT 0,
			last_read INTEGER NOT NULL DEFAULT 0,
			update_at INTEGER NOT NULL DEFAULT 0,
			shelf_order INTEGER NOT NULL DEFAULT 0,
			in_shelf INTEGER NOT NULL DEFAULT 0,
			format TEXT NOT NULL DEFAULT '',
			updated_at INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE INDEX IF NOT EXISTS idx_book_shelf ON book(in_shelf, shelf_order)`,
		`CREATE TABLE IF NOT EXISTS chapter (
			book_id TEXT NOT NULL,
			uid INTEGER NOT NULL,
			idx INTEGER NOT NULL DEFAULT 0,
			title TEXT NOT NULL DEFAULT '',
			level INTEGER NOT NULL DEFAULT 0,
			word_count INTEGER NOT NULL DEFAULT 0,
			paid INTEGER NOT NULL DEFAULT 0,
			tar TEXT NOT NULL DEFAULT '',
			format TEXT NOT NULL DEFAULT '',
			PRIMARY KEY (book_id, uid)
		)`,
		`CREATE TABLE IF NOT EXISTS chapter_content (
			book_id TEXT NOT NULL,
			uid INTEGER NOT NULL,
			title TEXT NOT NULL DEFAULT '',
			blocks_json TEXT NOT NULL,
			format TEXT NOT NULL DEFAULT '',
			fetched_at INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (book_id, uid)
		)`,
		`CREATE TABLE IF NOT EXISTS progress (
			book_id TEXT PRIMARY KEY,
			chapter_uid INTEGER NOT NULL DEFAULT 0,
			chapter_idx INTEGER NOT NULL DEFAULT 0,
			offset INTEGER NOT NULL DEFAULT 0,
			percent REAL NOT NULL DEFAULT 0,
			updated_at INTEGER NOT NULL DEFAULT 0,
			dirty INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE TABLE IF NOT EXISTS media (
			book_id TEXT NOT NULL,
			name TEXT NOT NULL,
			mime TEXT NOT NULL DEFAULT '',
			bytes BLOB,
			PRIMARY KEY (book_id, name)
		)`,
		`CREATE TABLE IF NOT EXISTS mp_article (
			review_id TEXT PRIMARY KEY,
			account_id TEXT NOT NULL DEFAULT '',
			title TEXT NOT NULL DEFAULT '',
			cover TEXT NOT NULL DEFAULT '',
			create_time INTEGER NOT NULL DEFAULT 0,
			content_json TEXT NOT NULL DEFAULT '',
			fetched_at INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE TABLE IF NOT EXISTS read_stat (
			mode TEXT NOT NULL,
			base_time INTEGER NOT NULL,
			payload_json TEXT NOT NULL,
			fetched_at INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (mode, base_time)
		)`,
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for _, stmt := range stmts {
		if _, err := tx.Exec(stmt); err != nil {
			return fmt.Errorf("library: migrate: %w", err)
		}
	}
	if _, err := tx.Exec(`INSERT INTO meta(key,value) VALUES('schema',?)
		ON CONFLICT(key) DO UPDATE SET value=excluded.value`, fmt.Sprint(schemaVersion)); err != nil {
		return err
	}
	return tx.Commit()
}

// GetMeta reads a meta value.
func (s *Store) GetMeta(key string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var v string
	_ = s.db.QueryRow(`SELECT value FROM meta WHERE key=?`, key).Scan(&v)
	return v
}

// SetMeta writes a meta value.
func (s *Store) SetMeta(key, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`INSERT INTO meta(key,value) VALUES(?,?)
		ON CONFLICT(key) DO UPDATE SET value=excluded.value`, key, value)
	return err
}
