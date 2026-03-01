package azure

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

func TestTransformRequest(t *testing.T) {
	p := NewWithConfig("https://myresource.openai.azure.com/openai/deployments/gpt-4o", "2024-10-21")
	ctx := context.Background()

	temp := 0.7
	req := &model.ChatCompletionRequest{
		Model: "gpt-4o",
		Messages: []model.Message{
			{Role: "user", Content: "Hello"},
		},
		Temperature: &temp,
	}

	httpReq, err := p.TransformRequest(ctx, req, "my-azure-key")
	require.NoError(t, err)

	assert.Contains(t, httpReq.URL.String(), "chat/completions")
	assert.Contains(t, httpReq.URL.String(), "api-version=2024-10-21")
	assert.Equal(t, "my-azure-key", httpReq.Header.Get("api-key"))
}

func TestTransformRequest_BearerToken(t *testing.T) {
	p := NewWithConfig("https://myresource.openai.azure.com/openai/deployments/gpt-4o", "2024-10-21")
	ctx := context.Background()

	req := &model.ChatCompletionRequest{
		Model: "gpt-4o",
		Messages: []model.Message{
			{Role: "user", Content: "Hello"},
		},
	}

	httpReq, err := p.TransformRequest(ctx, req, "Bearer ad-token-xyz")
	require.NoError(t, err)

	assert.Equal(t, "Bearer ad-token-xyz", httpReq.Header.Get("Authorization"))
	assert.Empty(t, httpReq.Header.Get("api-key"))
}

func TestTransformResponse(t *testing.T) {
	p := New()
	ctx := context.Background()

	respBody := `{
		"id": "chatcmpl-azure123",
		"object": "chat.completion",
		"created": 1700000000,
		"model": "gpt-4o",
		"choices": [{"index": 0, "message": {"role": "assistant", "content": "Hi!"}, "finish_reason": "stop"}],
		"usage": {"prompt_tokens": 5, "completion_tokens": 2, "total_tokens": 7}
	}`

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader([]byte(respBody))),
	}

	result, err := p.TransformResponse(ctx, resp)
	require.NoError(t, err)
	assert.Equal(t, "chatcmpl-azure123", result.ID)
	assert.Equal(t, "gpt-4o", result.Model)
}

func TestTransformResponse_Error(t *testing.T) {
	p := New()
	ctx := context.Background()

	errBody := `{"error":{"message":"invalid api key","type":"invalid_request_error","code":"401"}}`
	resp := &http.Response{
		StatusCode: http.StatusUnauthorized,
		Body:       io.NopCloser(bytes.NewReader([]byte(errBody))),
	}

	_, err := p.TransformResponse(ctx, resp)
	require.Error(t, err)

	var tianjiErr *model.TianjiError
	require.ErrorAs(t, err, &tianjiErr)
	assert.Equal(t, 401, tianjiErr.StatusCode)
	assert.Equal(t, "azure", tianjiErr.Provider)
}

func TestGetRequestURL_CustomBase(t *testing.T) {
	p := NewWithConfig("https://myresource.openai.azure.com/openai/deployments/gpt-4o", "2024-10-21")
	url := p.GetRequestURL("gpt-4o")
	assert.Contains(t, url, "chat/completions")
	assert.Contains(t, url, "api-version=2024-10-21")
}

func TestSetupHeaders_APIKey(t *testing.T) {
	p := New()
	req, _ := http.NewRequest(http.MethodPost, "http://test", nil)
	p.SetupHeaders(req, "test-key")
	assert.Equal(t, "test-key", req.Header.Get("api-key"))
}

func TestRequestBodyFormat(t *testing.T) {
	p := New()
	ctx := context.Background()

	temp := 0.5
	maxTokens := 50
	req := &model.ChatCompletionRequest{
		Model:       "gpt-4o",
		Messages:    []model.Message{{Role: "user", Content: "Test"}},
		Temperature: &temp,
		MaxTokens:   &maxTokens,
	}

	httpReq, err := p.TransformRequest(ctx, req, "key")
	require.NoError(t, err)

	body, _ := io.ReadAll(httpReq.Body)
	var parsed map[string]any
	require.NoError(t, json.Unmarshal(body, &parsed))

	// Azure does NOT include model in body (it's in the URL)
	assert.NotContains(t, parsed, "model")
	assert.Equal(t, 0.5, parsed["temperature"])
	assert.Equal(t, float64(50), parsed["max_tokens"])
}

func TestTransformStreamChunk(t *testing.T) {
	p := New()
	data := []byte(`data: {"id":"1","choices":[{"delta":{"content":"hi"}}]}`)
	_, _, err := p.TransformStreamChunk(context.Background(), data)
	_ = err
}

func TestGetSupportedParams(t *testing.T) {
	p := New()
	params := p.GetSupportedParams()
	assert.NotNil(t, params)
}

func TestMapParams(t *testing.T) {
	p := New()
	result := p.MapParams(map[string]any{"temperature": 0.7})
	assert.NotNil(t, result)
}
