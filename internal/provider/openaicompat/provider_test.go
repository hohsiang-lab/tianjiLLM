package openaicompat

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testProvider() *Provider {
	return NewFromConfig(SimpleProviderConfig{
		Name:    "testprovider",
		BaseURL: "https://api.test.example.com/v1",
	})
}

func TestTransformRequest(t *testing.T) {
	p := testProvider()
	maxCompletionTokens := 75
	stream := true
	req := &model.ChatCompletionRequest{
		Model:               "test-model",
		Messages:            []model.Message{{Role: "user", Content: "hello"}},
		MaxCompletionTokens: &maxCompletionTokens,
		ResponseFormat:      map[string]any{"type": "json_object"},
		Stream:              &stream,
		StreamOptions:       &model.StreamOptions{IncludeUsage: true},
	}
	httpReq, err := p.TransformRequest(context.Background(), req, "mykey")
	require.NoError(t, err)
	assert.Equal(t, "Bearer mykey", httpReq.Header.Get("Authorization"))
	assert.Contains(t, httpReq.URL.String(), "/chat/completions")
	body, err := io.ReadAll(httpReq.Body)
	require.NoError(t, err)
	var parsed map[string]any
	require.NoError(t, json.Unmarshal(body, &parsed))
	assert.Equal(t, float64(75), parsed["max_completion_tokens"])
	assert.Equal(t, map[string]any{"type": "json_object"}, parsed["response_format"])
	assert.Equal(t, map[string]any{"include_usage": true}, parsed["stream_options"])
}

func TestTransformRequest_AppliesParamMappingsToResponseFormatAndStreamOptions(t *testing.T) {
	p := NewFromConfig(SimpleProviderConfig{
		Name:    "mapped",
		BaseURL: "https://api.test.example.com/v1",
		ParamMappings: map[string]string{
			"response_format": "format",
			"stream_options":  "stream_config",
		},
	})
	stream := true
	req := &model.ChatCompletionRequest{
		Model:          "test-model",
		Messages:       []model.Message{{Role: "user", Content: "hello"}},
		ResponseFormat: map[string]any{"type": "json_object"},
		Stream:         &stream,
		StreamOptions:  &model.StreamOptions{IncludeUsage: true},
	}

	httpReq, err := p.TransformRequest(context.Background(), req, "mykey")
	require.NoError(t, err)
	body, err := io.ReadAll(httpReq.Body)
	require.NoError(t, err)
	var parsed map[string]any
	require.NoError(t, json.Unmarshal(body, &parsed))
	assert.Equal(t, map[string]any{"type": "json_object"}, parsed["format"])
	assert.Equal(t, map[string]any{"include_usage": true}, parsed["stream_config"])
	assert.NotContains(t, parsed, "response_format")
	assert.NotContains(t, parsed, "stream_options")
}

func TestTransformResponse_Success(t *testing.T) {
	p := testProvider()
	body := `{"id":"1","object":"chat.completion","choices":[{"message":{"role":"assistant","content":"hi"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader([]byte(body))),
	}
	result, err := p.TransformResponse(context.Background(), resp)
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestTransformResponse_Error(t *testing.T) {
	p := testProvider()
	resp := &http.Response{
		StatusCode: http.StatusUnauthorized,
		Body:       io.NopCloser(bytes.NewReader([]byte(`{"error":{"message":"bad key","type":"auth"}}`))),
	}
	_, err := p.TransformResponse(context.Background(), resp)
	assert.Error(t, err)
}

func TestTransformStreamChunk(t *testing.T) {
	p := testProvider()
	data := []byte(`data: {"id":"1","choices":[{"delta":{"content":"hi"}}]}`)
	_, _, err := p.TransformStreamChunk(context.Background(), data)
	// just shouldn't panic
	_ = err
}

func TestGetSupportedParams(t *testing.T) {
	p := testProvider()
	params := p.GetSupportedParams()
	assert.NotNil(t, params)
}

func TestGetSupportedParams_Custom(t *testing.T) {
	p := NewFromConfig(SimpleProviderConfig{
		Name:            "custom",
		BaseURL:         "https://test.example.com",
		SupportedParams: []string{"temperature", "max_tokens"},
	})
	params := p.GetSupportedParams()
	assert.Equal(t, []string{"temperature", "max_tokens"}, params)
}

func TestMapParams(t *testing.T) {
	p := NewFromConfig(SimpleProviderConfig{
		Name:    "custom",
		BaseURL: "https://test.example.com",
		ParamMappings: map[string]string{
			"max_tokens": "max_completion_tokens",
		},
	})
	result := p.MapParams(map[string]any{"temperature": 0.7, "max_tokens": 100})
	assert.Equal(t, 0.7, result["temperature"])
	assert.Equal(t, 100, result["max_completion_tokens"])
	assert.Nil(t, result["max_tokens"])
}

func TestChainedParamMappingsResolveToTerminalKey(t *testing.T) {
	p := NewFromConfig(SimpleProviderConfig{
		Name:    "chained",
		BaseURL: "https://test.example.com",
		ParamMappings: map[string]string{
			"max_completion_tokens": "max_tokens",
			"max_tokens":            "token_limit",
		},
	})

	mapped := p.MapParams(map[string]any{"max_completion_tokens": 123})
	assert.Equal(t, 123, mapped["token_limit"])
	assert.NotContains(t, mapped, "max_completion_tokens")
	assert.NotContains(t, mapped, "max_tokens")

	maxTokens := 123
	for range 100 {
		httpReq, err := p.TransformRequest(context.Background(), &model.ChatCompletionRequest{
			Model:               "test-model",
			Messages:            []model.Message{{Role: "user", Content: "hello"}},
			MaxCompletionTokens: &maxTokens,
		}, "mykey")
		require.NoError(t, err)

		var body map[string]any
		require.NoError(t, json.NewDecoder(httpReq.Body).Decode(&body))
		assert.Equal(t, float64(123), body["token_limit"])
		assert.NotContains(t, body, "max_completion_tokens")
		assert.NotContains(t, body, "max_tokens")
	}
}

func TestSetupHeaders(t *testing.T) {
	p := NewFromConfig(SimpleProviderConfig{
		Name:       "custom",
		BaseURL:    "https://test.example.com",
		AuthHeader: "x-api-key",
		AuthPrefix: "",
		Headers:    map[string]string{"X-Custom": "value"},
	})
	req, _ := http.NewRequest("POST", "https://test.example.com", nil)
	p.SetupHeaders(req, "mykey")
	assert.Equal(t, "Bearer mykey", req.Header.Get("X-Api-Key"))
	assert.Equal(t, "value", req.Header.Get("X-Custom"))
}

func TestGetRequestURL(t *testing.T) {
	p := testProvider()
	url := p.GetRequestURL("test-model")
	assert.Equal(t, "https://api.test.example.com/v1/chat/completions", url)
}
