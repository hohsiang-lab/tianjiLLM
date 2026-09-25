package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/praxisllmlab/tianjiLLM/internal/model"
)

func TestTransformEmbeddingRequest_BasicFields(t *testing.T) {
	p := NewWithBaseURL("https://api.openai.com")
	req := &model.EmbeddingRequest{
		Model: "text-embedding-3-small",
		Input: "hello world",
	}
	httpReq, err := p.TransformEmbeddingRequest(context.Background(), req, "sk-test")
	require.NoError(t, err)
	assert.Equal(t, "https://api.openai.com/embeddings", httpReq.URL.String())
	assert.Equal(t, "Bearer sk-test", httpReq.Header.Get("Authorization"))

	var body map[string]any
	require.NoError(t, json.NewDecoder(httpReq.Body).Decode(&body))
	assert.Equal(t, "text-embedding-3-small", body["model"])
	assert.Equal(t, "hello world", body["input"])
}

func TestTransformEmbeddingRequest_ExtraParamsPassThrough(t *testing.T) {
	// ExtraParams (e.g. Jina-specific fields) should be merged into request body.
	p := NewWithBaseURL("https://api.openai.com")
	req := &model.EmbeddingRequest{
		Model: "jina-embeddings-v3",
		Input: "test",
		ExtraParams: map[string]any{
			"task":       "retrieval.passage",
			"normalized": true,
		},
	}
	httpReq, err := p.TransformEmbeddingRequest(context.Background(), req, "sk-test")
	require.NoError(t, err)

	var body map[string]any
	require.NoError(t, json.NewDecoder(httpReq.Body).Decode(&body))
	assert.Equal(t, "retrieval.passage", body["task"])
	assert.Equal(t, true, body["normalized"])
}

func TestTransformEmbeddingRequest_NilExtraParams(t *testing.T) {
	p := NewWithBaseURL("https://api.openai.com")
	req := &model.EmbeddingRequest{
		Model:       "text-embedding-3-small",
		Input:       "hello",
		ExtraParams: nil,
	}
	httpReq, err := p.TransformEmbeddingRequest(context.Background(), req, "sk-test")
	require.NoError(t, err)

	var body map[string]any
	require.NoError(t, json.NewDecoder(httpReq.Body).Decode(&body))
	// Should not have jina-specific fields
	assert.NotContains(t, body, "task")
	assert.NotContains(t, body, "normalized")
}

func TestTransformEmbeddingResponse_FloatArray(t *testing.T) {
	p := NewWithBaseURL("https://api.openai.com")
	respBody := `{
		"object": "list",
		"data": [{"object":"embedding","index":0,"embedding":[0.1,0.2,0.3]}],
		"model": "text-embedding-3-small",
		"usage": {"prompt_tokens":3,"total_tokens":3}
	}`
	// Use a real HTTP response
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(respBody))
	}))
	defer srv.Close()

	httpResp, err := http.Get(srv.URL)
	require.NoError(t, err)

	result, err := p.TransformEmbeddingResponse(context.Background(), httpResp)
	require.NoError(t, err)
	require.Len(t, result.Data, 1)
	assert.InDelta(t, 0.1, result.Data[0].Embedding[0], 0.001)
	assert.InDelta(t, 0.2, result.Data[0].Embedding[1], 0.001)
	assert.InDelta(t, 0.3, result.Data[0].Embedding[2], 0.001)
}

func TestTransformEmbeddingResponse_PreservesMultiInputOrderingAndUsage(t *testing.T) {
	p := NewWithBaseURL("https://api.openai.com")
	resp := embeddingResponse(http.StatusOK, `{
		"object":"list",
		"data":[
			{"object":"embedding","index":0,"embedding":[0.1,0.2]},
			{"object":"embedding","index":1,"embedding":[0.3,0.4]}
		],
		"model":"text-embedding-3-small",
		"usage":{"prompt_tokens":4,"total_tokens":4}
	}`)

	result, err := p.TransformEmbeddingResponse(context.Background(), resp)

	require.NoError(t, err)
	require.Len(t, result.Data, 2)
	assert.Equal(t, 0, result.Data[0].Index)
	assert.Equal(t, 1, result.Data[1].Index)
	assert.Equal(t, []float64{0.1, 0.2}, result.Data[0].Embedding)
	assert.Equal(t, []float64{0.3, 0.4}, result.Data[1].Embedding)
	assert.Equal(t, 4, result.Usage.PromptTokens)
	assert.Equal(t, 4, result.Usage.TotalTokens)
}

func TestTransformEmbeddingResponse_RejectsInvalidShape(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{
			name: "non-list object",
			body: `{"object":"embedding","data":[{"object":"embedding","index":0,"embedding":[0.1]}],"model":"m","usage":{"prompt_tokens":1,"total_tokens":1}}`,
		},
		{
			name: "wrong item object",
			body: `{"object":"list","data":[{"object":"vector","index":0,"embedding":[0.1]}],"model":"m","usage":{"prompt_tokens":1,"total_tokens":1}}`,
		},
		{
			name: "non-zero-based index",
			body: `{"object":"list","data":[{"object":"embedding","index":1,"embedding":[0.1]}],"model":"m","usage":{"prompt_tokens":1,"total_tokens":1}}`,
		},
		{
			name: "empty numeric vector",
			body: `{"object":"list","data":[{"object":"embedding","index":0,"embedding":[]}],"model":"m","usage":{"prompt_tokens":1,"total_tokens":1}}`,
		},
		{
			name: "invalid usage",
			body: `{"object":"list","data":[{"object":"embedding","index":0,"embedding":[0.1]}],"model":"m","usage":{"prompt_tokens":2,"total_tokens":1}}`,
		},
		{
			name: "missing usage",
			body: `{"object":"list","data":[{"object":"embedding","index":0,"embedding":[0.1]}],"model":"m"}`,
		},
		{
			name: "missing prompt tokens",
			body: `{"object":"list","data":[{"object":"embedding","index":0,"embedding":[0.1]}],"model":"m","usage":{"total_tokens":1}}`,
		},
		{
			name: "missing total tokens",
			body: `{"object":"list","data":[{"object":"embedding","index":0,"embedding":[0.1]}],"model":"m","usage":{"prompt_tokens":1}}`,
		},
		{
			name: "base64 vector",
			body: `{"object":"list","data":[{"object":"embedding","index":0,"embedding":"Zm9v"}],"model":"m","usage":{"prompt_tokens":1,"total_tokens":1}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewWithBaseURL("https://api.openai.com").
				TransformEmbeddingResponse(context.Background(), embeddingResponse(http.StatusOK, tt.body))
			require.Error(t, err)
		})
	}
}

func TestTransformEmbeddingResponse_Non200(t *testing.T) {
	p := NewWithBaseURL("https://api.openai.com")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(429)
		_, _ = w.Write([]byte(`{"error":{"message":"rate limited","type":"rate_limit_error"}}`))
	}))
	defer srv.Close()

	httpResp, err := http.Get(srv.URL)
	require.NoError(t, err)

	_, err = p.TransformEmbeddingResponse(context.Background(), httpResp)
	require.Error(t, err)
}

func embeddingResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(bytes.NewBufferString(body)),
	}
}
