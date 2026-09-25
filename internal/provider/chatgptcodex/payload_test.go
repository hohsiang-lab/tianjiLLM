package chatgptcodex

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func intPtr(value int) *int {
	return &value
}

func TestBuildPayload_DoesNotInjectPersona(t *testing.T) {
	req := &model.ChatCompletionRequest{
		Model: "chatgpt/gpt-5.5",
		Messages: []model.Message{
			{Role: "user", Content: "hello"},
		},
	}

	payload, err := BuildPayload(req)

	require.NoError(t, err)
	assert.Empty(t, payload.Instructions)
	data, err := json.Marshal(payload)
	require.NoError(t, err)
	var got map[string]any
	require.NoError(t, json.Unmarshal(data, &got))
	assert.NotContains(t, got, "instructions")
}

func TestBuildPayload_MapsInstructionsAndInputMessages(t *testing.T) {
	req := &model.ChatCompletionRequest{
		Model: "chatgpt/gpt-5.5",
		Messages: []model.Message{
			{Role: "system", Content: "Follow policy."},
			{Role: "developer", Content: "Use concise answers."},
			{Role: "user", Content: "hello"},
			{Role: "assistant", Content: "hi"},
		},
	}

	payload, err := BuildPayload(req)

	require.NoError(t, err)
	assert.Equal(t, "gpt-5.5", payload.Model)
	assert.Equal(t, "system: Follow policy.\ndeveloper: Use concise answers.", payload.Instructions)
	require.Len(t, payload.Input, 2)
	assert.Equal(t, InputMessage{
		Type:    "message",
		Role:    "user",
		Content: []ContentPart{{Type: "input_text", Text: "hello"}},
	}, payload.Input[0])
	assert.Equal(t, InputMessage{
		Type:    "message",
		Role:    "assistant",
		Content: []ContentPart{{Type: "input_text", Text: "hi"}},
	}, payload.Input[1])
}

func TestBuildPayload_MapsNonLeadingInstructionRolesAsConversationInput(t *testing.T) {
	req := &model.ChatCompletionRequest{
		Model: "chatgpt/gpt-5.5",
		Messages: []model.Message{
			{Role: "user", Content: "hello"},
			{Role: "system", Content: "Remember this constraint."},
			{Role: "developer", Content: "Use terse output."},
		},
	}

	payload, err := BuildPayload(req)

	require.NoError(t, err)
	assert.Empty(t, payload.Instructions)
	require.Len(t, payload.Input, 3)
	assert.Equal(t, "system", payload.Input[1].Role)
	assert.Equal(t, []ContentPart{{Type: "input_text", Text: "Remember this constraint."}}, payload.Input[1].Content)
	assert.Equal(t, "developer", payload.Input[2].Role)
	assert.Equal(t, []ContentPart{{Type: "input_text", Text: "Use terse output."}}, payload.Input[2].Content)
}

func TestBuildPayload_MapsTypedContentParts(t *testing.T) {
	detail := "high"
	req := &model.ChatCompletionRequest{
		Model: "chatgpt/gpt-5.5",
		Messages: []model.Message{{
			Role: "user",
			Content: []model.ContentPart{
				{Type: "text", Text: "look"},
				{Type: "image_url", ImageURL: &model.ImageURL{URL: "https://example.test/image.png", Detail: &detail}},
			},
		}},
	}

	payload, err := BuildPayload(req)

	require.NoError(t, err)
	require.Len(t, payload.Input, 1)
	assert.Equal(t, []ContentPart{
		{Type: "input_text", Text: "look"},
		{Type: "input_image", ImageURL: "https://example.test/image.png", Detail: &detail},
	}, payload.Input[0].Content)
}

func TestBuildPayload_MapsOfficialResponsesImageURLString(t *testing.T) {
	raw := []byte(`{
		"model":"openai/gpt-5.5",
		"messages":[{
			"role":"user",
			"content":[
				{"type":"input_text","text":"edit this"},
				{"type":"input_image","image_url":"https://example.test/reference.png","detail":"high"}
			]
		}]
	}`)
	var req model.ChatCompletionRequest
	require.NoError(t, json.Unmarshal(raw, &req))

	payload, err := BuildPayload(&req)

	require.NoError(t, err)
	require.Len(t, payload.Input, 1)
	require.Len(t, payload.Input[0].Content, 2)
	image := payload.Input[0].Content[1]
	assert.Equal(t, "input_image", image.Type)
	assert.Equal(t, "https://example.test/reference.png", image.ImageURL)
	require.NotNil(t, image.Detail)
	assert.Equal(t, "high", *image.Detail)

	data, err := json.Marshal(payload)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"image_url":"https://example.test/reference.png"`)
	assert.NotContains(t, string(data), `"image_url":{"url":`)
	assert.Contains(t, string(data), `"detail":"high"`)
}

func TestBuildPayload_NormalizesLegacyImageURLObject(t *testing.T) {
	raw := []byte(`{
		"model":"openai/gpt-5.5",
		"messages":[{
			"role":"user",
			"content":[
				{"type":"image_url","image_url":{"url":"data:image/png;base64,abc123","detail":"low"}}
			]
		}]
	}`)
	var req model.ChatCompletionRequest
	require.NoError(t, json.Unmarshal(raw, &req))

	payload, err := BuildPayload(&req)

	require.NoError(t, err)
	require.Len(t, payload.Input, 1)
	require.Len(t, payload.Input[0].Content, 1)
	image := payload.Input[0].Content[0]
	assert.Equal(t, "input_image", image.Type)
	assert.Equal(t, "data:image/png;base64,abc123", image.ImageURL)
	require.NotNil(t, image.Detail)
	assert.Equal(t, "low", *image.Detail)

	data, err := json.Marshal(payload)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"image_url":"data:image/png;base64,abc123"`)
	assert.NotContains(t, string(data), `"image_url":{"url":`)
}

func TestBuildPayload_NormalizesLegacyInputImageURLObject(t *testing.T) {
	raw := []byte(`{
		"model":"openai/gpt-5.5",
		"messages":[{
			"role":"user",
			"content":[
				{"type":"input_image","image_url":{"url":"https://example.test/reference.png","detail":"high"}}
			]
		}]
	}`)
	var req model.ChatCompletionRequest
	require.NoError(t, json.Unmarshal(raw, &req))

	payload, err := BuildPayload(&req)

	require.NoError(t, err)
	require.Len(t, payload.Input, 1)
	require.Len(t, payload.Input[0].Content, 1)
	image := payload.Input[0].Content[0]
	assert.Equal(t, "input_image", image.Type)
	assert.Equal(t, "https://example.test/reference.png", image.ImageURL)
	require.NotNil(t, image.Detail)
	assert.Equal(t, "high", *image.Detail)

	data, err := json.Marshal(payload)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"image_url":"https://example.test/reference.png"`)
	assert.Contains(t, string(data), `"detail":"high"`)
	assert.NotContains(t, string(data), `"image_url":{"url":`)
}

func TestBuildPayload_RejectsMalformedImageURL(t *testing.T) {
	raw := []byte(`{
		"model":"openai/gpt-5.5",
		"messages":[{
			"role":"user",
			"content":[{"type":"input_image","image_url":{"detail":"high"}}]
		}]
	}`)
	var req model.ChatCompletionRequest
	require.NoError(t, json.Unmarshal(raw, &req))

	_, err := BuildPayload(&req)

	require.Error(t, err)
	assert.True(t, errors.Is(err, model.ErrClientValidation))
	assert.Contains(t, err.Error(), "image content part url")
	assert.NotContains(t, err.Error(), "high")
}

func TestBuildPayload_RejectsTypedEmptyImageURL(t *testing.T) {
	req := &model.ChatCompletionRequest{
		Model: "chatgpt/gpt-5.5",
		Messages: []model.Message{{
			Role: "user",
			Content: []model.ContentPart{
				{Type: "image_url", ImageURL: &model.ImageURL{URL: " "}},
			},
		}},
	}

	_, err := BuildPayload(req)

	require.Error(t, err)
	assert.True(t, errors.Is(err, model.ErrClientValidation))
	assert.Contains(t, err.Error(), "image content part url")
}

func TestBuildPayload_PreservesCompatibleOptions(t *testing.T) {
	raw := []byte(`{
		"model":"openai/gpt-5.5",
		"messages":[{"role":"user","content":"hello"}],
		"temperature":0.4,
		"top_p":0.8,
		"max_completion_tokens":123,
		"metadata":{"request_id":"req_123"},
		"user":"user_123"
	}`)
	var req model.ChatCompletionRequest
	require.NoError(t, json.Unmarshal(raw, &req))

	payload, err := BuildPayload(&req)

	require.NoError(t, err)
	require.NotNil(t, payload.MaxOutputTokens)
	assert.Equal(t, 123, *payload.MaxOutputTokens)
	assert.Equal(t, 0.4, *payload.Temperature)
	assert.Equal(t, 0.8, *payload.TopP)
	assert.Equal(t, map[string]any{"request_id": "req_123", "user": "user_123"}, payload.Metadata)
}

func TestBuildPayload_OmitsUpstreamUnsupportedSamplingForGraphitiModels(t *testing.T) {
	tests := []struct {
		name                string
		model               string
		maxTokens           *int
		maxCompletionTokens *int
	}{
		{name: "gpt-5.4-mini max_tokens", model: "openai/gpt-5.4-mini", maxTokens: intPtr(120)},
		{name: "gpt-5.6-luna max_completion_tokens", model: "chatgpt/gpt-5.6-luna", maxCompletionTokens: intPtr(120)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			temperature := 0.2
			payload, err := BuildPayload(&model.ChatCompletionRequest{
				Model:               tt.model,
				MaxTokens:           tt.maxTokens,
				MaxCompletionTokens: tt.maxCompletionTokens,
				Messages: []model.Message{{
					Role:    "user",
					Content: "return JSON",
				}},
				Temperature: &temperature,
			})

			require.NoError(t, err)
			assert.Nil(t, payload.Temperature)
			assert.Nil(t, payload.MaxOutputTokens)
		})
	}
}

func TestBuildPayload_MapsVerifiedStructuredOutput(t *testing.T) {
	tests := []struct {
		name   string
		format map[string]any
		want   map[string]any
	}{
		{
			name:   "json object",
			format: map[string]any{"type": "json_object"},
			want:   map[string]any{"type": "json_object"},
		},
		{
			name: "json schema",
			format: map[string]any{
				"type": "json_schema",
				"json_schema": map[string]any{
					"name":   "entity",
					"schema": map[string]any{"type": "object"},
					"strict": true,
				},
			},
			want: map[string]any{
				"type":   "json_schema",
				"name":   "entity",
				"schema": map[string]any{"type": "object"},
				"strict": true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload, err := BuildPayload(&model.ChatCompletionRequest{
				Model:          "chatgpt/gpt-5.6-terra",
				Messages:       []model.Message{{Role: "user", Content: "extract"}},
				ResponseFormat: tt.format,
			})
			require.NoError(t, err)

			data, err := json.Marshal(payload)
			require.NoError(t, err)
			var raw map[string]any
			require.NoError(t, json.Unmarshal(data, &raw))
			assert.Equal(t, map[string]any{"format": tt.want}, raw["text"])
			assert.NotContains(t, raw, "response_format")
		})
	}
}

func TestBuildPayload_OmitsExplicitTextResponseFormat(t *testing.T) {
	payload, err := BuildPayload(&model.ChatCompletionRequest{
		Model:          "chatgpt/gpt-5.6-terra",
		Messages:       []model.Message{{Role: "user", Content: "hello"}},
		ResponseFormat: map[string]any{"type": "text"},
	})

	require.NoError(t, err)
	assert.Nil(t, payload.Text)
}

func TestBuildPayload_MapsVerifiedToolsAndToolChoice(t *testing.T) {
	strict := true
	payload, err := BuildPayload(&model.ChatCompletionRequest{
		Model:    "chatgpt/gpt-5.6-terra",
		Messages: []model.Message{{Role: "user", Content: "look it up"}},
		Tools: []model.Tool{{
			Type: "function",
			Function: model.ToolFunction{
				Name:        "lookup",
				Description: "Find a record",
				Parameters:  map[string]any{"type": "object"},
				Strict:      &strict,
			},
		}},
		ToolChoice: map[string]any{
			"type":     "function",
			"function": map[string]any{"name": "lookup"},
		},
	})
	require.NoError(t, err)

	data, err := json.Marshal(payload)
	require.NoError(t, err)
	var raw map[string]any
	require.NoError(t, json.Unmarshal(data, &raw))
	tools, ok := raw["tools"].([]any)
	require.True(t, ok)
	require.Len(t, tools, 1)
	assert.Equal(t, map[string]any{
		"type":        "function",
		"name":        "lookup",
		"description": "Find a record",
		"parameters":  map[string]any{"type": "object"},
		"strict":      true,
	}, tools[0])
	assert.Equal(t, map[string]any{"type": "function", "name": "lookup"}, raw["tool_choice"])
}

func TestBuildPayload_MapsToolCallConversationHistory(t *testing.T) {
	rawRequest := []byte(`{
		"model":"chatgpt/gpt-5.6-terra",
		"messages":[
			{"role":"user","content":"look it up"},
			{"role":"assistant","content":null,"tool_calls":[{
				"id":"call_1",
				"type":"function",
				"function":{"name":"lookup","arguments":"{\"id\":\"123\"}"}
			}]},
			{"role":"tool","tool_call_id":"call_1","content":"{\"name\":\"Ada\"}"},
			{"role":"assistant","content":"Ada"}
		]
	}`)
	var req model.ChatCompletionRequest
	require.NoError(t, json.Unmarshal(rawRequest, &req))

	payload, err := BuildPayload(&req)

	require.NoError(t, err)
	data, err := json.Marshal(payload)
	require.NoError(t, err)
	var raw map[string]any
	require.NoError(t, json.Unmarshal(data, &raw))
	assert.Equal(t, []any{
		map[string]any{
			"type": "message",
			"role": "user",
			"content": []any{
				map[string]any{"type": "input_text", "text": "look it up"},
			},
		},
		map[string]any{
			"type":      "function_call",
			"call_id":   "call_1",
			"name":      "lookup",
			"arguments": `{"id":"123"}`,
		},
		map[string]any{
			"type":    "function_call_output",
			"call_id": "call_1",
			"output":  `{"name":"Ada"}`,
		},
		map[string]any{
			"type": "message",
			"role": "assistant",
			"content": []any{
				map[string]any{"type": "input_text", "text": "Ada"},
			},
		},
	}, raw["input"])
}

func TestBuildPayload_PreservesAssistantRefusalHistory(t *testing.T) {
	refusal := "cannot comply"
	payload, err := BuildPayload(&model.ChatCompletionRequest{
		Model: "chatgpt/gpt-5.6-terra",
		Messages: []model.Message{
			{Role: "user", Content: "do the unsafe thing"},
			{Role: "assistant", Content: nil, Refusal: &refusal},
			{Role: "assistant", Content: "I can help safely.", Refusal: &refusal},
		},
	})

	require.NoError(t, err)
	require.Len(t, payload.Input, 3)
	assert.Equal(t, []ContentPart{{Type: "refusal", Refusal: refusal}}, payload.Input[1].Content)
	assert.Equal(t, []ContentPart{
		{Type: "input_text", Text: "I can help safely."},
		{Type: "refusal", Refusal: refusal},
	}, payload.Input[2].Content)
}

func TestBuildPayload_PreservesNamedMessages(t *testing.T) {
	userName := "example_user"
	assistantName := "example_assistant"
	payload, err := BuildPayload(&model.ChatCompletionRequest{
		Model: "chatgpt/gpt-5.6-terra",
		Messages: []model.Message{
			{Role: "user", Name: &userName, Content: "hello"},
			{Role: "assistant", Name: &assistantName, Content: "hi"},
		},
	})

	require.NoError(t, err)
	require.Len(t, payload.Input, 2)
	assert.Equal(t, userName, payload.Input[0].Name)
	assert.Equal(t, assistantName, payload.Input[1].Name)
}

func TestBuildPayload_MapsToolOutputTextParts(t *testing.T) {
	rawRequest := []byte(`{
		"model":"chatgpt/gpt-5.6-terra",
		"messages":[{
			"role":"tool",
			"tool_call_id":"call_1",
			"content":[
				{"type":"text","text":"first"},
				{"type":"text","text":"second"}
			]
		}]
	}`)
	var req model.ChatCompletionRequest
	require.NoError(t, json.Unmarshal(rawRequest, &req))

	payload, err := BuildPayload(&req)

	require.NoError(t, err)
	require.Len(t, payload.Input, 1)
	require.NotNil(t, payload.Input[0].Output)
	assert.Equal(t, "first\nsecond", *payload.Input[0].Output)
}

func TestBuildPayload_IncludesStoreFalse(t *testing.T) {
	req := &model.ChatCompletionRequest{
		Model:    "chatgpt/gpt-5.5",
		Messages: []model.Message{{Role: "user", Content: "hello"}},
	}

	payload, err := BuildPayload(req)

	require.NoError(t, err)
	data, err := json.Marshal(payload)
	require.NoError(t, err)
	assert.JSONEq(t, `false`, string(json.RawMessage(mustJSONField(t, data, "store"))))
}

func TestBuildPayload_AllowsClientStoreFalse(t *testing.T) {
	raw := []byte(`{
		"model":"openai/gpt-5.5",
		"messages":[{"role":"user","content":"hello"}],
		"store":false
	}`)
	var req model.ChatCompletionRequest
	require.NoError(t, json.Unmarshal(raw, &req))
	require.Contains(t, req.ExtraParams, "store")

	payload, err := BuildPayload(&req)

	require.NoError(t, err)
	data, err := json.Marshal(payload)
	require.NoError(t, err)
	assert.JSONEq(t, `false`, string(json.RawMessage(mustJSONField(t, data, "store"))))
}

func TestBuildPayload_RejectsClientStoreTrue(t *testing.T) {
	raw := []byte(`{
		"model":"openai/gpt-5.5",
		"messages":[{"role":"user","content":"hello"}],
		"store":true
	}`)
	var req model.ChatCompletionRequest
	require.NoError(t, json.Unmarshal(raw, &req))

	_, err := BuildPayload(&req)

	require.Error(t, err)
	assert.True(t, errors.Is(err, model.ErrClientValidation))
	assert.Contains(t, err.Error(), "store must be false")
}

func TestBuildPayload_RejectsUnknownExtraParams(t *testing.T) {
	req := &model.ChatCompletionRequest{
		Model:       "chatgpt/gpt-5.5",
		Messages:    []model.Message{{Role: "user", Content: "hello"}},
		ExtraParams: map[string]any{"parallel_tool_calls": true},
	}

	_, err := BuildPayload(req)

	require.Error(t, err)
	assert.True(t, errors.Is(err, model.ErrClientValidation))
	assert.Contains(t, err.Error(), "unsupported")
	assert.Contains(t, err.Error(), "unknown extra parameters")
}

func TestBuildPayload_RejectsUnsupportedFields(t *testing.T) {
	n := 2
	logprobs := true
	topLogprobs := 3
	stop := []string{"stop"}
	seed := 42

	tests := []struct {
		name string
		req  model.ChatCompletionRequest
		want string
	}{
		{name: "tools", req: model.ChatCompletionRequest{Tools: []model.Tool{{Type: "function"}}}, want: "tools"},
		{name: "n greater than one", req: model.ChatCompletionRequest{N: &n}, want: "n > 1"},
		{name: "logprobs", req: model.ChatCompletionRequest{LogProbs: &logprobs}, want: "logprobs"},
		{name: "top_logprobs", req: model.ChatCompletionRequest{TopLogProbs: &topLogprobs}, want: "top_logprobs"},
		{name: "stop", req: model.ChatCompletionRequest{Stop: stop}, want: "stop"},
		{name: "seed", req: model.ChatCompletionRequest{Seed: &seed}, want: "seed"},
		{name: "tool role missing call id", req: model.ChatCompletionRequest{Messages: []model.Message{{Role: "tool", Content: "result"}}}, want: "tool_call_id"},
		{name: "malformed tool call", req: model.ChatCompletionRequest{Messages: []model.Message{{Role: "assistant", Content: "result", ToolCalls: []model.ToolCall{{ID: "call_1"}}}}}, want: "tool_calls"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.req.Model = "chatgpt/gpt-5.5"
			if len(tt.req.Messages) == 0 {
				tt.req.Messages = []model.Message{{Role: "user", Content: "hello"}}
			}

			_, err := BuildPayload(&tt.req)

			require.Error(t, err)
			assert.True(t, errors.Is(err, model.ErrClientValidation))
			assert.Contains(t, err.Error(), tt.want)
		})
	}
}

func TestBuildPayload_DiagnosticDoesNotLeakUnsupportedValues(t *testing.T) {
	req := &model.ChatCompletionRequest{
		Model:       "chatgpt/gpt-5.5",
		Messages:    []model.Message{{Role: "user", Content: "hello"}},
		ExtraParams: map[string]any{"secret": "Bearer sk-test-secret"},
	}

	_, err := BuildPayload(req)

	require.Error(t, err)
	assert.NotContains(t, err.Error(), "sk-test-secret")
	assert.NotContains(t, err.Error(), "Bearer")
}

func mustJSONField(t *testing.T, data []byte, field string) json.RawMessage {
	t.Helper()
	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(data, &raw))
	got, ok := raw[field]
	require.Truef(t, ok, "field %q missing from %s", field, data)
	return got
}
