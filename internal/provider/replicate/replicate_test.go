package replicate

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/model"
	"github.com/praxisllmlab/tianjiLLM/internal/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestProvider() *Provider {
	return &Provider{baseURL: defaultBaseURL}
}

func TestProviderRegistered(t *testing.T) {
	p, err := provider.Get("replicate")
	require.NoError(t, err)
	assert.NotNil(t, p)
}

func TestGetRequestURL(t *testing.T) {
	p := newTestProvider()
	assert.Equal(t, "https://api.replicate.com/v1/models/meta/llama-3-70b/predictions", p.GetRequestURL("meta/llama-3-70b"))
}

func TestSetupHeaders_TokenAuth(t *testing.T) {
	p := newTestProvider()
	req, _ := http.NewRequest(http.MethodPost, "https://example.com", nil)
	p.SetupHeaders(req, "r8_test_key")

	assert.Equal(t, "Token r8_test_key", req.Header.Get("Authorization"))
	assert.Equal(t, "application/json", req.Header.Get("Content-Type"))
}

func TestMapParams(t *testing.T) {
	p := newTestProvider()

	result := p.MapParams(map[string]any{
		"max_tokens":  500,
		"stop":        []string{"END"},
		"temperature": 0.7,
	})

	assert.Equal(t, 500, result["max_new_tokens"])
	assert.Equal(t, []string{"END"}, result["stop_sequences"])
	assert.Equal(t, 0.7, result["temperature"])
	assert.NotContains(t, result, "max_tokens")
	assert.NotContains(t, result, "stop")
}

func TestTransformRequest(t *testing.T) {
	p := newTestProvider()
	ctx := context.Background()

	temp := 0.8
	maxTokens := 100
	stream := true
	seed := 42
	req := &model.ChatCompletionRequest{
		Model: "meta/llama-3-70b",
		Messages: []model.Message{
			{Role: "user", Content: "Hello"},
		},
		Temperature: &temp,
		MaxTokens:   &maxTokens,
		Stream:      &stream,
		Seed:        &seed,
	}

	httpReq, err := p.TransformRequest(ctx, req, "r8_key")
	require.NoError(t, err)

	assert.Equal(t, http.MethodPost, httpReq.Method)
	assert.Equal(t, "https://api.replicate.com/v1/models/meta/llama-3-70b/predictions", httpReq.URL.String())
	assert.Equal(t, "Token r8_key", httpReq.Header.Get("Authorization"))

	body, _ := io.ReadAll(httpReq.Body)
	var parsed map[string]any
	require.NoError(t, json.Unmarshal(body, &parsed))

	assert.Equal(t, true, parsed["stream"])

	input, ok := parsed["input"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, 0.8, input["temperature"])
	assert.Equal(t, float64(100), input["max_new_tokens"])
	assert.Equal(t, float64(42), input["seed"])
}

func TestTransformResponse(t *testing.T) {
	p := newTestProvider()
	ctx := context.Background()

	respBody := `{
		"id": "chatcmpl-replicate",
		"object": "chat.completion",
		"model": "meta/llama-3-70b",
		"choices": [{"index": 0, "message": {"role": "assistant", "content": "Hi!"}, "finish_reason": "stop"}],
		"usage": {"prompt_tokens": 5, "completion_tokens": 2, "total_tokens": 7}
	}`

	resp := &http.Response{
		StatusCode: http.StatusCreated,
		Body:       io.NopCloser(bytes.NewReader([]byte(respBody))),
		Header:     http.Header{},
	}

	result, err := p.TransformResponse(ctx, resp)
	require.NoError(t, err)
	assert.Equal(t, "chatcmpl-replicate", result.ID)
	require.Len(t, result.Choices, 1)
}

func TestTransformResponse_Error(t *testing.T) {
	p := newTestProvider()
	ctx := context.Background()

	resp := &http.Response{
		StatusCode: http.StatusNotFound,
		Body:       io.NopCloser(bytes.NewReader([]byte(`{"detail":"Model not found"}`))),
		Header:     http.Header{},
	}

	_, err := p.TransformResponse(ctx, resp)
	require.Error(t, err)

	var tianjiErr *model.TianjiError
	require.ErrorAs(t, err, &tianjiErr)
	assert.Equal(t, 404, tianjiErr.StatusCode)
	assert.Equal(t, "replicate", tianjiErr.Provider)
	assert.Equal(t, "Model not found", tianjiErr.Message)
}

func TestTransformStreamChunk(t *testing.T) {
	p := &Provider{}
	// passing done signal
	_, _, _ = p.TransformStreamChunk(context.Background(), []byte("data: [DONE]"))
}

func TestGetSupportedParams(t *testing.T) {
	p := &Provider{}
	params := p.GetSupportedParams()
	assert.NotEmpty(t, params)
}

func TestMapParams_MaxTokens(t *testing.T) {
	p := &Provider{}
	in := map[string]any{"max_tokens": 100, "stop": []string{"end"}, "other": "val"}
	out := p.MapParams(in)
	assert.Equal(t, 100, out["max_new_tokens"])
	assert.Equal(t, []string{"end"}, out["stop_sequences"])
	assert.Equal(t, "val", out["other"])
}
