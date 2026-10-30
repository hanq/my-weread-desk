package rag

import (
	"math"
	"sort"
	"strings"
	"unicode"
)

// Tokenize produces retrieval terms for mixed Chinese/Latin text.
//
// Chinese is written without spaces, so every CJK run is expanded into single
// characters and character bigrams: "林黛玉" becomes 林, 黛, 玉, 林黛, 黛玉.
// Bigrams carry the phrase precision; unigrams are what make a one-character
// query such as "雨" able to match at all, which bigrams alone cannot do. The
// extra terms roughly double the index, which is still negligible for a book.
// Latin words and numbers stay whole and are lowercased.
func Tokenize(s string) []string {
	tokens := make([]string, 0, len(s))
	var cjk []rune
	var latin []rune

	flushCJK := func() {
		if len(cjk) == 0 {
			return
		}
		for _, r := range cjk {
			tokens = append(tokens, string(r))
		}
		for i := 0; i+1 < len(cjk); i++ {
			tokens = append(tokens, string(cjk[i:i+2]))
		}
		cjk = cjk[:0]
	}
	flushLatin := func() {
		if len(latin) == 0 {
			return
		}
		tokens = append(tokens, strings.ToLower(string(latin)))
		latin = latin[:0]
	}

	for _, r := range s {
		switch {
		case isCJK(r):
			flushLatin()
			cjk = append(cjk, r)
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			flushCJK()
			latin = append(latin, r)
		default:
			flushCJK()
			flushLatin()
		}
	}
	flushCJK()
	flushLatin()
	return tokens
}

func isCJK(r rune) bool {
	return unicode.In(r, unicode.Han, unicode.Hiragana, unicode.Katakana, unicode.Hangul)
}

// Doc is one indexed chunk.
type Doc struct {
	Chunk Chunk
	Len   int
}

type posting struct {
	doc int
	tf  int
}

// Index is an in-memory BM25 index. A book is small enough that scoring every
// document per query is faster than maintaining a traversal structure, and it
// keeps the index dependency-free.
type Index struct {
	docs   []Doc
	seqAt  map[int]int
	df     map[string]int
	post   map[string][]posting
	avgLen float64
}

const (
	bm25K1 = 1.2
	bm25B  = 0.75
)

// Build indexes chunks in the order given. Seq is preserved on the Chunk so
// callers can expand a hit to its neighbours.
func Build(chunks []Chunk) *Index {
	idx := &Index{
		docs:  make([]Doc, len(chunks)),
		seqAt: make(map[int]int, len(chunks)),
		df:    map[string]int{},
		post:  map[string][]posting{},
	}
	total := 0
	for i, c := range chunks {
		tokens := Tokenize(c.Text)
		idx.docs[i] = Doc{Chunk: c, Len: len(tokens)}
		idx.seqAt[c.Seq] = i
		total += len(tokens)

		counts := map[string]int{}
		for _, t := range tokens {
			counts[t]++
		}
		for term, tf := range counts {
			idx.df[term]++
			idx.post[term] = append(idx.post[term], posting{doc: i, tf: tf})
		}
	}
	if len(chunks) > 0 {
		idx.avgLen = float64(total) / float64(len(chunks))
	}
	return idx
}

// Len reports how many chunks are indexed.
func (i *Index) Len() int { return len(i.docs) }

// Chunk returns the chunk at an index position.
func (i *Index) Chunk(at int) (Chunk, bool) {
	if at < 0 || at >= len(i.docs) {
		return Chunk{}, false
	}
	return i.docs[at].Chunk, true
}

// BySeq returns the chunk with the given sequence number. Callers use it to
// pull the neighbour that continues a hit, without knowing storage order.
func (i *Index) BySeq(seq int) (Chunk, bool) {
	at, ok := i.seqAt[seq]
	if !ok {
		return Chunk{}, false
	}
	return i.docs[at].Chunk, true
}

// Hit is one ranked chunk.
type Hit struct {
	Chunk Chunk   `json:"chunk"`
	Score float64 `json:"score"`
}

// Search ranks the index against a query. Repeated query terms are counted
// once: bigram expansion already produces overlapping terms, and counting
// duplicates would over-weight whichever phrase happened to repeat.
func (i *Index) Search(query string, limit int) []Hit {
	terms := Tokenize(query)
	if len(terms) == 0 || len(i.docs) == 0 {
		return nil
	}
	if limit <= 0 || limit > len(i.docs) {
		limit = len(i.docs)
	}

	scores := make([]float64, len(i.docs))
	seen := make(map[string]bool, len(terms))
	n := float64(len(i.docs))
	for _, term := range terms {
		if seen[term] {
			continue
		}
		seen[term] = true

		plist := i.post[term]
		if len(plist) == 0 {
			continue
		}
		df := float64(i.df[term])
		idf := math.Log(1 + (n-df+0.5)/(df+0.5))
		for _, p := range plist {
			dl := float64(i.docs[p.doc].Len)
			norm := 1.0
			if i.avgLen > 0 {
				norm = 1 - bm25B + bm25B*dl/i.avgLen
			}
			denom := float64(p.tf) + bm25K1*norm
			if denom <= 0 {
				continue
			}
			scores[p.doc] += idf * float64(p.tf) * (bm25K1 + 1) / denom
		}
	}

	hits := make([]Hit, 0, limit)
	for at, score := range scores {
		if score <= 0 {
			continue
		}
		hits = append(hits, Hit{Chunk: i.docs[at].Chunk, Score: score})
	}
	sort.SliceStable(hits, func(a, b int) bool {
		if hits[a].Score != hits[b].Score {
			return hits[a].Score > hits[b].Score
		}
		// Equal scores prefer the earlier chunk so a stable result set can be
		// relied on by the UI and by tests.
		return hits[a].Chunk.Seq < hits[b].Chunk.Seq
	})
	if len(hits) > limit {
		hits = hits[:limit]
	}
	return hits
}

// Expand widens a hit by one chunk on either side. A retrieved paragraph often
// depends on the sentence just before it, and the neighbour chunks are already
// loaded, so continuity costs nothing but a slice read.
func (i *Index) Expand(hit Hit, before, after int) []Chunk {
	target := hit.Chunk.Seq
	var out []Chunk
	for at := range i.docs {
		seq := i.docs[at].Chunk.Seq
		if seq < target-before || seq > target+after {
			continue
		}
		out = append(out, i.docs[at].Chunk)
	}
	if len(out) == 0 {
		return []Chunk{hit.Chunk}
	}
	return out
}
