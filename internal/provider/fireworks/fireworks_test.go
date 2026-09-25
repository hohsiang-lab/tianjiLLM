package fireworks

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
	p, err := provider.Get("fireworks_ai")
	require.NoError(t, err)
	assert.NotNil(t, p)
}

func TestGetRequestURL(t *testing.T) {
	p := newTestProvider()
	assert.Equal(t, "https://api.fireworks.ai/inference/v1/chat/completions", p.GetRequestURL("llama-v3-70b"))
}

func TestGetSupportedParams(t *testing.T) {
	p := newTestProvider()
	params := p.GetSupportedParams()
	assert.Contains(t, params, "model")
	assert.Contains(t, params, "top_k")
	assert.Contains(t, params, "tools")
}

func TestTransformRequest(t *testing.T) {
	p := newTestProvider()
	ctx := context.Background()

	req := &model.ChatCompletionRequest{
		Model:    "llama-v3-70b",
		Messages: []model.Message{{Role: "user", Content: "Hi"}},
	}

	httpReq, err := p.TransformRequest(ctx, req, "fw-key")
	require.NoError(t, err)

	assert.Equal(t, http.MethodPost, httpReq.Method)
	assert.Equal(t, "https://api.fireworks.ai/inference/v1/chat/completions", httpReq.URL.String())
	assert.Equal(t, "Bearer fw-key", httpReq.Header.Get("Authorization"))

	body, _ := io.ReadAll(httpReq.Body)
	assert.Contains(t, string(body), `"model":"llama-v3-70b"`)
}
