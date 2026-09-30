package retrieval

import "testing"

func TestRRFRewardsBothPaths(t *testing.T) {
	hits := fuse([]Hit{{ChunkID: 1}, {ChunkID: 2}}, []Hit{{ChunkID: 2}, {ChunkID: 3}}, 3)
	if len(hits) != 3 || hits[0].ChunkID != 2 || hits[0].Score <= hits[1].Score {
		t.Fatalf("unexpected fusion: %+v", hits)
	}
}

func TestExcerptUsesUnicodeCharacters(t *testing.T) {
	if excerpt("中文") != "中文" || !validQuery("中文") || validQuery("") {
		t.Fatal("Unicode query or excerpt failed")
	}
}

func TestCitedHitsExcludeUnusedSourcesAndKeepNumbers(t *testing.T) {
	hits := []Hit{{ChunkID: 1}, {ChunkID: 2}, {ChunkID: 3}}
	citations := citedHits("见 [3] 和 [1]，仍见 [3]。", hits)
	if len(citations) != 2 || citations[0].ChunkID != 1 || citations[0].CitationNumber != 1 || citations[1].ChunkID != 3 || citations[1].CitationNumber != 3 {
		t.Fatalf("incorrect citations: %+v", citations)
	}
}
