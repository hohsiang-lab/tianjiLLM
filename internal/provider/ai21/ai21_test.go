package ai21

import (
	"context"
	"net/http"
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/model"
	"github.com/praxisllmlab/tianjiLLM/internal/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProviderRegistered(t *testing.T) {
	p, err := provider.Get("ai21")
	require.NoError(t, err)
	assert.NotNil(t, p)
}

func TestGetSupportedParams(t *testing.T) {
	p := New()
	params := p.GetSupportedParams()
	assert.NotEmpty(t, params)
	assert.Contains(t, params, "model")
}

func TestGetRequestURL(t *testing.T) {
	p := New()
	url := p.GetRequestURL("jamba-1.5-large")
	assert.Contains(t, url, "api.ai21.com")
}

func TestTransformRequest(t *testing.T) {
	p := New()
	req := &model.ChatCompletionRequest{
		Model:    "jamba-1.5-large",
		Messages: []model.Message{{Role: "user", Content: "Hello"}},
	}
	httpReq, err := p.TransformRequest(context.Background(), req, "test-key")
	require.NoError(t, err)
	assert.Equal(t, http.MethodPost, httpReq.Method)
	assert.Equal(t, "Bearer test-key", httpReq.Header.Get("Authorization"))
}

func TestSetupHeaders(t *testing.T) {
	p := New()
	req, _ := http.NewRequest("POST", "https://example.com", nil)
	p.SetupHeaders(req, "my-key")
	assert.Equal(t, "Bearer my-key", req.Header.Get("Authorization"))
}

func TestMapParams(t *testing.T) {
	p := New()
	params := map[string]any{"model": "test"}
	result := p.MapParams(params)
	assert.Equal(t, params, result)
}
