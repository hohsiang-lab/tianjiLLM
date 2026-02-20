package cohere

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
	p, err := provider.Get("cohere")
	require.NoError(t, err)
	assert.NotNil(t, p)
}

func TestGetRequestURL(t *testing.T) {
	p := newTestProvider()
	assert.Equal(t, "https://api.cohere.ai/v2/chat", p.GetRequestURL("command-r-plus"))
}

func TestGetSupportedParams(t *testing.T) {
	p := newTestProvider()
	params := p.GetSupportedParams()
	assert.Contains(t, params, "model")
	assert.Contains(t, params, "seed")
	assert.Contains(t, params, "tools")
	assert.Contains(t, params, "max_completion_tokens")
}

func TestMapParams(t *testing.T) {
	p := newTestProvider()

	result := p.MapParams(map[string]any{
		"top_p":                 0.9,
		"stop":                  []string{"END"},
		"n":                     3,
		"max_completion_tokens": 500,
		"temperature":           0.7,
	})

	assert.Equal(t, 0.9, result["p"])
	assert.Equal(t, []string{"END"}, result["stop_sequences"])
	assert.Equal(t, 3, result["num_generations"])
	assert.Equal(t, 500, result["max_tokens"])
	assert.Equal(t, 0.7, result["temperature"])
	assert.NotContains(t, result, "top_p")
	assert.NotContains(t, result, "stop")
	assert.NotContains(t, result, "n")
}

func TestTransformRequest(t *testing.T) {
	p := newTestProvider()
	ctx := context.Background()

	temp := 0.8
	topP := 0.9
	stream := true
	req := &model.ChatCompletionRequest{
		Model: "command-r-plus",
		Messages: []model.Message{
			{Role: "user", Content: "Hello"},
		},
		Temperature: &temp,
		TopP:        &topP,
		Stream:      &stream,
	}

	httpReq, err := p.TransformRequest(ctx, req, "co-key")
	require.NoError(t, err)

	assert.Equal(t, http.MethodPost, httpReq.Method)
	assert.Equal(t, "https://api.cohere.ai/v2/chat", httpReq.URL.String())
	assert.Equal(t, "Bearer co-key", httpReq.Header.Get("Authorization"))
	assert.Equal(t, "application/json", httpReq.Header.Get("Content-Type"))

	body, _ := io.ReadAll(httpReq.Body)
	var parsed map[string]any
	require.NoError(t, json.Unmarshal(body, &parsed))

	assert.Equal(t, "command-r-plus", parsed["model"])
	assert.Equal(t, 0.8, parsed["temperature"])
	assert.Equal(t, 0.9, parsed["p"])
	assert.Equal(t, true, parsed["stream"])
}

func TestTransformResponse(t *testing.T) {
	p := newTestProvider()
	ctx := context.Background()

	respBody := `{
		"id": "chatcmpl-cohere",
		"object": "chat.completion",
		"model": "command-r-plus",
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
	assert.Equal(t, "chatcmpl-cohere", result.ID)
	assert.Equal(t, "command-r-plus", result.Model)
	require.Len(t, result.Choices, 1)
	assert.Equal(t, "Hi!", result.Choices[0].Message.Content)
}

func TestTransformResponse_Error(t *testing.T) {
	p := newTestProvider()
	ctx := context.Background()

	resp := &http.Response{
		StatusCode: http.StatusUnauthorized,
		Body:       io.NopCloser(bytes.NewReader([]byte(`{"message":"invalid api key"}`))),
		Header:     http.Header{},
	}

	_, err := p.TransformResponse(ctx, resp)
	require.Error(t, err)

	var tianjiErr *model.TianjiError
	require.ErrorAs(t, err, &tianjiErr)
	assert.Equal(t, 401, tianjiErr.StatusCode)
	assert.Equal(t, "cohere", tianjiErr.Provider)
	assert.Equal(t, "invalid api key", tianjiErr.Message)
}
