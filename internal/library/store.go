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

const schemaVersion = 11

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
			is_audio INTEGER NOT NULL DEFAULT 0,
			track_count INTEGER NOT NULL DEFAULT 0,
			group_id TEXT NOT NULL DEFAULT '',
			group_name TEXT NOT NULL DEFAULT '',
			group_source TEXT NOT NULL DEFAULT '',
			last_read INTEGER NOT NULL DEFAULT 0,
			update_at INTEGER NOT NULL DEFAULT 0,
			shelf_order INTEGER NOT NULL DEFAULT 0,
			in_shelf INTEGER NOT NULL DEFAULT 0,
			format TEXT NOT NULL DEFAULT '',
			updated_at INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE TABLE IF NOT EXISTS shelf_group (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL DEFAULT '',
			source TEXT NOT NULL DEFAULT 'cloud',
			sort_order INTEGER NOT NULL DEFAULT 0,
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
		`CREATE TABLE IF NOT EXISTS device_read_time (
			day TEXT PRIMARY KEY,
			seconds INTEGER NOT NULL DEFAULT 0,
			updated_at INTEGER NOT NULL DEFAULT 0
		)`,
		// Retrieval index. The chunk table stores the text and, more importantly,
		// the block span so an answer can be opened at the exact paragraph.
		`CREATE TABLE IF NOT EXISTS rag_index (
			book_id TEXT PRIMARY KEY,
			status TEXT NOT NULL DEFAULT 'pending',
			chunker_version INTEGER NOT NULL DEFAULT 1,
			chunk_count INTEGER NOT NULL DEFAULT 0,
			char_count INTEGER NOT NULL DEFAULT 0,
			error TEXT NOT NULL DEFAULT '',
			updated_at INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE TABLE IF NOT EXISTS rag_chunk (
			book_id TEXT NOT NULL,
			seq INTEGER NOT NULL,
			chapter_uid INTEGER NOT NULL,
			chapter_idx INTEGER NOT NULL,
			chapter_title TEXT NOT NULL DEFAULT '',
			block_start INTEGER NOT NULL,
			block_end INTEGER NOT NULL,
			char_count INTEGER NOT NULL,
			text TEXT NOT NULL,
			PRIMARY KEY (book_id, seq)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_rag_chunk_book ON rag_chunk(book_id, seq)`,
		// Story graph. For the first iteration one chapter is one scene: it costs
		// no extra model calls and still gives the timeline a real anchor per
		// chapter. Finer segmentation can be layered on later without a schema
		// change, because block_start/block_end already describe the span.
		`CREATE TABLE IF NOT EXISTS story_scene (
			book_id TEXT NOT NULL,
			seq INTEGER NOT NULL,
			chapter_uid INTEGER NOT NULL,
			chapter_idx INTEGER NOT NULL,
			chapter_title TEXT NOT NULL DEFAULT '',
			block_start INTEGER NOT NULL DEFAULT 0,
			block_end INTEGER NOT NULL DEFAULT 0,
			location TEXT NOT NULL DEFAULT '',
			time_hint TEXT NOT NULL DEFAULT '',
			pov TEXT NOT NULL DEFAULT '',
			mood TEXT NOT NULL DEFAULT '',
			summary TEXT NOT NULL DEFAULT '',
			participants_json TEXT NOT NULL DEFAULT '[]',
			source TEXT NOT NULL DEFAULT 'chapter',
			updated_at INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (book_id, seq)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_story_scene_book ON story_scene(book_id, seq)`,
		// Job state lives in the database, not in memory: extracting a whole book
		// is expensive enough that a crash must resume instead of restarting.
		`CREATE TABLE IF NOT EXISTS story_job (
			book_id TEXT NOT NULL,
			job TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'pending',
			total INTEGER NOT NULL DEFAULT 0,
			done INTEGER NOT NULL DEFAULT 0,
			failed INTEGER NOT NULL DEFAULT 0,
			error TEXT NOT NULL DEFAULT '',
			updated_at INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (book_id, job)
		)`,
		`CREATE TABLE IF NOT EXISTS story_job_item (
			book_id TEXT NOT NULL,
			job TEXT NOT NULL,
			item_key TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'pending',
			error TEXT NOT NULL DEFAULT '',
			updated_at INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (book_id, job, item_key)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_story_job_item ON story_job_item(book_id, job, status)`,
		// Canon anchors are the load-bearing beats of the original work. They are
		// book-level definitions; whether a branch has realised one is branch
		// state, tracked separately, so the same graph serves every world line.
		`CREATE TABLE IF NOT EXISTS canon_anchor (
			id TEXT NOT NULL,
			book_id TEXT NOT NULL,
			scene_seq INTEGER NOT NULL DEFAULT 0,
			deadline_seq INTEGER NOT NULL DEFAULT 0,
			kind TEXT NOT NULL DEFAULT 'beat',
			summary TEXT NOT NULL DEFAULT '',
			weight REAL NOT NULL DEFAULT 0.8,
			source TEXT NOT NULL DEFAULT 'manual',
			created_at INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (book_id, id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_canon_anchor_book ON canon_anchor(book_id, deadline_seq)`,
		`CREATE INDEX IF NOT EXISTS idx_canon_anchor_scene ON canon_anchor(book_id, scene_seq)`,
		// World sheet: the small, book-specific number set derived from the text.
		// It is deliberately tiny — a model cannot track ten stats reliably.
		`CREATE TABLE IF NOT EXISTS world_stat (
			book_id TEXT NOT NULL,
			key TEXT NOT NULL,
			label TEXT NOT NULL DEFAULT '',
			kind TEXT NOT NULL DEFAULT 'resource',
			min REAL NOT NULL DEFAULT 0,
			max REAL NOT NULL DEFAULT 100,
			initial REAL NOT NULL DEFAULT 0,
			decay REAL NOT NULL DEFAULT 0,
			evidence_seq INTEGER NOT NULL DEFAULT -1,
			source TEXT NOT NULL DEFAULT 'derived',
			PRIMARY KEY (book_id, key)
		)`,
		`CREATE TABLE IF NOT EXISTS world_item (
			book_id TEXT NOT NULL,
			id TEXT NOT NULL,
			name TEXT NOT NULL DEFAULT '',
			kind TEXT NOT NULL DEFAULT '',
			owner TEXT NOT NULL DEFAULT '',
			first_seq INTEGER NOT NULL DEFAULT -1,
			effects TEXT NOT NULL DEFAULT '',
			limit_per_branch INTEGER NOT NULL DEFAULT 1,
			source TEXT NOT NULL DEFAULT 'derived',
			PRIMARY KEY (book_id, id)
		)`,
		// A branch is a world line. Its constitution and state live in the row so
		// a branch can be exported or resumed without extra side files.
		`CREATE TABLE IF NOT EXISTS story_branch (
			id TEXT PRIMARY KEY,
			book_id TEXT NOT NULL,
			fork_seq INTEGER NOT NULL DEFAULT 0,
			rejoin_seq INTEGER NOT NULL DEFAULT 0,
			title TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'active',
			summary TEXT NOT NULL DEFAULT '',
			state_json TEXT NOT NULL DEFAULT '{}',
			memory_md TEXT NOT NULL DEFAULT '',
			created_at INTEGER NOT NULL DEFAULT 0,
			updated_at INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE INDEX IF NOT EXISTS idx_story_branch_book ON story_branch(book_id, created_at)`,
		`CREATE TABLE IF NOT EXISTS story_branch_scene (
			branch_id TEXT NOT NULL,
			seq INTEGER NOT NULL,
			canon_seq INTEGER NOT NULL DEFAULT -1,
			text TEXT NOT NULL,
			summary TEXT NOT NULL DEFAULT '',
			verdict_json TEXT NOT NULL DEFAULT '',
			choices_json TEXT NOT NULL DEFAULT '[]',
			model TEXT NOT NULL DEFAULT '',
			created_at INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (branch_id, seq)
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
	// Existing databases were created before the shelf gained folders and
	// audiobooks, and CREATE TABLE IF NOT EXISTS cannot add columns. Every
	// column is added only when PRAGMA reports it missing so the migration is
	// idempotent across restarts and partial upgrades.
	for _, col := range []struct{ name, spec string }{
		{"is_audio", "INTEGER NOT NULL DEFAULT 0"},
		{"track_count", "INTEGER NOT NULL DEFAULT 0"},
		{"group_id", "TEXT NOT NULL DEFAULT ''"},
		{"group_name", "TEXT NOT NULL DEFAULT ''"},
		{"group_source", "TEXT NOT NULL DEFAULT ''"},
	} {
		if err := ensureColumn(tx, "book", col.name, col.spec); err != nil {
			return err
		}
	}
	// story_branch_scene gained a choices column after branches shipped, so an
	// existing database needs the same guarded ALTER.
	if err := ensureColumn(tx, "story_branch_scene", "choices_json", "TEXT NOT NULL DEFAULT '[]'"); err != nil {
		return err
	}
	// An index on a new column can only be created after the ALTER TABLE pass:
	// on a database from the previous schema the column does not exist yet.
	if _, err := tx.Exec(`CREATE INDEX IF NOT EXISTS idx_book_group ON book(group_id)`); err != nil {
		return fmt.Errorf("library: migrate: %w", err)
	}
	if _, err := tx.Exec(`INSERT INTO meta(key,value) VALUES('schema',?)
		ON CONFLICT(key) DO UPDATE SET value=excluded.value`, fmt.Sprint(schemaVersion)); err != nil {
		return err
	}
	return tx.Commit()
}

// ensureColumn adds a column when the table does not already define it. SQLite
// has no ADD COLUMN IF NOT EXISTS, so PRAGMA table_info is the only portable
// way to keep the migration re-runnable.
func ensureColumn(tx *sql.Tx, table, name, spec string) error {
	rows, err := tx.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		return fmt.Errorf("library: migrate: inspect %s: %w", table, err)
	}
	found := false
	for rows.Next() {
		var (
			cid     int
			colName string
			colType string
			notNull int
			dflt    sql.NullString
			pk      int
		)
		if err := rows.Scan(&cid, &colName, &colType, &notNull, &dflt, &pk); err != nil {
			_ = rows.Close()
			return fmt.Errorf("library: migrate: read %s: %w", table, err)
		}
		if colName == name {
			found = true
		}
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if found {
		return nil
	}
	if _, err := tx.Exec(`ALTER TABLE ` + table + ` ADD COLUMN ` + name + ` ` + spec); err != nil {
		return fmt.Errorf("library: migrate: add %s.%s: %w", table, name, err)
	}
	return nil
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
