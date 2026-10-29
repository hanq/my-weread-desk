package library

import (
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"my-weread-desk/internal/weread"
)

// ErrNotFound is returned when a cached row is missing.
var ErrNotFound = errors.New("library: not found")

// ReplaceShelf reconciles the cached shelf with a fresh snapshot.
func (s *Store) ReplaceShelf(books []weread.Book) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	now := time.Now().Unix()
	if _, err := tx.Exec(`UPDATE book SET in_shelf=0`); err != nil {
		return err
	}
	stmt, err := tx.Prepare(`INSERT INTO book
		(id,title,author,cover,category,intro,rating,rating_count,word_count,finished,secret,is_mp,
		 last_read,update_at,shelf_order,in_shelf,updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,1,?)
		ON CONFLICT(id) DO UPDATE SET
		 title=excluded.title, author=excluded.author, cover=excluded.cover,
		 category=excluded.category, intro=CASE WHEN excluded.intro<>'' THEN excluded.intro ELSE book.intro END,
		 rating=CASE WHEN excluded.rating>0 THEN excluded.rating ELSE book.rating END,
		 word_count=CASE WHEN excluded.word_count>0 THEN excluded.word_count ELSE book.word_count END,
		 finished=excluded.finished, secret=excluded.secret, is_mp=excluded.is_mp,
		 last_read=excluded.last_read, update_at=excluded.update_at,
		 shelf_order=excluded.shelf_order, in_shelf=1, updated_at=excluded.updated_at`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for i, b := range books {
		if _, err := stmt.Exec(
			b.ID, b.Title, b.Author, b.Cover, b.Category, b.Intro, b.Rating, b.RatingN,
			b.WordCount, boolInt(b.Finished), boolInt(b.Secret), boolInt(b.IsMP),
			b.LastRead, b.UpdateAt, i, now,
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// UpsertBook stores a single book without touching its shelf membership.
func (s *Store) UpsertBook(b weread.Book) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`INSERT INTO book
		(id,title,author,cover,category,intro,rating,rating_count,word_count,finished,secret,is_mp,
		 last_read,update_at,in_shelf,updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,1,?)
		ON CONFLICT(id) DO UPDATE SET
		 title=excluded.title, author=excluded.author, cover=excluded.cover,
		 category=excluded.category, intro=excluded.intro, rating=excluded.rating,
		 rating_count=excluded.rating_count,
		 word_count=CASE WHEN excluded.word_count>0 THEN excluded.word_count ELSE book.word_count END,
		 finished=excluded.finished, secret=excluded.secret, is_mp=excluded.is_mp,
		 last_read=CASE WHEN excluded.last_read>0 THEN excluded.last_read ELSE book.last_read END,
		 update_at=excluded.update_at, updated_at=excluded.updated_at`,
		b.ID, b.Title, b.Author, b.Cover, b.Category, b.Intro, b.Rating, b.RatingN,
		b.WordCount, boolInt(b.Finished), boolInt(b.Secret), boolInt(b.IsMP),
		b.LastRead, b.UpdateAt, time.Now().Unix())
	return err
}

const bookColumns = `id,title,author,cover,category,intro,rating,rating_count,word_count,
	finished,secret,is_mp,last_read,update_at`

// Books returns the cached shelf ordered by the last sync order.
func (s *Store) Books() ([]weread.Book, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows, err := s.db.Query(`SELECT ` + bookColumns + `, shelf_order FROM book
		WHERE in_shelf=1 ORDER BY last_read DESC, update_at DESC, shelf_order ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanBooks(rows, true)
}

// AllBooks returns every cached book, including ones removed from the shelf.
func (s *Store) AllBooks() ([]weread.Book, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows, err := s.db.Query(`SELECT ` + bookColumns + `, shelf_order FROM book ORDER BY last_read DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanBooks(rows, true)
}

// Book returns a single cached book.
func (s *Store) Book(id string) (*weread.Book, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	row := s.db.QueryRow(`SELECT `+bookColumns+` FROM book WHERE id=?`, id)
	b, err := scanBookRow(row)
	if err != nil {
		return nil, err
	}
	return b, nil
}

// SetBookFormat remembers whether a book is epub or txt.
func (s *Store) SetBookFormat(bookID, format string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`UPDATE book SET format=? WHERE id=?`, format, bookID)
	return err
}

// BookFormat returns the cached format (may be empty).
func (s *Store) BookFormat(bookID string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var f string
	_ = s.db.QueryRow(`SELECT format FROM book WHERE id=?`, bookID).Scan(&f)
	return f
}

func scanBookRow(row *sql.Row) (*weread.Book, error) {
	var (
		b                      weread.Book
		finished, secret, isMP int
	)
	err := row.Scan(
		&b.ID, &b.Title, &b.Author, &b.Cover, &b.Category, &b.Intro, &b.Rating,
		&b.RatingN, &b.WordCount, &finished, &secret, &isMP, &b.LastRead, &b.UpdateAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	b.Finished = finished != 0
	b.Secret = secret != 0
	b.IsMP = isMP != 0
	return &b, nil
}

func scanBooks(rows *sql.Rows, withOrder bool) ([]weread.Book, error) {
	var out []weread.Book
	for rows.Next() {
		var (
			b                             weread.Book
			finished, secret, isMP, order int
		)
		if err := rows.Scan(
			&b.ID, &b.Title, &b.Author, &b.Cover, &b.Category, &b.Intro, &b.Rating,
			&b.RatingN, &b.WordCount, &finished, &secret, &isMP, &b.LastRead, &b.UpdateAt, &order,
		); err != nil {
			return nil, err
		}
		b.Finished = finished != 0
		b.Secret = secret != 0
		b.IsMP = isMP != 0
		out = append(out, b)
	}
	return out, rows.Err()
}

// ---------------------------------------------------------------- chapters

// ReplaceChapters stores a table of contents.
func (s *Store) ReplaceChapters(bookID string, chapters []weread.Chapter) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(`DELETE FROM chapter WHERE book_id=?`, bookID); err != nil {
		return err
	}
	stmt, err := tx.Prepare(`INSERT INTO chapter
		(book_id,uid,idx,title,level,word_count,paid,tar,format)
		VALUES (?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, c := range chapters {
		if _, err := stmt.Exec(bookID, c.UID, c.Index, c.Title, c.Level, c.WordCount,
			boolInt(c.Paid), c.Tar, c.Format); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// Chapters returns the cached table of contents.
func (s *Store) Chapters(bookID string) ([]weread.Chapter, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows, err := s.db.Query(`SELECT uid,idx,title,level,word_count,paid,tar,format
		FROM chapter WHERE book_id=? ORDER BY idx ASC, uid ASC`, bookID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []weread.Chapter
	for rows.Next() {
		var (
			c    weread.Chapter
			paid int
		)
		if err := rows.Scan(&c.UID, &c.Index, &c.Title, &c.Level, &c.WordCount,
			&paid, &c.Tar, &c.Format); err != nil {
			return nil, err
		}
		c.Paid = paid != 0
		out = append(out, c)
	}
	return out, rows.Err()
}

// SaveChapterContent caches a decoded chapter.
func (s *Store) SaveChapterContent(c *weread.ChapterContent) error {
	if c == nil {
		return errors.New("library: nil chapter")
	}
	blocks, err := json.Marshal(c.Blocks)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err = s.db.Exec(`INSERT INTO chapter_content
		(book_id,uid,title,blocks_json,format,fetched_at) VALUES (?,?,?,?,?,?)
		ON CONFLICT(book_id,uid) DO UPDATE SET
		 title=excluded.title, blocks_json=excluded.blocks_json,
		 format=excluded.format, fetched_at=excluded.fetched_at`,
		c.BookID, c.ChapterUID, c.Title, string(blocks), c.Format, c.FetchedAt)
	return err
}

// ChapterContent loads a cached chapter.
func (s *Store) ChapterContent(bookID string, uid int64) (*weread.ChapterContent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var (
		c          weread.ChapterContent
		jsonBlocks string
	)
	err := s.db.QueryRow(`SELECT book_id,uid,title,blocks_json,format,fetched_at
		FROM chapter_content WHERE book_id=? AND uid=?`, bookID, uid).
		Scan(&c.BookID, &c.ChapterUID, &c.Title, &jsonBlocks, &c.Format, &c.FetchedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(jsonBlocks), &c.Blocks); err != nil {
		return nil, err
	}
	return &c, nil
}

// HasChapterContent reports whether a chapter is cached.
func (s *Store) HasChapterContent(bookID string, uid int64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	var n int
	_ = s.db.QueryRow(`SELECT COUNT(1) FROM chapter_content WHERE book_id=? AND uid=?`,
		bookID, uid).Scan(&n)
	return n > 0
}

// CachedChapterUIDs returns the set of cached chapter ids for a book.
func (s *Store) CachedChapterUIDs(bookID string) (map[int64]bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows, err := s.db.Query(`SELECT uid FROM chapter_content WHERE book_id=?`, bookID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]bool{}
	for rows.Next() {
		var uid int64
		if err := rows.Scan(&uid); err != nil {
			return nil, err
		}
		out[uid] = true
	}
	return out, rows.Err()
}

// DropChapterContent removes cached chapters above a schema change.
func (s *Store) DropChapterContent(bookID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`DELETE FROM chapter_content WHERE book_id=?`, bookID)
	return err
}

// ---------------------------------------------------------------- progress

// SaveProgress stores a local or cloud reading position.
func (s *Store) SaveProgress(p weread.Progress, dirty bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`INSERT INTO progress
		(book_id,chapter_uid,chapter_idx,offset,percent,updated_at,dirty) VALUES (?,?,?,?,?,?,?)
		ON CONFLICT(book_id) DO UPDATE SET
		 chapter_uid=excluded.chapter_uid, chapter_idx=excluded.chapter_idx,
		 offset=excluded.offset, percent=excluded.percent,
		 updated_at=excluded.updated_at, dirty=excluded.dirty`,
		p.BookID, p.ChapterUID, p.ChapterIndex, p.Offset, p.Percent,
		firstNonZero(p.UpdateTime, time.Now().Unix()), boolInt(dirty))
	return err
}

// Progress loads the stored position.
func (s *Store) Progress(bookID string) (*weread.Progress, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var (
		p     weread.Progress
		dirty int
	)
	err := s.db.QueryRow(`SELECT book_id,chapter_uid,chapter_idx,offset,percent,updated_at,dirty
		FROM progress WHERE book_id=?`, bookID).
		Scan(&p.BookID, &p.ChapterUID, &p.ChapterIndex, &p.Offset, &p.Percent, &p.UpdateTime, &dirty)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// DirtyProgress lists positions that still need to be pushed to the server.
func (s *Store) DirtyProgress() ([]weread.Progress, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows, err := s.db.Query(`SELECT book_id,chapter_uid,chapter_idx,offset,percent,updated_at
		FROM progress WHERE dirty=1`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []weread.Progress
	for rows.Next() {
		var p weread.Progress
		if err := rows.Scan(&p.BookID, &p.ChapterUID, &p.ChapterIndex, &p.Offset, &p.Percent, &p.UpdateTime); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// ---------------------------------------------------------------- media

// SaveMedia stores an image blob.
func (s *Store) SaveMedia(bookID, name, mime string, data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`INSERT INTO media (book_id,name,mime,bytes) VALUES (?,?,?,?)
		ON CONFLICT(book_id,name) DO UPDATE SET mime=excluded.mime, bytes=excluded.bytes`,
		bookID, name, mime, data)
	return err
}

// Media loads an image blob.
func (s *Store) Media(bookID, name string) ([]byte, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var (
		data []byte
		mime string
	)
	err := s.db.QueryRow(`SELECT bytes,mime FROM media WHERE book_id=? AND name=?`,
		bookID, name).Scan(&data, &mime)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, "", ErrNotFound
	}
	if err != nil {
		return nil, "", err
	}
	return data, mime, nil
}

// HasMedia reports whether an image is cached.
func (s *Store) HasMedia(bookID, name string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	var n int
	_ = s.db.QueryRow(`SELECT COUNT(1) FROM media WHERE book_id=? AND name=?`, bookID, name).Scan(&n)
	return n > 0
}

// ---------------------------------------------------------------- mp articles

// SaveMPArticle caches an official-account article.
func (s *Store) SaveMPArticle(a weread.MPArticle, blocks []weread.Block) error {
	raw, err := json.Marshal(blocks)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err = s.db.Exec(`INSERT INTO mp_article
		(review_id,account_id,title,cover,create_time,content_json,fetched_at)
		VALUES (?,?,?,?,?,?,?)
		ON CONFLICT(review_id) DO UPDATE SET
		 title=excluded.title, cover=excluded.cover, content_json=excluded.content_json,
		 fetched_at=excluded.fetched_at`,
		a.ReviewID, a.Account, a.Title, a.Cover, a.CreateTime, string(raw), time.Now().Unix())
	return err
}

// MPArticle returns a cached article.
func (s *Store) MPArticle(reviewID string) (*weread.MPArticle, []weread.Block, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var (
		a          weread.MPArticle
		jsonBlocks string
	)
	err := s.db.QueryRow(`SELECT review_id,account_id,title,cover,create_time,content_json
		FROM mp_article WHERE review_id=?`, reviewID).
		Scan(&a.ReviewID, &a.Account, &a.Title, &a.Cover, &a.CreateTime, &jsonBlocks)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil, ErrNotFound
	}
	if err != nil {
		return nil, nil, err
	}
	var blocks []weread.Block
	if err := json.Unmarshal([]byte(jsonBlocks), &blocks); err != nil {
		return nil, nil, err
	}
	return &a, blocks, nil
}

// SaveReadStats caches a statistics payload.
func (s *Store) SaveReadStats(mode string, baseTime int64, stats *weread.ReadStats) error {
	raw, err := json.Marshal(stats)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err = s.db.Exec(`INSERT INTO read_stat (mode,base_time,payload_json,fetched_at)
		VALUES (?,?,?,?) ON CONFLICT(mode,base_time) DO UPDATE SET
		payload_json=excluded.payload_json, fetched_at=excluded.fetched_at`,
		mode, baseTime, string(raw), time.Now().Unix())
	return err
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func firstNonZero(values ...int64) int64 {
	for _, v := range values {
		if v != 0 {
			return v
		}
	}
	return 0
}
