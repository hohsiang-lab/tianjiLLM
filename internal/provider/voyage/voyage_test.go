package voyage

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/model"
	"github.com/praxisllmlab/tianjiLLM/internal/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProviderRegistered(t *testing.T) {
	p, err := provider.Get("voyage")
	require.NoError(t, err)
	assert.NotNil(t, p)
}

func TestGetWithBaseURLPreservesVoyageAdapter(t *testing.T) {
	configured, err := provider.GetWithBaseURL("voyage", "http://voyage.test/v1")
	require.NoError(t, err)
	require.IsType(t, &Provider{}, configured)

	embProvider, ok := configured.(provider.EmbeddingProvider)
	require.True(t, ok, "voyage provider should implement EmbeddingProvider")

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body: io.NopCloser(strings.NewReader(
			`{"object":"list","data":[{"object":"embedding","index":0,"embedding":[0.1]}],"model":"voyage-4-large","usage":{"total_tokens":3}}`,
		)),
	}
	result, err := embProvider.TransformEmbeddingResponse(context.Background(), resp)
	require.NoError(t, err)
	assert.Equal(t, model.EmbeddingUsage{PromptTokens: 3, TotalTokens: 3}, result.Usage)
}

func TestGetSupportedParams(t *testing.T) {
	p, _ := provider.Get("voyage")
	params := p.GetSupportedParams()
	assert.NotEmpty(t, params)
}

func TestGetRequestURL(t *testing.T) {
	p, _ := provider.Get("voyage")
	url := p.GetRequestURL("test-model")
	assert.NotEmpty(t, url)
}

func TestTransformEmbeddingResponse_TotalOnlyUsage(t *testing.T) {
	p, err := provider.Get("voyage")
	require.NoError(t, err)

	embProvider, ok := p.(provider.EmbeddingProvider)
	require.True(t, ok, "voyage provider should implement EmbeddingProvider")

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body: io.NopCloser(strings.NewReader(
			`{"object":"list","data":[{"object":"embedding","index":0,"embedding":[0.1]}],"model":"voyage-4-large","usage":{"total_tokens":3}}`,
		)),
	}
	result, err := embProvider.TransformEmbeddingResponse(context.Background(), resp)
	require.NoError(t, err)
	assert.Equal(t, model.EmbeddingUsage{PromptTokens: 3, TotalTokens: 3}, result.Usage)
}
