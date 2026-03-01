package falai

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
	p := &Provider{baseURL: "https://fal.run"}
	req := &model.ChatCompletionRequest{
		Model:    "fal-ai/fast-sdxl",
		Messages: []model.Message{{Role: "user", Content: "a beautiful sunset"}},
	}
	httpReq, err := p.TransformRequest(context.Background(), req, "test-key")
	require.NoError(t, err)
	assert.Equal(t, "https://fal.run/fal-ai/fast-sdxl", httpReq.URL.String())
	assert.Equal(t, "Key test-key", httpReq.Header.Get("Authorization"))
	assert.Equal(t, "application/json", httpReq.Header.Get("Content-Type"))
	var body map[string]any
	require.NoError(t, json.NewDecoder(httpReq.Body).Decode(&body))
	assert.Equal(t, "a beautiful sunset", body["prompt"])
}

func TestTransformRequest_NoMessages(t *testing.T) {
	p := &Provider{baseURL: "https://fal.run"}
	req := &model.ChatCompletionRequest{Model: "fal-ai/fast-sdxl", Messages: []model.Message{}}
	httpReq, err := p.TransformRequest(context.Background(), req, "key")
	require.NoError(t, err)
	assert.NotNil(t, httpReq)
}

func TestTransformResponse_Success(t *testing.T) {
	p := &Provider{baseURL: "https://fal.run"}
	falResp := map[string]any{
		"images": []map[string]any{
			{"url": "https://example.com/image.png", "content_type": "image/png"},
		},
		"request_id": "req_123",
	}
	body, _ := json.Marshal(falResp)
	resp := &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(body))}
	result, err := p.TransformResponse(context.Background(), resp)
	require.NoError(t, err)
	require.Len(t, result.Choices, 1)
	assert.Equal(t, "https://example.com/image.png", result.Choices[0].Message.Content)
	assert.Equal(t, "assistant", result.Choices[0].Message.Role)
}

func TestTransformResponse_NoImages(t *testing.T) {
	p := &Provider{baseURL: "https://fal.run"}
	falResp := map[string]any{"images": []any{}, "request_id": "req_456"}
	body, _ := json.Marshal(falResp)
	resp := &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(body))}
	result, err := p.TransformResponse(context.Background(), resp)
	require.NoError(t, err)
	require.Len(t, result.Choices, 1)
	assert.Equal(t, "", result.Choices[0].Message.Content)
}

func TestTransformResponse_ErrorStatus(t *testing.T) {
	p := &Provider{baseURL: "https://fal.run"}
	resp := &http.Response{StatusCode: http.StatusUnauthorized, Body: io.NopCloser(bytes.NewReader([]byte("unauthorized")))}
	_, err := p.TransformResponse(context.Background(), resp)
	require.Error(t, err)
	var tianjiErr *model.TianjiError
	require.ErrorAs(t, err, &tianjiErr)
	assert.Equal(t, 401, tianjiErr.StatusCode)
	assert.Equal(t, "fal_ai", tianjiErr.Provider)
}

func TestTransformStreamChunk(t *testing.T) {
	p := &Provider{}
	chunk, done, err := p.TransformStreamChunk(context.Background(), []byte("data"))
	assert.Nil(t, chunk)
	assert.True(t, done)
	assert.Error(t, err)
}

func TestMapParams(t *testing.T) {
	p := &Provider{}
	input := map[string]any{"model": "test", "n": 1}
	result := p.MapParams(input)
	assert.Equal(t, input, result)
}

func TestSetupHeaders(t *testing.T) {
	p := &Provider{}
	req, _ := http.NewRequest(http.MethodPost, "https://example.com", nil)
	p.SetupHeaders(req, "my-api-key")
	assert.Equal(t, "Key my-api-key", req.Header.Get("Authorization"))
	assert.Equal(t, "application/json", req.Header.Get("Content-Type"))
}
