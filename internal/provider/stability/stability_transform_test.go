package stability

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
	p := &Provider{baseURL: "https://api.stability.ai"}
	req := &model.ChatCompletionRequest{
		Model:    "stable-diffusion-xl-1024-v1-0",
		Messages: []model.Message{{Role: "user", Content: "a futuristic city"}},
	}
	httpReq, err := p.TransformRequest(context.Background(), req, "sk-test")
	require.NoError(t, err)
	assert.Contains(t, httpReq.URL.String(), "stable-diffusion-xl-1024-v1-0")
	assert.Contains(t, httpReq.URL.String(), "text-to-image")
	assert.Equal(t, "Bearer sk-test", httpReq.Header.Get("Authorization"))
	assert.Equal(t, "application/json", httpReq.Header.Get("Accept"))

	var body map[string]any
	require.NoError(t, json.NewDecoder(httpReq.Body).Decode(&body))
	prompts, ok := body["text_prompts"].([]any)
	require.True(t, ok)
	require.Len(t, prompts, 1)
	p0 := prompts[0].(map[string]any)
	assert.Equal(t, "a futuristic city", p0["text"])
}

func TestTransformRequest_NoMessages(t *testing.T) {
	p := &Provider{baseURL: "https://api.stability.ai"}
	req := &model.ChatCompletionRequest{Model: "stable-diffusion-v1-5", Messages: []model.Message{}}
	httpReq, err := p.TransformRequest(context.Background(), req, "key")
	require.NoError(t, err)
	assert.NotNil(t, httpReq)
}

func TestTransformResponse_Success(t *testing.T) {
	p := &Provider{}
	stabResp := map[string]any{
		"artifacts": []map[string]any{
			{"base64": "AAABBBCCC", "finishReason": "SUCCESS", "seed": 12345},
		},
	}
	body, _ := json.Marshal(stabResp)
	resp := &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(body))}
	result, err := p.TransformResponse(context.Background(), resp)
	require.NoError(t, err)
	require.Len(t, result.Choices, 1)
	assert.Contains(t, result.Choices[0].Message.Content.(string), "image:base64")
}

func TestTransformResponse_NoArtifacts(t *testing.T) {
	p := &Provider{}
	stabResp := map[string]any{"artifacts": []any{}}
	body, _ := json.Marshal(stabResp)
	resp := &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(body))}
	result, err := p.TransformResponse(context.Background(), resp)
	require.NoError(t, err)
	require.Len(t, result.Choices, 1)
	assert.Equal(t, "", result.Choices[0].Message.Content)
}

func TestTransformResponse_Error(t *testing.T) {
	p := &Provider{}
	resp := &http.Response{StatusCode: http.StatusBadRequest, Body: io.NopCloser(bytes.NewReader([]byte("bad request")))}
	_, err := p.TransformResponse(context.Background(), resp)
	require.Error(t, err)
	var tianjiErr *model.TianjiError
	require.ErrorAs(t, err, &tianjiErr)
	assert.Equal(t, "stability", tianjiErr.Provider)
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
	in := map[string]any{"n": 1, "size": "512x512"}
	assert.Equal(t, in, p.MapParams(in))
}

func TestSetupHeaders(t *testing.T) {
	p := &Provider{}
	req, _ := http.NewRequest(http.MethodPost, "https://example.com", nil)
	p.SetupHeaders(req, "stab-key")
	assert.Equal(t, "Bearer stab-key", req.Header.Get("Authorization"))
	assert.Equal(t, "application/json", req.Header.Get("Content-Type"))
	assert.Equal(t, "application/json", req.Header.Get("Accept"))
}
