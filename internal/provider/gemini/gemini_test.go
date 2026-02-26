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

// === Image generation tests ===

func TestTransformResponse_ImageOutput(t *testing.T) {
	resp := &geminiResponse{
		Candidates: []geminiCandidate{{
			Content: geminiContent{
				Parts: []geminiPart{{
					InlineData: &geminiInlineData{MimeType: "image/png", Data: "iVBORw0KGgo="},
				}},
				Role: "model",
			},
			FinishReason: "STOP",
		}},
	}
	result := transformToOpenAI(resp)
	require.Len(t, result.Choices, 1)
	parts, ok := result.Choices[0].Message.Content.([]model.ContentPart)
	require.True(t, ok, "expected []ContentPart")
	require.Len(t, parts, 1)
	assert.Equal(t, "image_url", parts[0].Type)
	assert.Equal(t, "data:image/png;base64,iVBORw0KGgo=", parts[0].ImageURL.URL)
}

func TestTransformResponse_MixedContent(t *testing.T) {
	resp := &geminiResponse{
		Candidates: []geminiCandidate{{
			Content: geminiContent{
				Parts: []geminiPart{
					{Text: "Here is the image:"},
					{InlineData: &geminiInlineData{MimeType: "image/png", Data: "abc123"}},
					{Text: "Done."},
				},
				Role: "model",
			},
			FinishReason: "STOP",
		}},
	}
	result := transformToOpenAI(resp)
	parts, ok := result.Choices[0].Message.Content.([]model.ContentPart)
	require.True(t, ok)
	require.Len(t, parts, 3)
	assert.Equal(t, "text", parts[0].Type)
	assert.Equal(t, "Here is the image:", parts[0].Text)
	assert.Equal(t, "image_url", parts[1].Type)
	assert.Equal(t, "data:image/png;base64,abc123", parts[1].ImageURL.URL)
	assert.Equal(t, "text", parts[2].Type)
	assert.Equal(t, "Done.", parts[2].Text)
}

func TestTransformRequest_WithModalities(t *testing.T) {
	p := New()
	ctx := context.Background()
	req := &model.ChatCompletionRequest{
		Model:      "gemini-2.0-flash",
		Messages:   []model.Message{{Role: "user", Content: "Draw a cat"}},
		Modalities: []string{"text", "image"},
	}
	httpReq, err := p.TransformRequest(ctx, req, "key")
	require.NoError(t, err)
	body, _ := io.ReadAll(httpReq.Body)
	var parsed map[string]any
	require.NoError(t, json.Unmarshal(body, &parsed))
	gc := parsed["generationConfig"].(map[string]any)
	mods := gc["responseModalities"].([]any)
	assert.Equal(t, []any{"TEXT", "IMAGE"}, mods)
}

func TestTransformResponse_TextOnlyBackwardCompat(t *testing.T) {
	resp := &geminiResponse{
		Candidates: []geminiCandidate{{
			Content: geminiContent{
				Parts: []geminiPart{{Text: "Hello world"}},
				Role:  "model",
			},
			FinishReason: "STOP",
		}},
	}
	result := transformToOpenAI(resp)
	content, ok := result.Choices[0].Message.Content.(string)
	require.True(t, ok, "text-only should be plain string")
	assert.Equal(t, "Hello world", content)
}

func TestStreamParseChunk_ImageInlineData(t *testing.T) {
	data := `{"candidates":[{"content":{"parts":[{"inlineData":{"mimeType":"image/png","data":"AAAA"}}],"role":"model"},"finishReason":"STOP"}]}`
	chunk, isDone, err := ParseStreamChunk([]byte(data))
	require.NoError(t, err)
	assert.True(t, isDone)
	require.Len(t, chunk.Choices, 1)
	require.Len(t, chunk.Choices[0].Delta.ContentParts, 1)
	assert.Equal(t, "image_url", chunk.Choices[0].Delta.ContentParts[0].Type)
	assert.Equal(t, "data:image/png;base64,AAAA", chunk.Choices[0].Delta.ContentParts[0].ImageURL.URL)
	assert.Nil(t, chunk.Choices[0].Delta.Content)
}

func TestTransformContentPart_DataURLParsing(t *testing.T) {
	part := map[string]any{
		"type": "image_url",
		"image_url": map[string]any{
			"url": "data:image/webp;base64,UklGR...",
		},
	}
	result := transformContentPart(part)
	inlineData := result["inlineData"].(map[string]any)
	assert.Equal(t, "image/webp", inlineData["mimeType"])
	assert.Equal(t, "UklGR...", inlineData["data"])
}

func TestTransformRequest_ModalitiesTextOnly(t *testing.T) {
	p := New()
	ctx := context.Background()
	req := &model.ChatCompletionRequest{
		Model:      "gemini-2.0-flash",
		Messages:   []model.Message{{Role: "user", Content: "Hello"}},
		Modalities: []string{"text"},
	}
	httpReq, err := p.TransformRequest(ctx, req, "key")
	require.NoError(t, err)
	body, _ := io.ReadAll(httpReq.Body)
	var parsed map[string]any
	require.NoError(t, json.Unmarshal(body, &parsed))
	gc, _ := parsed["generationConfig"].(map[string]any)
	if gc != nil {
		assert.Nil(t, gc["responseModalities"], "text-only should not set responseModalities")
	}
}

func TestTransformRequest_ModalitiesEmpty(t *testing.T) {
	p := New()
	ctx := context.Background()
	req := &model.ChatCompletionRequest{
		Model:      "gemini-2.0-flash",
		Messages:   []model.Message{{Role: "user", Content: "Hello"}},
		Modalities: []string{},
	}
	httpReq, err := p.TransformRequest(ctx, req, "key")
	require.NoError(t, err)
	body, _ := io.ReadAll(httpReq.Body)
	var parsed map[string]any
	require.NoError(t, json.Unmarshal(body, &parsed))
	assert.Nil(t, parsed["generationConfig"], "empty modalities + no other config should not create generationConfig")
}

func TestTransformResponse_UnknownMimeType(t *testing.T) {
	resp := &geminiResponse{
		Candidates: []geminiCandidate{{
			Content: geminiContent{
				Parts: []geminiPart{{
					InlineData: &geminiInlineData{MimeType: "audio/wav", Data: "RIFF"},
				}},
				Role: "model",
			},
			FinishReason: "STOP",
		}},
	}
	result := transformToOpenAI(resp)
	parts, ok := result.Choices[0].Message.Content.([]model.ContentPart)
	require.True(t, ok)
	assert.Equal(t, "data:audio/wav;base64,RIFF", parts[0].ImageURL.URL)
}

func TestTransformResponse_ImageInputRoundTrip(t *testing.T) {
	// Test that image input (data URL) is correctly parsed and could round-trip
	inputPart := map[string]any{
		"type": "image_url",
		"image_url": map[string]any{
			"url": "data:image/jpeg;base64,/9j/4AAQ",
		},
	}
	transformed := transformContentPart(inputPart)
	inlineData := transformed["inlineData"].(map[string]any)
	assert.Equal(t, "image/jpeg", inlineData["mimeType"])
	assert.Equal(t, "/9j/4AAQ", inlineData["data"])

	// Simulate Gemini returning an image
	resp := &geminiResponse{
		Candidates: []geminiCandidate{{
			Content: geminiContent{
				Parts: []geminiPart{{
					InlineData: &geminiInlineData{
						MimeType: inlineData["mimeType"].(string),
						Data:     inlineData["data"].(string),
					},
				}},
				Role: "model",
			},
			FinishReason: "STOP",
		}},
	}
	result := transformToOpenAI(resp)
	parts, ok := result.Choices[0].Message.Content.([]model.ContentPart)
	require.True(t, ok)
	assert.Equal(t, "data:image/jpeg;base64,/9j/4AAQ", parts[0].ImageURL.URL)
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
