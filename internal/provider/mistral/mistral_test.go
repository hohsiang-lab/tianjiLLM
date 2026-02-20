package mistral

import (
	"context"
	"io"
	"net/http"
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/model"
	"github.com/praxisllmlab/tianjiLLM/internal/provider"
	"github.com/praxisllmlab/tianjiLLM/internal/provider/openai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestProvider() *Provider {
	return &Provider{openai.NewWithBaseURL(defaultBaseURL)}
}

func TestProviderRegistered(t *testing.T) {
	p, err := provider.Get("mistral")
	require.NoError(t, err)
	assert.NotNil(t, p)
}

func TestGetRequestURL(t *testing.T) {
	p := newTestProvider()
	assert.Equal(t, "https://api.mistral.ai/v1/chat/completions", p.GetRequestURL("mistral-large-latest"))
}

func TestGetSupportedParams(t *testing.T) {
	p := newTestProvider()
	params := p.GetSupportedParams()
	assert.Contains(t, params, "parallel_tool_calls")
	assert.Contains(t, params, "seed")
}

func TestMapParams_MaxCompletionTokens(t *testing.T) {
	p := newTestProvider()
	result := p.MapParams(map[string]any{
		"max_completion_tokens": 1000,
		"temperature":           0.7,
	})
	assert.Equal(t, 1000, result["max_tokens"])
	assert.Equal(t, 0.7, result["temperature"])
	assert.NotContains(t, result, "max_completion_tokens")
}

func TestTransformRequest(t *testing.T) {
	p := newTestProvider()
	ctx := context.Background()

	req := &model.ChatCompletionRequest{
		Model:    "mistral-large-latest",
		Messages: []model.Message{{Role: "user", Content: "Hi"}},
	}

	httpReq, err := p.TransformRequest(ctx, req, "ms-key")
	require.NoError(t, err)

	assert.Equal(t, http.MethodPost, httpReq.Method)
	assert.Equal(t, "Bearer ms-key", httpReq.Header.Get("Authorization"))

	body, _ := io.ReadAll(httpReq.Body)
	assert.Contains(t, string(body), `"model":"mistral-large-latest"`)
}
