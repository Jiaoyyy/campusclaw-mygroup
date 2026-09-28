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
