package library

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"my-weread-desk/internal/story"
)

// ReplaceWorldSheet swaps a book's world sheet in one transaction.
func (s *Store) ReplaceWorldSheet(bookID string, sheet story.WorldSheet) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(`DELETE FROM world_stat WHERE book_id=?`, bookID); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM world_item WHERE book_id=?`, bookID); err != nil {
		return err
	}

	statStmt, err := tx.Prepare(`INSERT INTO world_stat
		(book_id,key,label,kind,min,max,initial,decay,evidence_seq,source)
		VALUES (?,?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer statStmt.Close()
	for _, def := range sheet.Stats {
		def = normaliseStat(def)
		if def.Key == "" {
			continue
		}
		if _, err := statStmt.Exec(bookID, def.Key, def.Label, def.Kind, def.Min, def.Max,
			def.Initial, def.Decay, def.EvidenceSeq, def.Source); err != nil {
			return err
		}
	}

	itemStmt, err := tx.Prepare(`INSERT INTO world_item
		(book_id,id,name,kind,owner,first_seq,effects,limit_per_branch,source)
		VALUES (?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer itemStmt.Close()
	for i, item := range sheet.Items {
		item = normaliseItem(item, i)
		if item.ID == "" || strings.TrimSpace(item.Name) == "" {
			continue
		}
		if _, err := itemStmt.Exec(bookID, item.ID, item.Name, item.Kind, item.Owner,
			item.FirstSeq, item.Effects, item.LimitPerBranch, item.Source); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// WorldSheet loads a book's world sheet. Missing tables yield an empty sheet so
// callers do not need a special case for "not derived yet".
func (s *Store) WorldSheet(bookID string) (*story.WorldSheet, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	sheet := &story.WorldSheet{BookID: bookID, Stats: []story.WorldStat{}, Items: []story.WorldItem{}}

	rows, err := s.db.Query(`SELECT key,label,kind,min,max,initial,decay,evidence_seq,source
		FROM world_stat WHERE book_id=? ORDER BY key ASC`, bookID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var def story.WorldStat
		if err := rows.Scan(&def.Key, &def.Label, &def.Kind, &def.Min, &def.Max,
			&def.Initial, &def.Decay, &def.EvidenceSeq, &def.Source); err != nil {
			_ = rows.Close()
			return nil, err
		}
		sheet.Stats = append(sheet.Stats, def)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}

	itemRows, err := s.db.Query(`SELECT id,name,kind,owner,first_seq,effects,limit_per_branch,source
		FROM world_item WHERE book_id=? ORDER BY first_seq ASC, name ASC`, bookID)
	if err != nil {
		return nil, err
	}
	defer itemRows.Close()
	for itemRows.Next() {
		var item story.WorldItem
		if err := itemRows.Scan(&item.ID, &item.Name, &item.Kind, &item.Owner,
			&item.FirstSeq, &item.Effects, &item.LimitPerBranch, &item.Source); err != nil {
			return nil, err
		}
		sheet.Items = append(sheet.Items, item)
	}
	return sheet, itemRows.Err()
}

// UpsertWorldStat stores one stat definition.
func (s *Store) UpsertWorldStat(bookID string, def story.WorldStat) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	def = normaliseStat(def)
	if def.Key == "" {
		return errors.New("library: 属性缺少 key")
	}
	_, err := s.db.Exec(`INSERT INTO world_stat
		(book_id,key,label,kind,min,max,initial,decay,evidence_seq,source)
		VALUES (?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(book_id,key) DO UPDATE SET
		 label=excluded.label, kind=excluded.kind, min=excluded.min, max=excluded.max,
		 initial=excluded.initial, decay=excluded.decay,
		 evidence_seq=excluded.evidence_seq, source=excluded.source`,
		bookID, def.Key, def.Label, def.Kind, def.Min, def.Max, def.Initial, def.Decay,
		def.EvidenceSeq, def.Source)
	return err
}

// UpsertWorldItem stores one item definition. An empty ID is generated.
func (s *Store) UpsertWorldItem(bookID string, item story.WorldItem) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	item = normaliseItem(item, 0)
	if strings.TrimSpace(item.Name) == "" {
		return errors.New("library: 道具缺少名称")
	}
	if item.ID == "" {
		item.ID = fmt.Sprintf("item-%d", time.Now().UnixNano())
	}
	_, err := s.db.Exec(`INSERT INTO world_item
		(book_id,id,name,kind,owner,first_seq,effects,limit_per_branch,source)
		VALUES (?,?,?,?,?,?,?,?,?)
		ON CONFLICT(book_id,id) DO UPDATE SET
		 name=excluded.name, kind=excluded.kind, owner=excluded.owner,
		 first_seq=excluded.first_seq, effects=excluded.effects,
		 limit_per_branch=excluded.limit_per_branch, source=excluded.source`,
		bookID, item.ID, item.Name, item.Kind, item.Owner, item.FirstSeq,
		item.Effects, item.LimitPerBranch, item.Source)
	return err
}

// DeleteWorldStat removes one stat definition.
func (s *Store) DeleteWorldStat(bookID, key string) error {
	return s.deleteWorldRow(`DELETE FROM world_stat WHERE book_id=? AND key=?`, bookID, key)
}

// DeleteWorldItem removes one item definition.
func (s *Store) DeleteWorldItem(bookID, id string) error {
	return s.deleteWorldRow(`DELETE FROM world_item WHERE book_id=? AND id=?`, bookID, id)
}

func (s *Store) deleteWorldRow(query, bookID, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	res, err := s.db.Exec(query, bookID, key)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// WorldSheetCounts reports how many stats and items a book has.
func (s *Store) WorldSheetCounts(bookID string) (stats, items int, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err = s.db.QueryRow(`SELECT COUNT(*) FROM world_stat WHERE book_id=?`, bookID).Scan(&stats); err != nil {
		return 0, 0, err
	}
	if err = s.db.QueryRow(`SELECT COUNT(*) FROM world_item WHERE book_id=?`, bookID).Scan(&items); err != nil {
		return 0, 0, err
	}
	return stats, items, nil
}

func normaliseStat(def story.WorldStat) story.WorldStat {
	def.Key = strings.TrimSpace(def.Key)
	def.Label = strings.TrimSpace(def.Label)
	if def.Label == "" {
		def.Label = def.Key
	}
	if def.Kind == "" {
		def.Kind = story.StatResource
	}
	if def.Max <= def.Min {
		def.Max = def.Min + 100
	}
	if def.Initial < def.Min || def.Initial > def.Max {
		def.Initial = def.Min
	}
	if def.Decay < 0 {
		def.Decay = 0
	}
	if def.Source == "" {
		def.Source = "derived"
	}
	return def
}

func normaliseItem(item story.WorldItem, index int) story.WorldItem {
	item.Name = strings.TrimSpace(item.Name)
	item.Kind = strings.TrimSpace(item.Kind)
	item.Owner = strings.TrimSpace(item.Owner)
	if item.ID == "" && index > 0 {
		item.ID = fmt.Sprintf("item-%d", index)
	}
	if item.LimitPerBranch <= 0 {
		item.LimitPerBranch = 1
	}
	if item.Source == "" {
		item.Source = "derived"
	}
	return item
}

// ensure database/sql stays referenced if the file is trimmed later.
var _ = sql.ErrNoRows
