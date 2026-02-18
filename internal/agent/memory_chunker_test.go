package agent

import "testing"

func TestChunkMarkdownSplitsByHeading(t *testing.T) {
	doc := "# Intro\nHello world\n## Details\nSome details here"
	chunks := chunkMarkdown(doc, 512)
	if len(chunks) < 2 {
		t.Fatalf("expected at least 2 chunks, got %d", len(chunks))
	}
	if chunks[0].Heading != "Intro" {
		t.Errorf("expected heading 'Intro', got %q", chunks[0].Heading)
	}
	if chunks[1].Heading != "Details" {
		t.Errorf("expected heading 'Details', got %q", chunks[1].Heading)
	}
}

func TestChunkMarkdownLargeBlockSplits(t *testing.T) {
	var long string
	for i := 0; i < 200; i++ {
		long += "This is a line of text that should be long enough to force splitting.\n"
	}
	chunks := chunkMarkdown(long, 50)
	if len(chunks) < 2 {
		t.Fatalf("expected multiple chunks for large text, got %d", len(chunks))
	}
	for _, c := range chunks {
		if estimateTokens(c.Content) > 60 {
			t.Errorf("chunk %d too large: %d estimated tokens", c.Index, estimateTokens(c.Content))
		}
	}
}

func TestChunkMarkdownEmptyInput(t *testing.T) {
	chunks := chunkMarkdown("", 512)
	if len(chunks) != 0 {
		t.Errorf("expected 0 chunks for empty input, got %d", len(chunks))
	}
}

func TestChunkMarkdownDefaultMaxTokens(t *testing.T) {
	doc := "# Title\nSome content"
	chunks := chunkMarkdown(doc, 0)
	if len(chunks) == 0 {
		t.Error("expected at least 1 chunk with maxTokens=0 (should default)")
	}
}
