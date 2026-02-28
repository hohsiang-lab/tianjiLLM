package rag

import (
	"testing"
)

func TestDefaultChunkConfig(t *testing.T) {
	cfg := DefaultChunkConfig()
	if cfg.ChunkSize <= 0 {
		t.Fatal("ChunkSize should be > 0")
	}
	if cfg.ChunkOverlap < 0 {
		t.Fatal("ChunkOverlap should be >= 0")
	}
}

func TestChunkTextEmpty(t *testing.T) {
	chunks := chunkText("", 100, 20)
	if chunks != nil {
		t.Fatalf("expected nil, got %v", chunks)
	}
}

func TestChunkTextSmall(t *testing.T) {
	chunks := chunkText("hello world", 100, 20)
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}
	if chunks[0] != "hello world" {
		t.Fatalf("unexpected chunk: %q", chunks[0])
	}
}

func TestChunkTextMultiple(t *testing.T) {
	// 10 words, chunk size 3, overlap 1 → should produce multiple chunks
	text := "one two three four five six seven eight nine ten"
	chunks := chunkText(text, 3, 1)
	if len(chunks) < 3 {
		t.Fatalf("expected >= 3 chunks, got %d", len(chunks))
	}
}

func TestChunkTextOverlapExceedsSize(t *testing.T) {
	chunks := chunkText("a b c d e", 2, 10)
	if len(chunks) == 0 {
		t.Fatal("expected chunks")
	}
}

func TestChunkTextZeroSize(t *testing.T) {
	chunks := chunkText("a b c", 0, 0)
	if len(chunks) == 0 {
		t.Fatal("expected chunks with zero size defaulting")
	}
}

func TestNewIngestPipeline(t *testing.T) {
	p := NewIngestPipeline(nil, nil)
	if p == nil {
		t.Fatal("nil pipeline")
	}
}

func TestNewQueryPipeline(t *testing.T) {
	p := NewQueryPipeline(nil, nil, nil)
	if p == nil {
		t.Fatal("nil pipeline")
	}
}
