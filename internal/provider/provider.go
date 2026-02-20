package provider

import (
	"context"
	"net/http"

	"github.com/praxisllmlab/tianjiLLM/internal/model"
)

// Provider defines the interface that all LLM providers must implement.
// This mirrors Python LiteLLM's BaseConfig pattern.
type Provider interface {
	// TransformRequest converts an OpenAI-compatible request into
	// a provider-native HTTP request.
	TransformRequest(ctx context.Context, req *model.ChatCompletionRequest, apiKey string) (*http.Request, error)

	// TransformResponse converts a provider-native HTTP response
	// into an OpenAI-compatible ModelResponse.
	TransformResponse(ctx context.Context, resp *http.Response) (*model.ModelResponse, error)

	// TransformStreamChunk converts a single SSE data line from
	// the provider into an OpenAI-compatible StreamChunk.
	TransformStreamChunk(ctx context.Context, data []byte) (*model.StreamChunk, bool, error)

	// GetSupportedParams returns the list of parameter names this
	// provider supports.
	GetSupportedParams() []string

	// MapParams transforms OpenAI parameter names to provider-native
	// parameter names (e.g., max_completion_tokens → max_tokens).
	MapParams(params map[string]any) map[string]any

	// GetRequestURL returns the full URL for the given model.
	GetRequestURL(model string) string

	// SetupHeaders sets provider-specific headers on the request.
	SetupHeaders(req *http.Request, apiKey string)
}

// EmbeddingProvider extends Provider with embedding support.
type EmbeddingProvider interface {
	TransformEmbeddingRequest(ctx context.Context, req *model.EmbeddingRequest, apiKey string) (*http.Request, error)
	TransformEmbeddingResponse(ctx context.Context, resp *http.Response) (*model.EmbeddingResponse, error)
}
