package bedrock

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
		Model: "anthropic.claude-v2",
		Messages: []model.Message{
			{Role: "system", Content: "You are helpful."},
			{Role: "user", Content: "Hello"},
		},
		Temperature: &temp,
		MaxTokens:   &maxTokens,
	}

	httpReq, err := p.TransformRequest(ctx, req, "")
	require.NoError(t, err)

	assert.Contains(t, httpReq.URL.String(), "anthropic.claude-v2")
	assert.Contains(t, httpReq.URL.String(), "converse")

	body, _ := io.ReadAll(httpReq.Body)
	var parsed map[string]any
	require.NoError(t, json.Unmarshal(body, &parsed))

	// System messages separated
	system, ok := parsed["system"].([]any)
	require.True(t, ok)
	assert.Len(t, system, 1)

	// Only user message in messages
	messages, ok := parsed["messages"].([]any)
	require.True(t, ok)
	assert.Len(t, messages, 1)

	inferenceConfig, _ := parsed["inferenceConfig"].(map[string]any)
	assert.Equal(t, 0.7, inferenceConfig["temperature"])
	assert.Equal(t, float64(100), inferenceConfig["maxTokens"])
}

func TestTransformResponse_TextContent(t *testing.T) {
	p := New()
	ctx := context.Background()

	bedrockResp := `{
		"output": {
			"message": {
				"role": "assistant",
				"content": [{"text": "Hello!"}]
			}
		},
		"stopReason": "end_turn",
		"usage": {"inputTokens": 10, "outputTokens": 5, "totalTokens": 15}
	}`

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader([]byte(bedrockResp))),
	}

	result, err := p.TransformResponse(ctx, resp)
	require.NoError(t, err)

	require.Len(t, result.Choices, 1)
	assert.Equal(t, "Hello!", result.Choices[0].Message.Content)
	assert.Equal(t, "stop", *result.Choices[0].FinishReason)
	assert.Equal(t, 10, result.Usage.PromptTokens)
	assert.Equal(t, 5, result.Usage.CompletionTokens)
}

func TestTransformResponse_ToolUse(t *testing.T) {
	p := New()
	ctx := context.Background()

	bedrockResp := `{
		"output": {
			"message": {
				"role": "assistant",
				"content": [
					{"toolUse": {"toolUseId": "tool_01", "name": "get_weather", "input": {"location": "SF"}}}
				]
			}
		},
		"stopReason": "tool_use",
		"usage": {"inputTokens": 20, "outputTokens": 10, "totalTokens": 30}
	}`

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader([]byte(bedrockResp))),
	}

	result, err := p.TransformResponse(ctx, resp)
	require.NoError(t, err)

	assert.Equal(t, "tool_calls", *result.Choices[0].FinishReason)
	require.Len(t, result.Choices[0].Message.ToolCalls, 1)
	assert.Equal(t, "get_weather", result.Choices[0].Message.ToolCalls[0].Function.Name)
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

func TestGetRequestURL(t *testing.T) {
	p := NewWithRegion("us-west-2")
	url := p.GetRequestURL("anthropic.claude-v2")
	assert.Contains(t, url, "us-west-2")
	assert.Contains(t, url, "anthropic.claude-v2")
	assert.Contains(t, url, "converse")
}
