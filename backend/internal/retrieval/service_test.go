package retrieval

import (
	"strings"
	"testing"
)

func TestFindHitsKeepsUnicodeSourceOffsets(t *testing.T) {
	body := "甲乙\n学习认证与授权。\n认证只限本班。"
	hits := findHits(body, "认证", 7, "A班讲义.md", 20)
	if len(hits) != 2 {
		t.Fatalf("want two hits, got %+v", hits)
	}
	if hits[0].Start != 5 || hits[0].End != 7 || hits[0].ChunkIndex != 1 || hits[0].Snippet != body {
		t.Fatalf("first hit has incorrect source position: %+v", hits[0])
	}
	if hits[1].Start != 12 || hits[1].End != 14 || hits[1].MaterialID != 7 {
		t.Fatalf("second hit has incorrect source position: %+v", hits[1])
	}
}

func TestFindHitsMatchesLiteralKeywordAndLimit(t *testing.T) {
	if hits := findHits("100% done, 100% checked", "%", 1, "literal.txt", 1); len(hits) != 1 || hits[0].Start != 3 {
		t.Fatalf("literal percent or limit failed: %+v", hits)
	}
	if hits := findHits("Go go", "GO", 1, "case.txt", 20); len(hits) != 2 {
		t.Fatalf("case-insensitive search failed: %+v", hits)
	}
	if hits := findHits(strings.Repeat("a ", 50), "a", 1, "many.txt", 20); len(hits) != 20 {
		t.Fatalf("result cap failed: got %d hits", len(hits))
	}
	if hits := findHits(strings.Repeat("甲", 401)+"检索", "检索", 1, "long.txt", 20); len(hits) != 1 || hits[0].ChunkIndex != 2 || hits[0].Start != 401 {
		t.Fatalf("chunk index failed: %+v", hits)
	}
}
