// Command rag_probe measures the chunker and the BM25 index against the books
// already cached on this machine.
//
// It opens the cache read-only and never writes: the point is to see real chunk
// counts, index build time and retrieval results before any of it is wired into
// the app. Chapter text is printed only as a short excerpt, because the output
// is meant for a developer looking at ranking quality.
package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"my-weread-desk/internal/config"
	"my-weread-desk/internal/rag"
	"my-weread-desk/internal/weread"
)

func main() {
	var (
		bookID  = flag.String("book", "", "书籍 ID；留空则取缓存章节最多的一本")
		list    = flag.Bool("list", false, "只列出缓存最多的书籍，不跑分块")
		queries flagList
		limit   = flag.Int("limit", 3, "每个查询返回的条数")
		excerpt = flag.Int("excerpt", 60, "命中片段打印的字数")
	)
	flag.Var(&queries, "q", "检索词，可重复")
	flag.Parse()

	dataDir, err := config.DataDir()
	if err != nil {
		fail("读取数据目录失败: %v", err)
	}
	path := filepath.Join(dataDir, "library.db")
	db, err := sql.Open("sqlite", path+"?mode=ro")
	if err != nil {
		fail("打开缓存库失败: %v", err)
	}
	defer db.Close()

	if *list {
		listCached(db)
		return
	}

	id, title, chapters, err := pickBook(db, *bookID)
	if err != nil {
		fail("%v", err)
	}
	fmt.Printf("书籍: %s (%s)\n", title, id)

	blocks, err := loadBlocks(db, id)
	if err != nil {
		fail("读取章节正文失败: %v", err)
	}
	chars := 0
	for _, list := range blocks {
		for _, b := range list {
			chars += len([]rune(b.Text))
		}
	}

	start := time.Now()
	chunks := rag.ChunkBook(id, chapters, blocks, rag.DefaultOptions())
	chunkCost := time.Since(start)

	total, smallest, largest := 0, 0, 0
	for i, c := range chunks {
		total += c.CharCount
		if i == 0 || c.CharCount < smallest {
			smallest = c.CharCount
		}
		if c.CharCount > largest {
			largest = c.CharCount
		}
	}
	avg := 0
	if len(chunks) > 0 {
		avg = total / len(chunks)
	}

	fmt.Printf("正文: %d 章, %d 字\n", len(blocks), chars)
	fmt.Printf("分块: %d 块, 平均 %d 字, 最短 %d, 最长 %d, 耗时 %s\n",
		len(chunks), avg, smallest, largest, chunkCost.Round(time.Microsecond))

	buildStart := time.Now()
	index := rag.Build(chunks)
	buildCost := time.Since(buildStart)
	fmt.Printf("索引: %d 块, 建索引 %s\n", index.Len(), buildCost.Round(time.Microsecond))

	queryList := queries
	if len(queryList) == 0 {
		queryList = []string{"雨", "第一次", "他说"}
	}
	for _, q := range queryList {
		searchStart := time.Now()
		hits := index.Search(q, *limit)
		cost := time.Since(searchStart)
		fmt.Printf("\n查询 %q: %d 条命中, 耗时 %s\n", q, len(hits), cost.Round(time.Microsecond))
		for i, hit := range hits {
			c := hit.Chunk
			fmt.Printf("  [%d] 第%d章 %s  blocks %d..%d  %d字  score=%.3f\n",
				i+1, c.ChapterIdx, chapterLabel(c.ChapterTitle), c.BlockStart, c.BlockEnd, c.CharCount, hit.Score)
			fmt.Printf("      %s\n", excerptOf(c.Text, *excerpt))
		}
	}
}

// pickBook returns the book with the most cached chapters unless an id is given.
func pickBook(db *sql.DB, want string) (string, string, []weread.Chapter, error) {
	if want == "" {
		row := db.QueryRow(`SELECT book_id, COUNT(*) AS n FROM chapter_content
			GROUP BY book_id ORDER BY n DESC LIMIT 1`)
		if err := row.Scan(&want, new(int)); err != nil {
			return "", "", nil, fmt.Errorf("缓存库里还没有可索引的章节: %w", err)
		}
	}
	var title string
	if err := db.QueryRow(`SELECT COALESCE(title,'') FROM book WHERE id=?`, want).Scan(&title); err != nil && err != sql.ErrNoRows {
		return "", "", nil, err
	}
	if title == "" {
		title = want
	}

	rows, err := db.Query(`SELECT uid, idx, title, level, word_count, paid
		FROM chapter WHERE book_id=? ORDER BY idx ASC, uid ASC`, want)
	if err != nil {
		return "", "", nil, err
	}
	defer rows.Close()
	var chapters []weread.Chapter
	for rows.Next() {
		var (
			c    weread.Chapter
			paid int
		)
		if err := rows.Scan(&c.UID, &c.Index, &c.Title, &c.Level, &c.WordCount, &paid); err != nil {
			return "", "", nil, err
		}
		c.Paid = paid != 0
		chapters = append(chapters, c)
	}
	return want, title, chapters, rows.Err()
}

func loadBlocks(db *sql.DB, bookID string) (map[int64][]weread.Block, error) {
	rows, err := db.Query(`SELECT uid, blocks_json FROM chapter_content WHERE book_id=?`, bookID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[int64][]weread.Block{}
	for rows.Next() {
		var (
			uid  int64
			body string
		)
		if err := rows.Scan(&uid, &body); err != nil {
			return nil, err
		}
		var blocks []weread.Block
		if err := json.Unmarshal([]byte(body), &blocks); err != nil {
			continue
		}
		out[uid] = blocks
	}
	return out, rows.Err()
}

func chapterLabel(title string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return "(无标题)"
	}
	return title
}

func excerptOf(text string, limit int) string {
	clean := strings.Join(strings.Fields(text), " ")
	runes := []rune(clean)
	if limit > 0 && len(runes) > limit {
		return string(runes[:limit]) + "…"
	}
	return clean
}

// listCached prints the books that actually have chapter bodies cached. The
// probe is only as useful as the sample, and most books in a real library are
// cached partially or not at all.
func listCached(db *sql.DB) {
	rows, err := db.Query(`SELECT cc.book_id, COALESCE(b.title,''), COUNT(*) AS n,
			SUM(LENGTH(cc.blocks_json)) AS chars
		FROM chapter_content cc LEFT JOIN book b ON b.id=cc.book_id
		GROUP BY cc.book_id ORDER BY chars DESC LIMIT 15`)
	if err != nil {
		fail("统计缓存失败: %v", err)
	}
	defer rows.Close()
	fmt.Printf("%-14s %-8s %-10s %s\n", "BOOK", "CHAPTERS", "BYTES", "TITLE")
	for rows.Next() {
		var (
			id, title string
			n, bytes  int64
		)
		if err := rows.Scan(&id, &title, &n, &bytes); err != nil {
			fail("读取统计失败: %v", err)
		}
		fmt.Printf("%-14s %-8d %-10d %s\n", id, n, bytes, title)
	}
}

type flagList []string

func (f *flagList) String() string { return strings.Join(*f, ",") }

func (f *flagList) Set(v string) error {
	*f = append(*f, v)
	return nil
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
