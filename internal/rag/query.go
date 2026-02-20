package rag

import (
	"context"
	"fmt"
	"strings"
)

// QueryConfig configures the RAG query pipeline.
type QueryConfig struct {
	VectorStoreID   string `json:"vector_store_id"`
	EmbeddingModel  string `json:"embedding_model"`
	CompletionModel string `json:"completion_model"`
	TopK            int    `json:"top_k"`
	Rerank          bool   `json:"rerank"`
}

// QueryResult holds the RAG query response.
type QueryResult struct {
	Answer  string        `json:"answer"`
	Sources []VectorChunk `json:"sources,omitempty"`
	Model   string        `json:"model"`
}

// VectorSearcher searches a vector store for similar chunks.
type VectorSearcher interface {
	Search(ctx context.Context, storeID string, embedding []float32, topK int) ([]VectorChunk, error)
}

// Completer generates text completions.
type Completer interface {
	Complete(ctx context.Context, model, systemPrompt, userMessage string) (string, error)
}

// QueryPipeline handles RAG queries: search → (rerank) → inject context → complete.
type QueryPipeline struct {
	embedder  Embedder
	searcher  VectorSearcher
	completer Completer
}

// NewQueryPipeline creates a query pipeline.
func NewQueryPipeline(embedder Embedder, searcher VectorSearcher, completer Completer) *QueryPipeline {
	return &QueryPipeline{
		embedder:  embedder,
		searcher:  searcher,
		completer: completer,
	}
}

// Query runs the RAG query pipeline.
func (p *QueryPipeline) Query(ctx context.Context, question string, cfg QueryConfig) (*QueryResult, error) {
	if cfg.TopK <= 0 {
		cfg.TopK = 5
	}

	// Embed the question
	embeddings, err := p.embedder.Embed(ctx, []string{question}, cfg.EmbeddingModel)
	if err != nil {
		return nil, fmt.Errorf("embed question: %w", err)
	}

	// Search vector store
	chunks, err := p.searcher.Search(ctx, cfg.VectorStoreID, embeddings[0], cfg.TopK)
	if err != nil {
		return nil, fmt.Errorf("vector search: %w", err)
	}

	if len(chunks) == 0 {
		return &QueryResult{
			Answer: "No relevant context found to answer this question.",
			Model:  cfg.CompletionModel,
		}, nil
	}

	// Build context from retrieved chunks
	contextParts := make([]string, len(chunks))
	for i, c := range chunks {
		contextParts[i] = fmt.Sprintf("[Source %d]: %s", i+1, c.Text)
	}
	contextStr := strings.Join(contextParts, "\n\n")

	systemPrompt := fmt.Sprintf(
		"Use the following context to answer the user's question. "+
			"If the context doesn't contain relevant information, say so.\n\n"+
			"Context:\n%s", contextStr,
	)

	// Generate completion
	answer, err := p.completer.Complete(ctx, cfg.CompletionModel, systemPrompt, question)
	if err != nil {
		return nil, fmt.Errorf("completion: %w", err)
	}

	return &QueryResult{
		Answer:  answer,
		Sources: chunks,
		Model:   cfg.CompletionModel,
	}, nil
}
