package jina

import (
	"context"
	"encoding/json"
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
	p, err := provider.Get("jina_ai")
	require.NoError(t, err)
	assert.NotNil(t, p)
}

func TestGetSupportedParams(t *testing.T) {
	p, err := provider.Get("jina_ai")
	require.NoError(t, err)
	params := p.GetSupportedParams()
	assert.NotEmpty(t, params)
	assert.Contains(t, params, "task")
	assert.Contains(t, params, "normalized")
	assert.Contains(t, params, "dimensions")
}

func TestGetRequestURL(t *testing.T) {
	p, err := provider.Get("jina_ai")
	require.NoError(t, err)
	url := p.GetRequestURL("test-model")
	assert.NotEmpty(t, url)
}

// TestTransformEmbeddingRequest_ExtraParams verifies that Jina-specific fields
// (task, normalized) passed via ExtraParams are forwarded in the upstream request body.
func TestTransformEmbeddingRequest_ExtraParams(t *testing.T) {
	p, err := provider.Get("jina_ai")
	require.NoError(t, err)

	embProvider, ok := p.(provider.EmbeddingProvider)
	require.True(t, ok, "jina provider should implement EmbeddingProvider")

	normalized := true
	req := &model.EmbeddingRequest{
		Model: "jina-embeddings-v3",
		Input: "test sentence",
		ExtraParams: map[string]any{
			"task":       "retrieval.query",
			"normalized": normalized,
		},
	}

	httpReq, err := embProvider.TransformEmbeddingRequest(context.Background(), req, "test-api-key")
	require.NoError(t, err)

	body, err := io.ReadAll(httpReq.Body)
	require.NoError(t, err)

	var parsed map[string]any
	require.NoError(t, json.Unmarshal(body, &parsed))

	assert.Equal(t, "jina-embeddings-v3", parsed["model"])
	assert.Equal(t, "test sentence", parsed["input"])
	assert.Equal(t, "retrieval.query", parsed["task"], "task should be forwarded to upstream")
	assert.Equal(t, true, parsed["normalized"], "normalized should be forwarded to upstream")
}

// TestTransformEmbeddingRequest_NoExtraParams verifies standard embedding request
// (without ExtraParams) still works correctly.
func TestTransformEmbeddingRequest_NoExtraParams(t *testing.T) {
	p, err := provider.Get("jina_ai")
	require.NoError(t, err)

	embProvider, ok := p.(provider.EmbeddingProvider)
	require.True(t, ok)

	dim := 1024
	req := &model.EmbeddingRequest{
		Model:      "jina-embeddings-v3",
		Input:      "test sentence",
		Dimensions: &dim,
	}

	httpReq, err := embProvider.TransformEmbeddingRequest(context.Background(), req, "test-api-key")
	require.NoError(t, err)

	body, err := io.ReadAll(httpReq.Body)
	require.NoError(t, err)

	var parsed map[string]any
	require.NoError(t, json.Unmarshal(body, &parsed))

	assert.Equal(t, "jina-embeddings-v3", parsed["model"])
	assert.Equal(t, float64(1024), parsed["dimensions"])
	assert.NotContains(t, parsed, "task")
	assert.NotContains(t, parsed, "normalized")
}

// TestEmbeddingRequestUnmarshal_ExtraParams verifies that unknown fields in
// the incoming JSON are captured in ExtraParams.
func TestEmbeddingRequestUnmarshal_ExtraParams(t *testing.T) {
	raw := `{
		"model": "jina-embeddings-v3",
		"input": "hello",
		"task": "retrieval.passage",
		"normalized": true
	}`

	var req model.EmbeddingRequest
	require.NoError(t, json.Unmarshal([]byte(raw), &req))

	assert.Equal(t, "jina-embeddings-v3", req.Model)
	assert.Equal(t, "hello", req.Input)
	assert.Equal(t, "retrieval.passage", req.ExtraParams["task"])
	assert.Equal(t, true, req.ExtraParams["normalized"])
}

// TestTransformEmbeddingRequest_Base64EncodingFormatStripped preserves the
// provider-adapter transformation for direct callers. The public
// /v1/embeddings handler translates base64 requests to float before this
// adapter is reached.
func TestTransformEmbeddingRequest_Base64EncodingFormatStripped(t *testing.T) {
	p, err := provider.Get("jina_ai")
	require.NoError(t, err)

	embProvider, ok := p.(provider.EmbeddingProvider)
	require.True(t, ok, "jina provider should implement EmbeddingProvider")

	req := &model.EmbeddingRequest{
		Model:          "jina-embeddings-v5-text-small",
		Input:          "test",
		EncodingFormat: "base64",
	}

	httpReq, err := embProvider.TransformEmbeddingRequest(context.Background(), req, "test-api-key")
	require.NoError(t, err, "encoding_format=base64 should be stripped, not rejected")
	require.NotNil(t, httpReq)

	body, err := io.ReadAll(httpReq.Body)
	require.NoError(t, err)

	var parsed map[string]any
	require.NoError(t, json.Unmarshal(body, &parsed))

	// encoding_format must not appear in the upstream request
	_, hasEncodingFormat := parsed["encoding_format"]
	assert.False(t, hasEncodingFormat, "encoding_format=base64 must be stripped from upstream Jina request")

	// original req must not be mutated
	assert.Equal(t, "base64", req.EncodingFormat, "original EmbeddingRequest must not be mutated")
}

func TestTransformEmbeddingResponse_TotalOnlyUsage(t *testing.T) {
	p, err := provider.Get("jina_ai")
	require.NoError(t, err)

	embProvider, ok := p.(provider.EmbeddingProvider)
	require.True(t, ok, "jina provider should implement EmbeddingProvider")

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body: io.NopCloser(strings.NewReader(
			`{"object":"list","data":[{"object":"embedding","index":0,"embedding":[0.1]}],"model":"jina-embeddings-v3","usage":{"total_tokens":3}}`,
		)),
	}
	result, err := embProvider.TransformEmbeddingResponse(context.Background(), resp)
	require.NoError(t, err)
	assert.Equal(t, 3, result.Usage.PromptTokens)
	assert.Equal(t, 3, result.Usage.TotalTokens)
}

// TestTransformEmbeddingRequest_ExtraParamsCannotOverrideKnownFields verifies
// that ExtraParams cannot shadow known body fields like model or input.
func TestTransformEmbeddingRequest_ExtraParamsCannotOverrideKnownFields(t *testing.T) {
	p, err := provider.Get("jina_ai")
	require.NoError(t, err)

	embProvider, ok := p.(provider.EmbeddingProvider)
	require.True(t, ok)

	req := &model.EmbeddingRequest{
		Model: "jina-embeddings-v3",
		Input: "real input",
		ExtraParams: map[string]any{
			"model": "injected-model", // should be ignored
			"input": "injected input", // should be ignored
			"task":  "retrieval.query",
		},
	}

	httpReq, err := embProvider.TransformEmbeddingRequest(context.Background(), req, "test-api-key")
	require.NoError(t, err)

	body, err := io.ReadAll(httpReq.Body)
	require.NoError(t, err)

	var parsed map[string]any
	require.NoError(t, json.Unmarshal(body, &parsed))

	assert.Equal(t, "jina-embeddings-v3", parsed["model"], "ExtraParams must not override model")
	assert.Equal(t, "real input", parsed["input"], "ExtraParams must not override input")
	assert.Equal(t, "retrieval.query", parsed["task"])
}

// TestEmbeddingRequestUnmarshal_RoundTrip verifies that standard fields survive
// unmarshal → transformEmbeddingRequestBody → marshal round-trip.
func TestEmbeddingRequestUnmarshal_RoundTrip(t *testing.T) {
	raw := `{"model":"jina-embeddings-v3","input":"hello","task":"retrieval.query","normalized":true,"dimensions":1024}`

	var req model.EmbeddingRequest
	require.NoError(t, json.Unmarshal([]byte(raw), &req))

	p, err := provider.Get("jina_ai")
	require.NoError(t, err)
	embProvider, ok := p.(provider.EmbeddingProvider)
	require.True(t, ok, "jina provider should implement EmbeddingProvider")
	httpReq, err := embProvider.TransformEmbeddingRequest(context.Background(), &req, "test-key")
	require.NoError(t, err)

	body, err := io.ReadAll(httpReq.Body)
	require.NoError(t, err)
	var out map[string]any
	err = json.Unmarshal(body, &out)
	require.NoError(t, err)

	assert.Equal(t, "jina-embeddings-v3", out["model"])
	assert.Equal(t, "hello", out["input"])
	assert.Equal(t, "retrieval.query", out["task"])
	assert.Equal(t, true, out["normalized"])
	assert.Equal(t, float64(1024), out["dimensions"])

	// Make sure no extra keys leaked in
	_, hasUser := out["user"]
	assert.False(t, hasUser, "empty user field should not be serialized")
}
