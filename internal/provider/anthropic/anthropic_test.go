package anthropic

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

func TestTransformRequest_BasicMessages(t *testing.T) {
	p := New()
	ctx := context.Background()

	temp := 0.7
	maxTokens := 100
	req := &model.ChatCompletionRequest{
		Model: "claude-sonnet-4-5-20250929",
		Messages: []model.Message{
			{Role: "system", Content: "You are a helpful assistant."},
			{Role: "user", Content: "Hello"},
		},
		Temperature: &temp,
		MaxTokens:   &maxTokens,
	}

	httpReq, err := p.TransformRequest(ctx, req, "sk-ant-test")
	require.NoError(t, err)

	assert.Equal(t, "https://api.anthropic.com/v1/messages", httpReq.URL.String())
	assert.Equal(t, "sk-ant-test", httpReq.Header.Get("x-api-key"))
	assert.Equal(t, "2023-06-01", httpReq.Header.Get("anthropic-version"))

	body, _ := io.ReadAll(httpReq.Body)
	var parsed map[string]any
	require.NoError(t, json.Unmarshal(body, &parsed))

	// System messages separated
	system, ok := parsed["system"].([]any)
	require.True(t, ok)
	assert.Len(t, system, 1)

	// Messages should only contain user message
	messages, ok := parsed["messages"].([]any)
	require.True(t, ok)
	assert.Len(t, messages, 1)

	assert.Equal(t, 0.7, parsed["temperature"])
	assert.Equal(t, float64(100), parsed["max_tokens"])
}

func TestTransformRequest_WithTools(t *testing.T) {
	p := New()
	ctx := context.Background()

	req := &model.ChatCompletionRequest{
		Model: "claude-sonnet-4-5-20250929",
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

	httpReq, err := p.TransformRequest(ctx, req, "sk-ant-test")
	require.NoError(t, err)

	body, _ := io.ReadAll(httpReq.Body)
	var parsed map[string]any
	require.NoError(t, json.Unmarshal(body, &parsed))

	tools, ok := parsed["tools"].([]any)
	require.True(t, ok)
	assert.Len(t, tools, 1)

	tool, _ := tools[0].(map[string]any)
	assert.Equal(t, "get_weather", tool["name"])
	assert.Equal(t, "Get weather", tool["description"])

	toolChoice, _ := parsed["tool_choice"].(map[string]any)
	assert.Equal(t, "auto", toolChoice["type"])
}

func TestTransformResponse_TextContent(t *testing.T) {
	p := New()
	ctx := context.Background()

	anthropicResp := `{
		"id": "msg_01XFD",
		"type": "message",
		"role": "assistant",
		"content": [{"type": "text", "text": "Hello!"}],
		"model": "claude-sonnet-4-5-20250929",
		"stop_reason": "end_turn",
		"usage": {"input_tokens": 10, "output_tokens": 5}
	}`

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader([]byte(anthropicResp))),
	}

	result, err := p.TransformResponse(ctx, resp)
	require.NoError(t, err)

	assert.Equal(t, "msg_01XFD", result.ID)
	assert.Equal(t, "chat.completion", result.Object)
	assert.Equal(t, "claude-sonnet-4-5-20250929", result.Model)
	require.Len(t, result.Choices, 1)
	assert.Equal(t, "Hello!", result.Choices[0].Message.Content)
	assert.Equal(t, "stop", *result.Choices[0].FinishReason)
	assert.Equal(t, 10, result.Usage.PromptTokens)
	assert.Equal(t, 5, result.Usage.CompletionTokens)
	assert.Equal(t, 15, result.Usage.TotalTokens)
}

func TestTransformResponse_ToolUse(t *testing.T) {
	p := New()
	ctx := context.Background()

	anthropicResp := `{
		"id": "msg_01",
		"type": "message",
		"role": "assistant",
		"content": [
			{"type": "text", "text": "Let me check."},
			{"type": "tool_use", "id": "toolu_01", "name": "get_weather", "input": {"location": "SF"}}
		],
		"model": "claude-sonnet-4-5-20250929",
		"stop_reason": "tool_use",
		"usage": {"input_tokens": 20, "output_tokens": 30}
	}`

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader([]byte(anthropicResp))),
	}

	result, err := p.TransformResponse(ctx, resp)
	require.NoError(t, err)

	assert.Equal(t, "tool_calls", *result.Choices[0].FinishReason)
	assert.Equal(t, "Let me check.", result.Choices[0].Message.Content)
	require.Len(t, result.Choices[0].Message.ToolCalls, 1)

	tc := result.Choices[0].Message.ToolCalls[0]
	assert.Equal(t, "toolu_01", tc.ID)
	assert.Equal(t, "function", tc.Type)
	assert.Equal(t, "get_weather", tc.Function.Name)
	assert.Contains(t, tc.Function.Arguments, "SF")
}

func TestTransformResponse_Error(t *testing.T) {
	p := New()
	ctx := context.Background()

	errBody := `{"error":{"type":"authentication_error","message":"invalid api key"}}`
	resp := &http.Response{
		StatusCode: http.StatusUnauthorized,
		Body:       io.NopCloser(bytes.NewReader([]byte(errBody))),
	}

	_, err := p.TransformResponse(ctx, resp)
	require.Error(t, err)

	var tianjiErr *model.TianjiError
	require.ErrorAs(t, err, &tianjiErr)
	assert.Equal(t, 401, tianjiErr.StatusCode)
	assert.Equal(t, "anthropic", tianjiErr.Provider)
}

func TestTransformToolChoice(t *testing.T) {
	tests := []struct {
		input    any
		expected string
	}{
		{"auto", "auto"},
		{"required", "any"},
		{"none", "none"},
	}

	for _, tt := range tests {
		result := transformToolChoice(tt.input)
		assert.Equal(t, tt.expected, result["type"])
	}
}

func TestMapStopReason(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"end_turn", "stop"},
		{"max_tokens", "length"},
		{"tool_use", "tool_calls"},
		{"stop_sequence", "stop"},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.expected, mapStopReason(tt.input))
	}
}

func TestNewWithBaseURL(t *testing.T) {
	p := NewWithBaseURL("https://custom.anthropic.example.com")
	assert.NotNil(t, p)
	url := p.GetRequestURL("claude-3")
	assert.Contains(t, url, "custom.anthropic.example.com")
}

func TestGetSupportedParams(t *testing.T) {
	p := New()
	params := p.GetSupportedParams()
	assert.Contains(t, params, "max_tokens")
	assert.Contains(t, params, "temperature")
}

func TestMapParams(t *testing.T) {
	p := New()
	result := p.MapParams(map[string]any{
		"max_completion_tokens": 100,
		"temperature":           0.7,
	})
	assert.Equal(t, 100, result["max_tokens"])
	assert.Equal(t, 0.7, result["temperature"])
}

func TestTransformStreamChunk_MessageStop(t *testing.T) {
	p := New()
	data := []byte(`{"type":"message_stop"}`)
	_, done, err := p.TransformStreamChunk(context.Background(), data)
	assert.NoError(t, err)
	assert.True(t, done)
}
