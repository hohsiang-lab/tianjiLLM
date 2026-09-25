package sambanova

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
	p, err := provider.Get("sambanova")
	require.NoError(t, err)
	assert.NotNil(t, p)
}

func TestGetRequestURL(t *testing.T) {
	p := newTestProvider()
	assert.Equal(t, "https://api.sambanova.ai/v1/chat/completions", p.GetRequestURL("Meta-Llama-3-70B"))
}

func TestGetSupportedParams(t *testing.T) {
	p := newTestProvider()
	params := p.GetSupportedParams()
	assert.Contains(t, params, "top_k")
}

func TestTransformRequest(t *testing.T) {
	p := newTestProvider()
	ctx := context.Background()

	req := &model.ChatCompletionRequest{
		Model:    "Meta-Llama-3-70B",
		Messages: []model.Message{{Role: "user", Content: "Hi"}},
	}

	httpReq, err := p.TransformRequest(ctx, req, "sn-key")
	require.NoError(t, err)

	assert.Equal(t, http.MethodPost, httpReq.Method)
	assert.Equal(t, "Bearer sn-key", httpReq.Header.Get("Authorization"))

	body, _ := io.ReadAll(httpReq.Body)
	assert.Contains(t, string(body), `"model":"Meta-Llama-3-70B"`)
}
