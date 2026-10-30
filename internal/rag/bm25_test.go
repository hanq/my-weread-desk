package rag

import (
	"reflect"
	"testing"
)

func TestTokenizeBigrams(t *testing.T) {
	got := Tokenize("林黛玉")
	// Unigrams make a one-character query match; bigrams add phrase precision.
	want := []string{"林", "黛", "玉", "林黛", "黛玉"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestSingleCharQueryMatches(t *testing.T) {
	// Regression: with bigrams only, a one-character query such as "雨" never
	// appeared as a token and returned nothing.
	idx := Build([]Chunk{
		{Seq: 0, Text: "雨下得很大，街上没有人。"},
		{Seq: 1, Text: "他把窗户关上，走回屋里。"},
	})
	hits := idx.Search("雨", 5)
	if len(hits) == 0 {
		t.Fatal("single-character query returned no hits")
	}
	if hits[0].Chunk.Seq != 0 {
		t.Fatalf("wrong hit: seq %d", hits[0].Chunk.Seq)
	}
}

func TestTokenizeKeepsLatinWordsAndDropsPunctuation(t *testing.T) {
	got := Tokenize("Go 1.20 与 SQLite")
	want := []string{"go", "1", "20", "与", "sqlite"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestTokenizeSingleCJKChar(t *testing.T) {
	if got := Tokenize("与"); !reflect.DeepEqual(got, []string{"与"}) {
		t.Fatalf("single CJK char must stay a token, got %v", got)
	}
}

func TestIndexRanksTheMatchingChunk(t *testing.T) {
	idx := Build([]Chunk{
		{Seq: 0, Text: "林黛玉初进荣国府，步步留心，时时在意。"},
		{Seq: 1, Text: "宋江怒杀阎婆惜，连夜投奔梁山。"},
		{Seq: 2, Text: "荣国府里张灯结彩，阖府上下都忙了起来。"},
	})
	hits := idx.Search("林黛玉", 3)
	if len(hits) == 0 {
		t.Fatal("no hits")
	}
	if hits[0].Chunk.Seq != 0 {
		t.Fatalf("wrong top hit: seq %d", hits[0].Chunk.Seq)
	}
	if hits[0].Score <= 0 {
		t.Fatalf("score must be positive, got %f", hits[0].Score)
	}
}

func TestSearchIgnoresEmptyQuery(t *testing.T) {
	idx := Build([]Chunk{{Seq: 0, Text: "正文"}})
	if hits := idx.Search("   ", 5); hits != nil {
		t.Fatalf("expected nil for a blank query, got %v", hits)
	}
}

func TestSearchRejectsNonMatchingCorpus(t *testing.T) {
	idx := Build([]Chunk{{Seq: 0, Text: "宋江怒杀阎婆惜"}})
	if hits := idx.Search("林黛玉", 5); len(hits) != 0 {
		t.Fatalf("expected no hits, got %d", len(hits))
	}
}

func TestSearchHonoursLimit(t *testing.T) {
	chunks := []Chunk{
		{Seq: 0, Text: "雨"},
		{Seq: 1, Text: "雨"},
		{Seq: 2, Text: "雨"},
	}
	idx := Build(chunks)
	if hits := idx.Search("雨", 2); len(hits) != 2 {
		t.Fatalf("expected 2 hits, got %d", len(hits))
	}
}

func TestExpandReturnsNeighbours(t *testing.T) {
	idx := Build([]Chunk{{Seq: 0, Text: "一"}, {Seq: 1, Text: "二"}, {Seq: 2, Text: "三"}})
	expanded := idx.Expand(Hit{Chunk: Chunk{Seq: 1}}, 1, 1)
	if len(expanded) != 3 {
		t.Fatalf("expected 3 chunks, got %d", len(expanded))
	}
	if expanded[0].Seq != 0 || expanded[2].Seq != 2 {
		t.Fatalf("neighbour order wrong: %d..%d", expanded[0].Seq, expanded[2].Seq)
	}
}
