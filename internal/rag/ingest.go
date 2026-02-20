package rag

import (
	"context"
	"fmt"
	"strings"
)

// ChunkConfig controls document chunking behavior.
type ChunkConfig struct {
	ChunkSize    int `json:"chunk_size"`
	ChunkOverlap int `json:"chunk_overlap"`
}

// DefaultChunkConfig returns sensible defaults.
func DefaultChunkConfig() ChunkConfig {
	return ChunkConfig{ChunkSize: 1000, ChunkOverlap: 200}
}

// IngestConfig configures the ingestion pipeline.
type IngestConfig struct {
	VectorStoreID  string      `json:"vector_store_id"`
	EmbeddingModel string      `json:"embedding_model"`
	Chunking       ChunkConfig `json:"chunking"`
}

// IngestPipeline processes documents: parse → chunk → embed → store.
type IngestPipeline struct {
	embedder Embedder
	vectorDB VectorStore
}

// Embedder generates embeddings for text.
type Embedder interface {
	Embed(ctx context.Context, texts []string, model string) ([][]float32, error)
}

// VectorStore persists embedded chunks.
type VectorStore interface {
	Upsert(ctx context.Context, storeID string, chunks []VectorChunk) error
}

// VectorChunk represents a text chunk with its embedding.
type VectorChunk struct {
	ID        string         `json:"id"`
	Text      string         `json:"text"`
	Embedding []float32      `json:"embedding"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

// NewIngestPipeline creates an ingest pipeline.
func NewIngestPipeline(embedder Embedder, vectorDB VectorStore) *IngestPipeline {
	return &IngestPipeline{embedder: embedder, vectorDB: vectorDB}
}

// Ingest processes a document through the pipeline.
func (p *IngestPipeline) Ingest(ctx context.Context, content string, cfg IngestConfig) error {
	if cfg.Chunking.ChunkSize == 0 {
		cfg.Chunking = DefaultChunkConfig()
	}

	chunks := chunkText(content, cfg.Chunking.ChunkSize, cfg.Chunking.ChunkOverlap)
	if len(chunks) == 0 {
		return fmt.Errorf("no content to ingest")
	}

	embeddings, err := p.embedder.Embed(ctx, chunks, cfg.EmbeddingModel)
	if err != nil {
		return fmt.Errorf("embed chunks: %w", err)
	}

	vectorChunks := make([]VectorChunk, len(chunks))
	for i, text := range chunks {
		vectorChunks[i] = VectorChunk{
			ID:        fmt.Sprintf("chunk-%d", i),
			Text:      text,
			Embedding: embeddings[i],
		}
	}

	return p.vectorDB.Upsert(ctx, cfg.VectorStoreID, vectorChunks)
}

// chunkText splits text into overlapping chunks.
func chunkText(text string, size, overlap int) []string {
	if size <= 0 {
		size = 1000
	}
	if overlap >= size {
		overlap = size / 5
	}

	words := strings.Fields(text)
	if len(words) == 0 {
		return nil
	}

	var chunks []string
	for i := 0; i < len(words); {
		end := i + size
		if end > len(words) {
			end = len(words)
		}
		chunks = append(chunks, strings.Join(words[i:end], " "))
		i += size - overlap
		if i >= len(words) {
			break
		}
	}
	return chunks
}
