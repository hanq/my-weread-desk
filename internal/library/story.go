package library

import (
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"my-weread-desk/internal/story"
)

// StoryJobSceneExtract is the job that turns cached chapters into scenes.
const StoryJobSceneExtract = "scene_extract"

// StoryJob is the persisted progress of a batch extraction.
type StoryJob struct {
	BookID    string `json:"bookId"`
	Job       string `json:"job"`
	Status    string `json:"status"`
	Total     int    `json:"total"`
	Done      int    `json:"done"`
	Failed    int    `json:"failed"`
	Error     string `json:"error"`
	UpdatedAt int64  `json:"updatedAt"`
}

// InitStoryJob creates the job and its per-chapter items.
//
// Existing items keep their status, which is what makes the job resumable: a
// restart after a crash skips every chapter that already succeeded instead of
// paying for it again.
func (s *Store) InitStoryJob(bookID, job string, keys []string) (StoryJob, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return StoryJob{}, err
	}
	defer func() { _ = tx.Rollback() }()

	now := time.Now().Unix()
	if _, err := tx.Exec(`INSERT INTO story_job(book_id,job,status,updated_at)
		VALUES (?,?,'pending',?)
		ON CONFLICT(book_id,job) DO UPDATE SET updated_at=excluded.updated_at`,
		bookID, job, now); err != nil {
		return StoryJob{}, err
	}
	stmt, err := tx.Prepare(`INSERT INTO story_job_item(book_id,job,item_key,status,updated_at)
		VALUES (?,?,?,'pending',?)
		ON CONFLICT(book_id,job,item_key) DO UPDATE SET updated_at=excluded.updated_at`)
	if err != nil {
		return StoryJob{}, err
	}
	defer stmt.Close()
	for _, key := range keys {
		if key == "" {
			continue
		}
		if _, err := stmt.Exec(bookID, job, key, now); err != nil {
			return StoryJob{}, err
		}
	}
	if _, err := tx.Exec(`UPDATE story_job SET
		 total=(SELECT COUNT(*) FROM story_job_item WHERE book_id=? AND job=?),
		 done=(SELECT COUNT(*) FROM story_job_item WHERE book_id=? AND job=? AND status='done'),
		 failed=(SELECT COUNT(*) FROM story_job_item WHERE book_id=? AND job=? AND status='failed'),
		 updated_at=?
		WHERE book_id=? AND job=?`,
		bookID, job, bookID, job, bookID, job, now, bookID, job); err != nil {
		return StoryJob{}, err
	}
	if err := tx.Commit(); err != nil {
		return StoryJob{}, err
	}
	return s.storyJobLocked(bookID, job)
}

// StoryJob returns the current job row.
func (s *Store) StoryJob(bookID, job string) (*StoryJob, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	j, err := s.storyJobLocked(bookID, job)
	if err != nil {
		return nil, err
	}
	return &j, nil
}

func (s *Store) storyJobLocked(bookID, job string) (StoryJob, error) {
	var j StoryJob
	err := s.db.QueryRow(`SELECT book_id,job,status,total,done,failed,error,updated_at
		FROM story_job WHERE book_id=? AND job=?`, bookID, job).
		Scan(&j.BookID, &j.Job, &j.Status, &j.Total, &j.Done, &j.Failed, &j.Error, &j.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return j, ErrNotFound
	}
	return j, err
}

// SetStoryJobStatus records a state transition and refreshes the counters.
func (s *Store) SetStoryJobStatus(bookID, job, status, message string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`UPDATE story_job SET status=?, error=?, updated_at=?,
		 done=(SELECT COUNT(*) FROM story_job_item WHERE book_id=? AND job=? AND status='done'),
		 failed=(SELECT COUNT(*) FROM story_job_item WHERE book_id=? AND job=? AND status='failed')
		WHERE book_id=? AND job=?`,
		status, message, time.Now().Unix(),
		bookID, job, bookID, job, bookID, job)
	return err
}

// PendingStoryItems returns chapters that still need work. Failed items are
// included so a manual re-run retries them instead of leaving permanent holes.
func (s *Store) PendingStoryItems(bookID, job string) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows, err := s.db.Query(`SELECT item_key FROM story_job_item
		WHERE book_id=? AND job=? AND status<>'done' ORDER BY item_key ASC`, bookID, job)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, err
		}
		out = append(out, key)
	}
	return out, rows.Err()
}

// MarkStoryItem records one chapter's outcome.
func (s *Store) MarkStoryItem(bookID, job, key, status, message string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`UPDATE story_job_item SET status=?, error=?, updated_at=?
		WHERE book_id=? AND job=? AND item_key=?`,
		status, message, time.Now().Unix(), bookID, job, key)
	return err
}

// ---------------------------------------------------------------- scenes

// UpsertStoryScene stores one scene. Re-running a chapter replaces its row
// instead of duplicating it.
func (s *Store) UpsertStoryScene(sc story.Scene) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	participants, err := json.Marshal(sc.Participants)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`INSERT INTO story_scene
		(book_id,seq,chapter_uid,chapter_idx,chapter_title,block_start,block_end,
		 location,time_hint,pov,mood,summary,participants_json,source,updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(book_id,seq) DO UPDATE SET
		 chapter_uid=excluded.chapter_uid, chapter_idx=excluded.chapter_idx,
		 chapter_title=excluded.chapter_title, block_start=excluded.block_start,
		 block_end=excluded.block_end, location=excluded.location,
		 time_hint=excluded.time_hint, pov=excluded.pov, mood=excluded.mood,
		 summary=excluded.summary, participants_json=excluded.participants_json,
		 source=excluded.source, updated_at=excluded.updated_at`,
		sc.BookID, sc.Seq, sc.ChapterUID, sc.ChapterIdx, sc.ChapterTitle,
		sc.BlockStart, sc.BlockEnd, sc.Location, sc.TimeHint, sc.POV, sc.Mood,
		sc.Summary, string(participants), sc.Source, time.Now().Unix())
	return err
}

// ReplaceStoryScenes swaps the whole graph in one transaction.
func (s *Store) ReplaceStoryScenes(bookID string, scenes []story.Scene) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec(`DELETE FROM story_scene WHERE book_id=?`, bookID); err != nil {
		return err
	}
	stmt, err := tx.Prepare(`INSERT INTO story_scene
		(book_id,seq,chapter_uid,chapter_idx,chapter_title,block_start,block_end,
		 location,time_hint,pov,mood,summary,participants_json,source,updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	now := time.Now().Unix()
	for _, sc := range scenes {
		participants, merr := json.Marshal(sc.Participants)
		if merr != nil {
			return merr
		}
		if _, err := stmt.Exec(sc.BookID, sc.Seq, sc.ChapterUID, sc.ChapterIdx, sc.ChapterTitle,
			sc.BlockStart, sc.BlockEnd, sc.Location, sc.TimeHint, sc.POV, sc.Mood,
			sc.Summary, string(participants), sc.Source, now); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// StoryScenes returns the timeline in reading order.
func (s *Store) StoryScenes(bookID string) ([]story.Scene, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows, err := s.db.Query(`SELECT book_id,seq,chapter_uid,chapter_idx,chapter_title,
		block_start,block_end,location,time_hint,pov,mood,summary,participants_json,source
		FROM story_scene WHERE book_id=? ORDER BY seq ASC`, bookID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []story.Scene
	for rows.Next() {
		var (
			sc           story.Scene
			participants string
		)
		if err := rows.Scan(&sc.BookID, &sc.Seq, &sc.ChapterUID, &sc.ChapterIdx, &sc.ChapterTitle,
			&sc.BlockStart, &sc.BlockEnd, &sc.Location, &sc.TimeHint, &sc.POV, &sc.Mood,
			&sc.Summary, &participants, &sc.Source); err != nil {
			return nil, err
		}
		if participants != "" {
			_ = json.Unmarshal([]byte(participants), &sc.Participants)
		}
		if sc.Participants == nil {
			sc.Participants = []string{}
		}
		out = append(out, sc)
	}
	return out, rows.Err()
}

// DropStoryGraph removes scenes and job state for a book.
func (s *Store) DropStoryGraph(bookID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for _, stmt := range []string{
		`DELETE FROM story_scene WHERE book_id=?`,
		`DELETE FROM story_job_item WHERE book_id=?`,
		`DELETE FROM story_job WHERE book_id=?`,
	} {
		if _, err := tx.Exec(stmt, bookID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// SceneCount reports how many scenes a book has.
func (s *Store) SceneCount(bookID string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM story_scene WHERE book_id=?`, bookID).Scan(&n)
	return n, err
}

// LastStoryItemError returns the first failure message, for the job banner.
func (s *Store) LastStoryItemError(bookID, job string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var msg string
	err := s.db.QueryRow(`SELECT error FROM story_job_item
		WHERE book_id=? AND job=? AND status='failed' AND error<>''
		ORDER BY updated_at DESC LIMIT 1`, bookID, job).Scan(&msg)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return strings.TrimSpace(msg), err
}
