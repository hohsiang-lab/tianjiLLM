package together

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
	p, err := provider.Get("together_ai")
	require.NoError(t, err)
	assert.NotNil(t, p)
}

func TestGetRequestURL(t *testing.T) {
	p := newTestProvider()
	assert.Equal(t, "https://api.together.xyz/v1/chat/completions", p.GetRequestURL("meta-llama/Llama-3-70b"))
}

func TestGetSupportedParams(t *testing.T) {
	p := newTestProvider()
	params := p.GetSupportedParams()
	assert.Contains(t, params, "model")
	assert.Contains(t, params, "messages")
	assert.Contains(t, params, "temperature")
	assert.Contains(t, params, "tools")
	assert.Contains(t, params, "stream_options")
	assert.Contains(t, params, "response_format")
}

func TestTransformRequest(t *testing.T) {
	p := newTestProvider()
	ctx := context.Background()

	temp := 0.7
	req := &model.ChatCompletionRequest{
		Model: "meta-llama/Llama-3-70b",
		Messages: []model.Message{
			{Role: "user", Content: "Hello"},
		},
		Temperature: &temp,
	}

	httpReq, err := p.TransformRequest(ctx, req, "tog-test-key")
	require.NoError(t, err)

	assert.Equal(t, http.MethodPost, httpReq.Method)
	assert.Equal(t, "https://api.together.xyz/v1/chat/completions", httpReq.URL.String())
	assert.Equal(t, "Bearer tog-test-key", httpReq.Header.Get("Authorization"))
	assert.Equal(t, "application/json", httpReq.Header.Get("Content-Type"))

	body, _ := io.ReadAll(httpReq.Body)
	assert.Contains(t, string(body), `"model":"meta-llama/Llama-3-70b"`)
}
