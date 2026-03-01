package cloudflare

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

func TestProviderRegistered(t *testing.T) {
	p, err := provider.Get("cloudflare")
	require.NoError(t, err)
	assert.NotNil(t, p)
}

func TestGetRequestURL(t *testing.T) {
	p := &Provider{}
	assert.Equal(t,
		"https://api.cloudflare.com/client/v4/accounts/abc123/@cf/meta/llama-3-8b/ai/run",
		p.GetRequestURL("abc123/@cf/meta/llama-3-8b"),
	)
}

func TestGetSupportedParams(t *testing.T) {
	p := &Provider{}
	params := p.GetSupportedParams()
	assert.Contains(t, params, "stream")
	assert.Contains(t, params, "max_tokens")
	assert.NotContains(t, params, "tools")
	assert.NotContains(t, params, "temperature")
}

func TestMapParams_MaxCompletionTokens(t *testing.T) {
	p := &Provider{}
	result := p.MapParams(map[string]any{
		"max_completion_tokens": 500,
	})
	assert.Equal(t, 500, result["max_tokens"])
	assert.NotContains(t, result, "max_completion_tokens")
}

func TestTransformRequest(t *testing.T) {
	p := &Provider{}
	ctx := context.Background()

	maxTokens := 256
	stream := false
	req := &model.ChatCompletionRequest{
		Model: "abc123/@cf/meta/llama-3-8b",
		Messages: []model.Message{
			{Role: "user", Content: "Hello"},
		},
		MaxTokens: &maxTokens,
		Stream:    &stream,
	}

	httpReq, err := p.TransformRequest(ctx, req, "cf-key")
	require.NoError(t, err)

	assert.Equal(t, http.MethodPost, httpReq.Method)
	assert.Equal(t, "Bearer cf-key", httpReq.Header.Get("Authorization"))
	assert.Equal(t, "application/json", httpReq.Header.Get("Content-Type"))

	body, _ := io.ReadAll(httpReq.Body)
	var parsed map[string]any
	require.NoError(t, json.Unmarshal(body, &parsed))

	assert.NotNil(t, parsed["messages"])
	assert.Equal(t, float64(256), parsed["max_tokens"])
	assert.Equal(t, false, parsed["stream"])
	assert.NotContains(t, parsed, "model")
}

func TestTransformResponse(t *testing.T) {
	p := &Provider{}
	ctx := context.Background()

	respBody := `{
		"id": "chatcmpl-cf",
		"object": "chat.completion",
		"model": "@cf/meta/llama-3-8b",
		"choices": [{"index": 0, "message": {"role": "assistant", "content": "Hi!"}, "finish_reason": "stop"}],
		"usage": {"prompt_tokens": 5, "completion_tokens": 2, "total_tokens": 7}
	}`

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader([]byte(respBody))),
		Header:     http.Header{},
	}

	result, err := p.TransformResponse(ctx, resp)
	require.NoError(t, err)
	assert.Equal(t, "chatcmpl-cf", result.ID)
	require.Len(t, result.Choices, 1)
}

func TestTransformResponse_Error(t *testing.T) {
	p := &Provider{}
	ctx := context.Background()

	resp := &http.Response{
		StatusCode: http.StatusForbidden,
		Body:       io.NopCloser(bytes.NewReader([]byte(`{"errors":[{"message":"authentication error"}]}`))),
		Header:     http.Header{},
	}

	_, err := p.TransformResponse(ctx, resp)
	require.Error(t, err)

	var tianjiErr *model.TianjiError
	require.ErrorAs(t, err, &tianjiErr)
	assert.Equal(t, 403, tianjiErr.StatusCode)
	assert.Equal(t, "cloudflare", tianjiErr.Provider)
	assert.Equal(t, "authentication error", tianjiErr.Message)
}

func TestTransformStreamChunk(t *testing.T) {
	p := &Provider{}
	// Empty data line → parsed as empty or error
	_, _, _ = p.TransformStreamChunk(context.Background(), []byte("data: [DONE]"))
}

func TestMapParams_Passthrough(t *testing.T) {
	p := &Provider{}
	in := map[string]any{"temperature": 0.7, "stream": true}
	out := p.MapParams(in)
	assert.Equal(t, 0.7, out["temperature"])
	assert.Equal(t, true, out["stream"])
}
