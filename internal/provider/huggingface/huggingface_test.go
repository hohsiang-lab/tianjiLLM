package huggingface

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
	p, err := provider.Get("huggingface")
	require.NoError(t, err)
	assert.NotNil(t, p)
}

func TestGetRequestURL(t *testing.T) {
	p := newTestProvider()
	assert.Equal(t, "https://router.huggingface.co/v1/chat/completions", p.GetRequestURL("meta-llama/Llama-3-70B"))
}

func TestTransformRequest(t *testing.T) {
	p := newTestProvider()
	ctx := context.Background()

	req := &model.ChatCompletionRequest{
		Model:    "meta-llama/Llama-3-70B",
		Messages: []model.Message{{Role: "user", Content: "Hi"}},
	}

	httpReq, err := p.TransformRequest(ctx, req, "hf-key")
	require.NoError(t, err)

	assert.Equal(t, http.MethodPost, httpReq.Method)
	assert.Equal(t, "Bearer hf-key", httpReq.Header.Get("Authorization"))

	body, _ := io.ReadAll(httpReq.Body)
	assert.Contains(t, string(body), `"model":"meta-llama/Llama-3-70B"`)
}
