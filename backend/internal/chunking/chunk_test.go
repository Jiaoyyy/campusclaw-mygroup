package chunking

import (
	"strings"
	"testing"
)

func TestAutoWindowsAndOffsets(t *testing.T) {
	body := strings.Repeat("中文", 450)
	chunks, processed, err := Split(body, Options{})
	if err != nil || processed != body || len(chunks) != 2 {
		t.Fatalf("split: chunks=%d err=%v", len(chunks), err)
	}
	if chunks[0].Start != 0 || chunks[0].End != 800 || chunks[1].Start != 720 || chunks[1].End != 900 {
		t.Fatalf("unexpected offsets: %+v", chunks)
	}
	for _, chunk := range chunks {
		if chunk.Text != string([]rune(processed)[chunk.Start:chunk.End]) {
			t.Fatalf("chunk %d does not match source", chunk.Index)
		}
	}
}

func TestHierarchyKeepsHeadings(t *testing.T) {
	chunks, _, err := Split("前言\n# 一\n内容\n## 二\n更多", Options{Strategy: "hierarchy"})
	if err != nil || len(chunks) != 3 || !strings.HasPrefix(chunks[1].Text, "# 一") || !strings.HasPrefix(chunks[2].Text, "## 二") {
		t.Fatalf("hierarchy: %+v %v", chunks, err)
	}
}

func TestCustomPreprocessingDoesNotChangeOriginal(t *testing.T) {
	original := "联系 test@example.com。\nhttps://example.com  认证"
	chunks, processed, err := Split(original, Options{Strategy: "custom", MaxChars: 100, Separator: "period", RemoveURLs: true, RemoveEmails: true, CollapseWhitespace: true})
	if err != nil || strings.Contains(processed, "example.com") || !strings.Contains(original, "example.com") || len(chunks) == 0 {
		t.Fatalf("custom: %q %+v %v", processed, chunks, err)
	}
}
