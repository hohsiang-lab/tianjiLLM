package contract

import (
	"context"
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/rag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockEmbedder struct {
	embedFn func(ctx context.Context, texts []string, model string) ([][]float32, error)
}

func (m *mockEmbedder) Embed(ctx context.Context, texts []string, model string) ([][]float32, error) {
	return m.embedFn(ctx, texts, model)
}

type mockVectorStore struct {
	upserted []rag.VectorChunk
}

func (m *mockVectorStore) Upsert(_ context.Context, _ string, chunks []rag.VectorChunk) error {
	m.upserted = append(m.upserted, chunks...)
	return nil
}

type mockVectorSearcher struct {
	chunks []rag.VectorChunk
}

func (m *mockVectorSearcher) Search(_ context.Context, _ string, _ []float32, topK int) ([]rag.VectorChunk, error) {
	if topK > len(m.chunks) {
		return m.chunks, nil
	}
	return m.chunks[:topK], nil
}

type mockCompleter struct {
	response string
}

func (m *mockCompleter) Complete(_ context.Context, _, _, _ string) (string, error) {
	return m.response, nil
}

func TestIngestPipeline_ChunksAndStores(t *testing.T) {
	store := &mockVectorStore{}
	embedder := &mockEmbedder{
		embedFn: func(_ context.Context, texts []string, _ string) ([][]float32, error) {
			result := make([][]float32, len(texts))
			for i := range texts {
				result[i] = []float32{float32(i), 0.1, 0.2}
			}
			return result, nil
		},
	}

	pipeline := rag.NewIngestPipeline(embedder, store)

	content := "word1 word2 word3 word4 word5 word6 word7 word8 word9 word10"
	err := pipeline.Ingest(context.Background(), content, rag.IngestConfig{
		VectorStoreID:  "store-1",
		EmbeddingModel: "text-embedding-3-small",
		Chunking:       rag.ChunkConfig{ChunkSize: 5, ChunkOverlap: 2},
	})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(store.upserted), 2, "should create multiple chunks")
}

func TestIngestPipeline_EmptyContent(t *testing.T) {
	store := &mockVectorStore{}
	embedder := &mockEmbedder{}

	pipeline := rag.NewIngestPipeline(embedder, store)

	err := pipeline.Ingest(context.Background(), "", rag.IngestConfig{
		VectorStoreID:  "store-1",
		EmbeddingModel: "text-embedding-3-small",
	})
	assert.Error(t, err, "should error on empty content")
}

func TestQueryPipeline_ReturnsAnswer(t *testing.T) {
	embedder := &mockEmbedder{
		embedFn: func(_ context.Context, texts []string, _ string) ([][]float32, error) {
			result := make([][]float32, len(texts))
			for i := range texts {
				result[i] = []float32{1.0, 0.0}
			}
			return result, nil
		},
	}
	searcher := &mockVectorSearcher{
		chunks: []rag.VectorChunk{
			{ID: "c1", Text: "Go is a programming language"},
			{ID: "c2", Text: "Go was created by Google"},
		},
	}
	completer := &mockCompleter{response: "Go is a programming language created by Google."}

	pipeline := rag.NewQueryPipeline(embedder, searcher, completer)

	result, err := pipeline.Query(context.Background(), "What is Go?", rag.QueryConfig{
		VectorStoreID:   "store-1",
		EmbeddingModel:  "text-embedding-3-small",
		CompletionModel: "gpt-4",
		TopK:            2,
	})
	require.NoError(t, err)
	assert.Contains(t, result.Answer, "Go")
	assert.Len(t, result.Sources, 2)
	assert.Equal(t, "gpt-4", result.Model)
}

func TestQueryPipeline_NoResults(t *testing.T) {
	embedder := &mockEmbedder{
		embedFn: func(_ context.Context, texts []string, _ string) ([][]float32, error) {
			result := make([][]float32, len(texts))
			for i := range texts {
				result[i] = []float32{0.0}
			}
			return result, nil
		},
	}
	searcher := &mockVectorSearcher{chunks: nil}
	completer := &mockCompleter{}

	pipeline := rag.NewQueryPipeline(embedder, searcher, completer)

	result, err := pipeline.Query(context.Background(), "What is nothing?", rag.QueryConfig{
		VectorStoreID:   "store-1",
		EmbeddingModel:  "text-embedding-3-small",
		CompletionModel: "gpt-4",
	})
	require.NoError(t, err)
	assert.Contains(t, result.Answer, "No relevant context")
}
