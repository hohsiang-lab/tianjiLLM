package integration

import (
	"context"
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/rag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type phase6MockEmbedder struct{}

func (m *phase6MockEmbedder) Embed(_ context.Context, texts []string, _ string) ([][]float32, error) {
	result := make([][]float32, len(texts))
	for i := range texts {
		result[i] = []float32{float32(i) * 0.1, 0.5, 0.2}
	}
	return result, nil
}

type phase6MockVectorStore struct {
	chunks []rag.VectorChunk
}

func (m *phase6MockVectorStore) Upsert(_ context.Context, _ string, chunks []rag.VectorChunk) error {
	m.chunks = append(m.chunks, chunks...)
	return nil
}

type phase6MockSearcher struct {
	chunks []rag.VectorChunk
}

func (m *phase6MockSearcher) Search(_ context.Context, _ string, _ []float32, topK int) ([]rag.VectorChunk, error) {
	if topK > len(m.chunks) {
		return m.chunks, nil
	}
	return m.chunks[:topK], nil
}

type phase6MockCompleter struct {
	response string
}

func (m *phase6MockCompleter) Complete(_ context.Context, _, _, _ string) (string, error) {
	return m.response, nil
}

func TestPhase6_RAG_IngestPipeline(t *testing.T) {
	store := &phase6MockVectorStore{}
	embedder := &phase6MockEmbedder{}
	pipeline := rag.NewIngestPipeline(embedder, store)

	content := "word1 word2 word3 word4 word5 word6 word7 word8 word9 word10 word11 word12"
	err := pipeline.Ingest(context.Background(), content, rag.IngestConfig{
		VectorStoreID:  "store-phase6",
		EmbeddingModel: "text-embedding-3-small",
		Chunking:       rag.ChunkConfig{ChunkSize: 5, ChunkOverlap: 2},
	})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(store.chunks), 2, "should create multiple chunks")
}

func TestPhase6_RAG_IngestEmptyContent(t *testing.T) {
	store := &phase6MockVectorStore{}
	embedder := &phase6MockEmbedder{}
	pipeline := rag.NewIngestPipeline(embedder, store)

	err := pipeline.Ingest(context.Background(), "", rag.IngestConfig{
		VectorStoreID:  "store-1",
		EmbeddingModel: "text-embedding-3-small",
	})
	assert.Error(t, err, "should error on empty content")
}

func TestPhase6_RAG_QueryPipeline(t *testing.T) {
	embedder := &phase6MockEmbedder{}
	searcher := &phase6MockSearcher{
		chunks: []rag.VectorChunk{
			{ID: "c1", Text: "Go is a programming language"},
			{ID: "c2", Text: "Go supports concurrency with goroutines"},
		},
	}
	completer := &phase6MockCompleter{response: "Go is a programming language that supports concurrency."}

	pipeline := rag.NewQueryPipeline(embedder, searcher, completer)

	result, err := pipeline.Query(context.Background(), "What is Go?", rag.QueryConfig{
		VectorStoreID:   "store-phase6",
		EmbeddingModel:  "text-embedding-3-small",
		CompletionModel: "gpt-4",
		TopK:            2,
	})
	require.NoError(t, err)
	assert.Contains(t, result.Answer, "Go")
	assert.Len(t, result.Sources, 2)
	assert.Equal(t, "gpt-4", result.Model)
}

func TestPhase6_RAG_QueryNoResults(t *testing.T) {
	embedder := &phase6MockEmbedder{}
	searcher := &phase6MockSearcher{chunks: nil}
	completer := &phase6MockCompleter{}

	pipeline := rag.NewQueryPipeline(embedder, searcher, completer)

	result, err := pipeline.Query(context.Background(), "What is nothing?", rag.QueryConfig{
		VectorStoreID:   "store-1",
		EmbeddingModel:  "text-embedding-3-small",
		CompletionModel: "gpt-4",
	})
	require.NoError(t, err)
	assert.Contains(t, result.Answer, "No relevant context")
}
