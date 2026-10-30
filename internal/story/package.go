package story

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// BranchPackageSchema versions the shared file. A reader that sees an unknown
// schema must refuse it instead of guessing field meaning.
const BranchPackageSchema = 1

// Import limits. A shared world line is untrusted input: these caps keep a
// hostile or broken file from exhausting memory or flooding the model context.
const (
	maxPackageBytes       = 2 << 20
	maxPackageScenes      = 200
	maxPackageSceneRunes  = 20000
	maxPackageMemoryRunes = 20000
)

// PackageAuthor carries provenance without requiring an account.
type PackageAuthor struct {
	Name string `json:"name"`
	Note string `json:"note"`
}

// BranchPackage is the portable form of a world line.
//
// It deliberately contains no original book text: only generated scenes and
// references (canon sequence numbers). Sharing a package therefore shares a
// reading experience, not a copy of the novel.
type BranchPackage struct {
	Schema    int           `json:"schema"`
	ID        string        `json:"id"`
	Book      BookRef       `json:"book"`
	BookKey   string        `json:"bookKey"`
	ForkSeq   int           `json:"forkSeq"`
	RejoinSeq int           `json:"rejoinSeq"`
	Title     string        `json:"title"`
	Status    string        `json:"status"`
	Summary   string        `json:"summary"`
	MemoryMD  string        `json:"memory"`
	State     State         `json:"state"`
	Scenes    []BranchScene `json:"scenes"`
	Author    PackageAuthor `json:"author"`
}

// BookKey is the portable identity of a book across accounts.
//
// A WeRead book id is account-scoped, so a shared package cannot rely on it.
// Title plus author is not perfect across editions, but it is stable enough to
// match the same work and cheap enough to compute without the text.
func BookKey(title, author string) string {
	normalise := func(s string) string {
		s = strings.ToLower(strings.TrimSpace(s))
		return strings.Join(strings.Fields(s), " ")
	}
	sum := sha256.Sum256([]byte(normalise(title) + "|" + normalise(author)))
	return "sha256:" + hex.EncodeToString(sum[:])
}

// BuildBranchPackage snapshots a branch for export.
func BuildBranchPackage(b Branch, scenes []BranchScene, author PackageAuthor) BranchPackage {
	pkg := BranchPackage{
		Schema:    BranchPackageSchema,
		ID:        b.ID,
		Book:      b.MemoryBook(),
		BookKey:   BookKey(b.MemoryBook().Title, b.MemoryBook().Author),
		ForkSeq:   b.ForkSeq,
		RejoinSeq: b.RejoinSeq,
		Title:     b.Title,
		Status:    b.Status,
		Summary:   b.Summary,
		MemoryMD:  b.MemoryMD,
		State:     *b.State.Clone(),
		Scenes:    append([]BranchScene(nil), scenes...),
		Author:    author,
	}
	pkg.State.Normalize()
	return pkg
}

// MemoryBook reads the book reference out of the constitution. The constitution
// is the authoritative record of what the branch was written against.
func (b Branch) MemoryBook() BookRef {
	if mem, err := ParseMemory(b.MemoryMD); err == nil {
		return mem.Book
	}
	return BookRef{}
}

// ParseBranchPackage validates an imported package.
//
// The checks are structural on purpose. They cannot tell whether a shared world
// line is well written, but they can stop the three failures that matter:
// a wrong schema, an oversized payload, and a malformed constitution.
func ParseBranchPackage(raw string) (*BranchPackage, error) {
	if len(raw) > maxPackageBytes {
		return nil, fmt.Errorf("story: 世界线包过大（%d 字节）", len(raw))
	}
	if strings.TrimSpace(raw) == "" {
		return nil, errors.New("story: 世界线包为空")
	}

	var pkg BranchPackage
	if err := json.Unmarshal([]byte(raw), &pkg); err != nil {
		return nil, fmt.Errorf("story: 世界线包解析失败: %w", err)
	}
	if pkg.Schema != BranchPackageSchema {
		return nil, fmt.Errorf("story: 不支持的世界线包版本 %d", pkg.Schema)
	}
	if strings.TrimSpace(pkg.BookKey) == "" {
		return nil, errors.New("story: 世界线包缺少 bookKey")
	}
	if len([]rune(pkg.MemoryMD)) > maxPackageMemoryRunes {
		return nil, errors.New("story: 世界线宪法过长")
	}
	if _, err := ParseMemory(pkg.MemoryMD); err != nil {
		return nil, fmt.Errorf("story: 世界线宪法无效: %w", err)
	}
	if len(pkg.Scenes) > maxPackageScenes {
		return nil, fmt.Errorf("story: 世界线包含 %d 幕，超过上限", len(pkg.Scenes))
	}
	for i, sc := range pkg.Scenes {
		if len([]rune(sc.Text)) > maxPackageSceneRunes {
			return nil, fmt.Errorf("story: 第 %d 幕过长", i+1)
		}
		if strings.TrimSpace(sc.Text) == "" {
			return nil, fmt.Errorf("story: 第 %d 幕为空", i+1)
		}
	}

	pkg.State.Normalize()
	pkg.Title = strings.TrimSpace(pkg.Title)
	pkg.Summary = strings.TrimSpace(pkg.Summary)
	pkg.Author.Name = strings.TrimSpace(pkg.Author.Name)
	pkg.Author.Note = strings.TrimSpace(pkg.Author.Note)
	return &pkg, nil
}

// MatchesBook reports whether the package was written against this book.
func (p *BranchPackage) MatchesBook(title, author string) bool {
	return p != nil && p.BookKey == BookKey(title, author)
}
