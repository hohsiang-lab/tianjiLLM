package ollama

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/praxisllmlab/tianjiLLM/internal/model"
	"github.com/praxisllmlab/tianjiLLM/internal/provider"
	openaiProvider "github.com/praxisllmlab/tianjiLLM/internal/provider/openai"
)

const (
	defaultBaseURL   = "http://localhost:11434/v1"
	qwen3QueryPrefix = "Instruct: Given a web search query, retrieve relevant passages that answer the query\nQuery: "
)

// Provider adapts Ollama's OpenAI-compatible API while preserving
// embedding-model-specific request semantics.
type Provider struct{ *openaiProvider.Provider }

func init() {
	provider.Register("ollama", NewWithBaseURL(defaultBaseURL))
}

// NewWithBaseURL returns an Ollama provider targeting an OpenAI-compatible
// Ollama base URL, typically ending in /v1.
func NewWithBaseURL(baseURL string) *Provider {
	return &Provider{Provider: openaiProvider.NewWithBaseURL(baseURL)}
}

// WithBaseURL preserves Ollama-specific embedding handling when a deployment
// supplies a custom api_base.
func (p *Provider) WithBaseURL(baseURL string) provider.Provider {
	return NewWithBaseURL(baseURL)
}

// SupportsStandardOpenAIChatCapabilities keeps Ollama available to the
// OpenAI-compatible endpoint fallback even though its default base URL is
// not OpenAI's URL.
func (p *Provider) SupportsStandardOpenAIChatCapabilities() bool { return true }

// TransformEmbeddingRequest translates OpenClaw's input_type hint into the
// asymmetric retrieval format expected by Qwen3-Embedding:
//   - query: prepend Qwen3's retrieval instruction
//   - passage/document: keep the text unchanged
//
// Ollama's OpenAI-compatible endpoint does not consume input_type itself, so
// the field is removed before forwarding.
func (p *Provider) TransformEmbeddingRequest(ctx context.Context, req *model.EmbeddingRequest, apiKey string) (*http.Request, error) {
	copy := *req
	copy.ExtraParams = cloneExtraParams(req.ExtraParams)

	inputType, _ := copy.ExtraParams["input_type"].(string)
	delete(copy.ExtraParams, "input_type")

	if strings.EqualFold(strings.TrimSpace(inputType), "query") && isQwen3EmbeddingModel(copy.Model) {
		copy.Input = prefixQueryInput(copy.Input)
	}

	return p.Provider.TransformEmbeddingRequest(ctx, &copy, apiKey)
}

// TransformResponse preserves Ollama as the provider identity in upstream
// errors emitted by the embedded OpenAI-compatible implementation.
func (p *Provider) TransformResponse(ctx context.Context, resp *http.Response) (*model.ModelResponse, error) {
	result, err := p.Provider.TransformResponse(ctx, resp)
	return result, attributeOllamaError(err)
}

// TransformEmbeddingResponse preserves Ollama as the provider identity in
// embedding errors emitted by the embedded OpenAI-compatible implementation.
func (p *Provider) TransformEmbeddingResponse(ctx context.Context, resp *http.Response) (*model.EmbeddingResponse, error) {
	result, err := p.Provider.TransformEmbeddingResponse(ctx, resp)
	return result, attributeOllamaError(err)
}

func (p *Provider) GetSupportedParams() []string {
	params := append([]string(nil), p.Provider.GetSupportedParams()...)
	for _, param := range params {
		if param == "input_type" {
			return params
		}
	}
	return append(params, "input_type")
}

func attributeOllamaError(err error) error {
	if err == nil {
		return nil
	}

	var tianjiErr *model.TianjiError
	if errors.As(err, &tianjiErr) {
		copy := *tianjiErr
		copy.Provider = "ollama"
		return &copy
	}
	return err
}

func cloneExtraParams(extra map[string]any) map[string]any {
	if len(extra) == 0 {
		return nil
	}
	cloned := make(map[string]any, len(extra))
	for key, value := range extra {
		cloned[key] = value
	}
	return cloned
}

func isQwen3EmbeddingModel(modelName string) bool {
	return strings.Contains(strings.ToLower(modelName), "qwen3-embedding")
}

func prefixQueryInput(input any) any {
	switch value := input.(type) {
	case string:
		return prefixQuery(value)
	case []string:
		prefixed := make([]string, len(value))
		for i, text := range value {
			prefixed[i] = prefixQuery(text)
		}
		return prefixed
	case []any:
		prefixed := make([]any, len(value))
		for i, item := range value {
			if text, ok := item.(string); ok {
				prefixed[i] = prefixQuery(text)
			} else {
				prefixed[i] = item
			}
		}
		return prefixed
	default:
		return input
	}
}

func prefixQuery(text string) string {
	if strings.HasPrefix(text, qwen3QueryPrefix) {
		return text
	}
	return qwen3QueryPrefix + text
}
