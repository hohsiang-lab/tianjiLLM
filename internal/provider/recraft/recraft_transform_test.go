package recraft

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
	p := &Provider{baseURL: "https://external.api.recraft.ai/v1"}
	req := &model.ChatCompletionRequest{
		Model:    "recraft-v3",
		Messages: []model.Message{{Role: "user", Content: "a mountain landscape"}},
	}
	httpReq, err := p.TransformRequest(context.Background(), req, "test-token")
	require.NoError(t, err)
	assert.Contains(t, httpReq.URL.String(), "/images/generations")
	assert.Equal(t, "Bearer test-token", httpReq.Header.Get("Authorization"))
	var body map[string]any
	require.NoError(t, json.NewDecoder(httpReq.Body).Decode(&body))
	assert.Equal(t, "a mountain landscape", body["prompt"])
}

func TestTransformRequest_NoMessages(t *testing.T) {
	p := &Provider{baseURL: "https://external.api.recraft.ai/v1"}
	req := &model.ChatCompletionRequest{Model: "recraft-v3", Messages: []model.Message{}}
	httpReq, err := p.TransformRequest(context.Background(), req, "key")
	require.NoError(t, err)
	assert.NotNil(t, httpReq)
}

func TestTransformResponse_WithURL(t *testing.T) {
	p := &Provider{baseURL: "https://external.api.recraft.ai/v1"}
	rcResp := map[string]any{
		"data": []map[string]any{{"url": "https://cdn.recraft.ai/img.png", "b64_json": ""}},
	}
	body, _ := json.Marshal(rcResp)
	resp := &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(body))}
	result, err := p.TransformResponse(context.Background(), resp)
	require.NoError(t, err)
	require.Len(t, result.Choices, 1)
	assert.Equal(t, "https://cdn.recraft.ai/img.png", result.Choices[0].Message.Content)
}

func TestTransformResponse_WithB64(t *testing.T) {
	p := &Provider{baseURL: "https://external.api.recraft.ai/v1"}
	rcResp := map[string]any{
		"data": []map[string]any{{"url": "", "b64_json": "AAABBBCCC"}},
	}
	body, _ := json.Marshal(rcResp)
	resp := &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(body))}
	result, err := p.TransformResponse(context.Background(), resp)
	require.NoError(t, err)
	require.Len(t, result.Choices, 1)
	assert.Contains(t, result.Choices[0].Message.Content.(string), "image:base64")
}

func TestTransformResponse_Empty(t *testing.T) {
	p := &Provider{baseURL: "https://external.api.recraft.ai/v1"}
	rcResp := map[string]any{"data": []any{}}
	body, _ := json.Marshal(rcResp)
	resp := &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(body))}
	result, err := p.TransformResponse(context.Background(), resp)
	require.NoError(t, err)
	require.Len(t, result.Choices, 1)
	assert.Equal(t, "", result.Choices[0].Message.Content)
}

func TestTransformResponse_Error(t *testing.T) {
	p := &Provider{}
	resp := &http.Response{StatusCode: http.StatusForbidden, Body: io.NopCloser(bytes.NewReader([]byte("forbidden")))}
	_, err := p.TransformResponse(context.Background(), resp)
	require.Error(t, err)
	var tianjiErr *model.TianjiError
	require.ErrorAs(t, err, &tianjiErr)
	assert.Equal(t, "recraft", tianjiErr.Provider)
}

func TestTransformStreamChunk(t *testing.T) {
	p := &Provider{}
	chunk, done, err := p.TransformStreamChunk(context.Background(), nil)
	assert.Nil(t, chunk)
	assert.True(t, done)
	assert.Error(t, err)
}

func TestMapParams(t *testing.T) {
	p := &Provider{}
	in := map[string]any{"size": "1024x1024"}
	assert.Equal(t, in, p.MapParams(in))
}

func TestSetupHeaders(t *testing.T) {
	p := &Provider{}
	req, _ := http.NewRequest(http.MethodPost, "https://example.com", nil)
	p.SetupHeaders(req, "recraft-key")
	assert.Equal(t, "Bearer recraft-key", req.Header.Get("Authorization"))
	assert.Equal(t, "application/json", req.Header.Get("Content-Type"))
}
