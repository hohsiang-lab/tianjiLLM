package jina

import (
	"context"
	"net/http"

	"github.com/praxisllmlab/tianjiLLM/internal/model"
	"github.com/praxisllmlab/tianjiLLM/internal/provider"
	openaiProvider "github.com/praxisllmlab/tianjiLLM/internal/provider/openai"
)

const defaultBaseURL = "https://api.jina.ai/v1"

type Provider struct{ *openaiProvider.Provider }

func init() {
	provider.Register("jina_ai", &Provider{openaiProvider.NewWithBaseURL(defaultBaseURL)})
}

// WithBaseURL implements provider.BaseURLConfigurable. It returns a new Jina
// provider pointing at the given base URL while preserving all Jina-specific
// embedding request normalization.
func (p *Provider) WithBaseURL(baseURL string) provider.Provider {
	return &Provider{openaiProvider.NewWithBaseURL(baseURL)}
}

// TransformEmbeddingRequest keeps direct provider callers from asking Jina for
// base64 strings, which cannot unmarshal into []float64. The public embeddings
// handler normally translates base64 requests to float before this hook. The
// original req is not mutated.
func (p *Provider) TransformEmbeddingRequest(ctx context.Context, req *model.EmbeddingRequest, apiKey string) (*http.Request, error) {
	if req.EncodingFormat == "base64" {
		// Shallow-copy and clear encoding_format so Jina returns float arrays.
		copy := *req
		copy.EncodingFormat = ""
		req = &copy
	}
	return p.Provider.TransformEmbeddingRequest(ctx, req, apiKey)
}

// TransformEmbeddingResponse normalizes Jina's valid total-only usage shape
// into the standard embedding usage fields Tianji exposes to clients.
func (p *Provider) TransformEmbeddingResponse(ctx context.Context, resp *http.Response) (*model.EmbeddingResponse, error) {
	return p.TransformEmbeddingResponseWithTotalOnlyUsage(ctx, resp)
}

func (p *Provider) GetSupportedParams() []string {
	return []string{
		"model", "input", "encoding_format", "dimensions",
		"task", "normalized", // Jina-specific embedding params
	}
}
