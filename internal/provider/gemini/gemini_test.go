package gemini

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

func TestTransformRequest_BasicMessage(t *testing.T) {
	p := New()
	ctx := context.Background()

	temp := 0.7
	maxTokens := 100
	req := &model.ChatCompletionRequest{
		Model: "gemini-2.0-flash",
		Messages: []model.Message{
			{Role: "user", Content: "Hello"},
		},
		Temperature: &temp,
		MaxTokens:   &maxTokens,
	}

	httpReq, err := p.TransformRequest(ctx, req, "gemini-api-key")
	require.NoError(t, err)

	assert.Contains(t, httpReq.URL.String(), "gemini-2.0-flash")
	assert.Contains(t, httpReq.URL.String(), "generateContent")
	assert.Contains(t, httpReq.URL.String(), "key=gemini-api-key")

	body, _ := io.ReadAll(httpReq.Body)
	var parsed map[string]any
	require.NoError(t, json.Unmarshal(body, &parsed))

	contents, ok := parsed["contents"].([]any)
	require.True(t, ok)
	assert.Len(t, contents, 1)

	genConfig, _ := parsed["generationConfig"].(map[string]any)
	assert.Equal(t, 0.7, genConfig["temperature"])
	assert.Equal(t, float64(100), genConfig["maxOutputTokens"])
}

func TestTransformRequest_WithTools(t *testing.T) {
	p := New()
	ctx := context.Background()

	req := &model.ChatCompletionRequest{
		Model: "gemini-2.0-flash",
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
	}

	httpReq, err := p.TransformRequest(ctx, req, "key")
	require.NoError(t, err)

	body, _ := io.ReadAll(httpReq.Body)
	var parsed map[string]any
	require.NoError(t, json.Unmarshal(body, &parsed))

	tools, ok := parsed["tools"].([]any)
	require.True(t, ok)
	assert.Len(t, tools, 1)
}

func TestTransformResponse(t *testing.T) {
	p := New()
	ctx := context.Background()

	geminiResp := `{
		"candidates": [{
			"content": {
				"parts": [{"text": "Hello!"}],
				"role": "model"
			},
			"finishReason": "STOP"
		}],
		"usageMetadata": {
			"promptTokenCount": 10,
			"candidatesTokenCount": 5,
			"totalTokenCount": 15
		}
	}`

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader([]byte(geminiResp))),
	}

	result, err := p.TransformResponse(ctx, resp)
	require.NoError(t, err)

	require.Len(t, result.Choices, 1)
	assert.Equal(t, "Hello!", result.Choices[0].Message.Content)
	assert.Equal(t, "stop", *result.Choices[0].FinishReason)
	assert.Equal(t, 10, result.Usage.PromptTokens)
	assert.Equal(t, 5, result.Usage.CompletionTokens)
}

func TestTransformResponse_ToolCall(t *testing.T) {
	p := New()
	ctx := context.Background()

	geminiResp := `{
		"candidates": [{
			"content": {
				"parts": [{"functionCall": {"name": "get_weather", "args": {"location": "SF"}}}],
				"role": "model"
			},
			"finishReason": "TOOL_CALLS"
		}],
		"usageMetadata": {"promptTokenCount": 10, "candidatesTokenCount": 5, "totalTokenCount": 15}
	}`

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader([]byte(geminiResp))),
	}

	result, err := p.TransformResponse(ctx, resp)
	require.NoError(t, err)

	require.Len(t, result.Choices[0].Message.ToolCalls, 1)
	assert.Equal(t, "get_weather", result.Choices[0].Message.ToolCalls[0].Function.Name)
	assert.Equal(t, "tool_calls", *result.Choices[0].FinishReason)
}

func TestMapRole(t *testing.T) {
	assert.Equal(t, "model", mapRole("assistant"))
	assert.Equal(t, "user", mapRole("user"))
	assert.Equal(t, "user", mapRole("system"))
}

func TestMapFinishReason(t *testing.T) {
	assert.Equal(t, "stop", mapFinishReason("STOP"))
	assert.Equal(t, "length", mapFinishReason("MAX_TOKENS"))
	assert.Equal(t, "content_filter", mapFinishReason("SAFETY"))
	assert.Equal(t, "tool_calls", mapFinishReason("TOOL_CALLS"))
}

func TestStreamRequest_URL(t *testing.T) {
	p := New()
	ctx := context.Background()

	stream := true
	req := &model.ChatCompletionRequest{
		Model: "gemini-2.0-flash",
		Messages: []model.Message{
			{Role: "user", Content: "Hello"},
		},
		Stream: &stream,
	}

	httpReq, err := p.TransformRequest(ctx, req, "key")
	require.NoError(t, err)
	assert.Contains(t, httpReq.URL.String(), "streamGenerateContent")
}
