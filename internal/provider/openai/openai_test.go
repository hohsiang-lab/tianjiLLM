package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func loadFixture(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err, "failed to load fixture: %s", path)
	return data
}

func TestTransformRequest(t *testing.T) {
	p := New()
	ctx := context.Background()

	temp := 0.7
	maxTokens := 100
	stream := false
	req := &model.ChatCompletionRequest{
		Model: "gpt-4o",
		Messages: []model.Message{
			{Role: "system", Content: "You are a helpful assistant."},
			{Role: "user", Content: "Hello, how are you?"},
		},
		Temperature: &temp,
		MaxTokens:   &maxTokens,
		Stream:      &stream,
	}

	httpReq, err := p.TransformRequest(ctx, req, "sk-test-key")
	require.NoError(t, err)

	assert.Equal(t, http.MethodPost, httpReq.Method)
	assert.Equal(t, "https://api.openai.com/v1/chat/completions", httpReq.URL.String())
	assert.Equal(t, "Bearer sk-test-key", httpReq.Header.Get("Authorization"))
	assert.Equal(t, "application/json", httpReq.Header.Get("Content-Type"))

	body, err := io.ReadAll(httpReq.Body)
	require.NoError(t, err)

	var parsed map[string]any
	require.NoError(t, json.Unmarshal(body, &parsed))
	assert.Equal(t, "gpt-4o", parsed["model"])
	assert.Equal(t, 0.7, parsed["temperature"])
	assert.Equal(t, float64(100), parsed["max_tokens"])
	assert.Equal(t, false, parsed["stream"])
}

func TestTransformRequest_WithTools(t *testing.T) {
	p := New()
	ctx := context.Background()

	req := &model.ChatCompletionRequest{
		Model: "gpt-4o",
		Messages: []model.Message{
			{Role: "user", Content: "What's the weather?"},
		},
		Tools: []model.Tool{
			{
				Type: "function",
				Function: model.ToolFunction{
					Name:        "get_weather",
					Description: "Get weather",
					Parameters:  map[string]any{"type": "object"},
				},
			},
		},
		ToolChoice: "auto",
	}

	httpReq, err := p.TransformRequest(ctx, req, "sk-test")
	require.NoError(t, err)

	body, _ := io.ReadAll(httpReq.Body)
	var parsed map[string]any
	require.NoError(t, json.Unmarshal(body, &parsed))

	tools, ok := parsed["tools"].([]any)
	require.True(t, ok)
	assert.Len(t, tools, 1)
	assert.Equal(t, "auto", parsed["tool_choice"])
}

func TestTransformResponse(t *testing.T) {
	p := New()
	ctx := context.Background()
	fixture := loadFixture(t, "../../../test/fixtures/openai/chat_completion_response.json")

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader(fixture)),
		Header:     http.Header{},
	}

	result, err := p.TransformResponse(ctx, resp)
	require.NoError(t, err)

	assert.Equal(t, "chatcmpl-abc123", result.ID)
	assert.Equal(t, "chat.completion", result.Object)
	assert.Equal(t, "gpt-4o-2024-08-06", result.Model)
	require.Len(t, result.Choices, 1)
	assert.Equal(t, "assistant", result.Choices[0].Message.Role)
	assert.Equal(t, 20, result.Usage.PromptTokens)
	assert.Equal(t, 10, result.Usage.CompletionTokens)
	assert.Equal(t, 30, result.Usage.TotalTokens)
}

func TestTransformResponse_Error(t *testing.T) {
	p := New()
	ctx := context.Background()

	errorBody := `{"error":{"message":"invalid api key","type":"authentication_error","code":"invalid_api_key"}}`
	resp := &http.Response{
		StatusCode: http.StatusUnauthorized,
		Body:       io.NopCloser(bytes.NewReader([]byte(errorBody))),
		Header:     http.Header{},
	}

	_, err := p.TransformResponse(ctx, resp)
	require.Error(t, err)

	var tianjiErr *model.TianjiError
	require.ErrorAs(t, err, &tianjiErr)
	assert.Equal(t, 401, tianjiErr.StatusCode)
	assert.Equal(t, "openai", tianjiErr.Provider)
}

func TestTransformRequest_CustomBaseURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	p := NewWithBaseURL(server.URL)
	assert.Equal(t, server.URL+"/chat/completions", p.GetRequestURL("gpt-4o"))
}

func TestTransformResponse_ImagesPassthrough(t *testing.T) {
	// Simulate OpenRouter response with images[] field
	responseBody := `{
		"id": "gen-abc123",
		"object": "chat.completion",
		"created": 1700000000,
		"model": "google/gemini-3-pro-image-preview",
		"choices": [{
			"index": 0,
			"message": {
				"role": "assistant",
				"content": "Here is your red circle",
				"images": [{
					"image_url": {
						"url": "data:image/png;base64,iVBORw0KGgoAAAANSUhEUg=="
					}
				}]
			},
			"finish_reason": "stop"
		}],
		"usage": {"prompt_tokens": 10, "completion_tokens": 125, "total_tokens": 135}
	}`

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewBufferString(responseBody)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}

	p := New()
	result, err := p.TransformResponse(context.Background(), resp)
	require.NoError(t, err)
	require.Len(t, result.Choices, 1)

	msg := result.Choices[0].Message
	require.NotNil(t, msg)
	assert.Equal(t, "Here is your red circle", msg.Content)
	require.Len(t, msg.Images, 1, "images should be preserved in passthrough")
	assert.Equal(t, "data:image/png;base64,iVBORw0KGgoAAAANSUhEUg==", msg.Images[0].ImageURL.URL)
}

func TestTransformRequest_ModalitiesPassthrough(t *testing.T) {
	p := New()
	ctx := context.Background()

	req := &model.ChatCompletionRequest{
		Model:      "openrouter/google/gemini-3-pro-image-preview",
		Modalities: []string{"text", "image"},
		Messages: []model.Message{
			{Role: "user", Content: "Draw a circle"},
		},
	}

	httpReq, err := p.TransformRequest(ctx, req, "sk-test")
	require.NoError(t, err)

	body, _ := io.ReadAll(httpReq.Body)
	var parsed map[string]any
	json.Unmarshal(body, &parsed)

	modalities, ok := parsed["modalities"].([]any)
	require.True(t, ok, "modalities should be in request body")
	assert.Equal(t, "text", modalities[0])
	assert.Equal(t, "image", modalities[1])
}

func TestTransformResponse_ImagesWithEmptyContent(t *testing.T) {
	responseBody := `{
		"id": "gen-001",
		"object": "chat.completion",
		"created": 1700000000,
		"model": "google/gemini-3-pro-image-preview",
		"choices": [{
			"index": 0,
			"message": {
				"role": "assistant",
				"content": "",
				"images": [{
					"image_url": {"url": "data:image/png;base64,abc123"}
				}]
			},
			"finish_reason": "stop"
		}],
		"usage": {"prompt_tokens": 10, "completion_tokens": 50, "total_tokens": 60}
	}`
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewBufferString(responseBody)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}
	p := New()
	result, err := p.TransformResponse(context.Background(), resp)
	require.NoError(t, err)
	msg := result.Choices[0].Message
	assert.Equal(t, "", msg.Content)
	require.Len(t, msg.Images, 1, "images preserved even with empty content")
	assert.Equal(t, "data:image/png;base64,abc123", msg.Images[0].ImageURL.URL)
}

func TestTransformResponse_MultipleImages(t *testing.T) {
	responseBody := `{
		"id": "gen-002",
		"object": "chat.completion",
		"created": 1700000000,
		"model": "flux-pro",
		"choices": [{
			"index": 0,
			"message": {
				"role": "assistant",
				"content": "Here are 2 images",
				"images": [
					{"image_url": {"url": "data:image/png;base64,img1"}},
					{"image_url": {"url": "data:image/jpeg;base64,img2"}}
				]
			},
			"finish_reason": "stop"
		}],
		"usage": {"prompt_tokens": 5, "completion_tokens": 200, "total_tokens": 205}
	}`
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewBufferString(responseBody)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}
	p := New()
	result, err := p.TransformResponse(context.Background(), resp)
	require.NoError(t, err)
	msg := result.Choices[0].Message
	require.Len(t, msg.Images, 2, "all images preserved")
	assert.Equal(t, "data:image/png;base64,img1", msg.Images[0].ImageURL.URL)
	assert.Equal(t, "data:image/jpeg;base64,img2", msg.Images[1].ImageURL.URL)
}

func TestTransformResponse_NoImages_TextOnly(t *testing.T) {
	responseBody := `{
		"id": "gen-003",
		"object": "chat.completion",
		"created": 1700000000,
		"model": "gpt-4",
		"choices": [{
			"index": 0,
			"message": {
				"role": "assistant",
				"content": "Hello world"
			},
			"finish_reason": "stop"
		}],
		"usage": {"prompt_tokens": 5, "completion_tokens": 2, "total_tokens": 7}
	}`
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewBufferString(responseBody)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}
	p := New()
	result, err := p.TransformResponse(context.Background(), resp)
	require.NoError(t, err)
	msg := result.Choices[0].Message
	assert.Equal(t, "Hello world", msg.Content)
	assert.Empty(t, msg.Images, "no images for text-only response")
}
