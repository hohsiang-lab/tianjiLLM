package chatgptcodex

import (
	"errors"
	"strings"
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTransformStreamChunk_OutputTextDelta(t *testing.T) {
	chunk, done, err := TransformStreamChunk([]byte(`{"type":"response.output_text.delta","response_id":"resp_123","delta":"OK"}`), "openai/gpt-5.5")

	require.NoError(t, err)
	require.False(t, done)
	require.NotNil(t, chunk)
	assert.Equal(t, "resp_123", chunk.ID)
	assert.Equal(t, "chat.completion.chunk", chunk.Object)
	assert.Equal(t, "gpt-5.5", chunk.Model)
	require.Len(t, chunk.Choices, 1)
	require.NotNil(t, chunk.Choices[0].Delta.Content)
	assert.Equal(t, "OK", *chunk.Choices[0].Delta.Content)
}

func TestTransformStreamChunk_ResponseCompletedCarriesUsageAndDone(t *testing.T) {
	chunk, done, err := TransformStreamChunk([]byte(`{"type":"response.completed","response":{"id":"resp_123","model":"gpt-5.5","usage":{"input_tokens":7,"output_tokens":2,"total_tokens":9}}}`), "openai/gpt-5.5")

	require.NoError(t, err)
	require.True(t, done)
	require.NotNil(t, chunk)
	assert.Empty(t, chunk.Choices)
	require.NotNil(t, chunk.Usage)
	assert.Equal(t, 7, chunk.Usage.PromptTokens)
	assert.Equal(t, 2, chunk.Usage.CompletionTokens)
	assert.Equal(t, 9, chunk.Usage.TotalTokens)
}

func TestTransformStreamChunk_ResponseCompletedUsesTopLevelUsageWithoutTotal(t *testing.T) {
	chunk, done, err := TransformStreamChunk([]byte(`{"type":"response.completed","usage":{"input_tokens":7,"output_tokens":2}}`), "openai/gpt-5.5")

	require.NoError(t, err)
	require.True(t, done)
	require.NotNil(t, chunk)
	require.NotNil(t, chunk.Usage)
	assert.Equal(t, 7, chunk.Usage.PromptTokens)
	assert.Equal(t, 2, chunk.Usage.CompletionTokens)
	assert.Equal(t, 9, chunk.Usage.TotalTokens)
}

func TestTransformStreamChunk_ResponseCompletedWithoutUsageDoesNotFabricateUsage(t *testing.T) {
	chunk, done, err := TransformStreamChunk([]byte(`{
		"type":"response.completed",
		"response":{
			"id":"resp_123",
			"model":"gpt-5.5",
			"output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"ready"}]}]
		}
	}`), "openai/gpt-5.5")

	require.NoError(t, err)
	require.True(t, done)
	require.NotNil(t, chunk)
	assert.Nil(t, chunk.Usage)
}

func TestStreamTranslator_ResponseCompletedPreservesTerminalOnlyOutput(t *testing.T) {
	var translator StreamTranslator
	chunk, done, err := translator.Transform([]byte(`{
		"type":"response.completed",
		"response":{
			"id":"resp_terminal",
			"model":"gpt-5.5",
			"output":[
				{"type":"message","role":"assistant","content":[
					{"type":"output_text","text":"ready"},
					{"type":"refusal","refusal":"no"}
				]},
				{"type":"function_call","id":"fc_1","call_id":"call_1","name":"lookup","arguments":"{}"}
			],
			"usage":{"input_tokens":7,"output_tokens":3,"total_tokens":10}
		}
	}`), "openai/gpt-5.5")

	require.NoError(t, err)
	require.True(t, done)
	require.NotNil(t, chunk)
	require.Len(t, chunk.Choices, 1)
	require.NotNil(t, chunk.Choices[0].Delta.Content)
	assert.Equal(t, "ready", *chunk.Choices[0].Delta.Content)
	require.NotNil(t, chunk.Choices[0].Delta.Refusal)
	assert.Equal(t, "no", *chunk.Choices[0].Delta.Refusal)
	require.Len(t, chunk.Choices[0].Delta.ToolCalls, 1)
	assert.Equal(t, "call_1", chunk.Choices[0].Delta.ToolCalls[0].ID)
	assert.Equal(t, "lookup", chunk.Choices[0].Delta.ToolCalls[0].Function.Name)
	assert.Equal(t, "{}", chunk.Choices[0].Delta.ToolCalls[0].Function.Arguments)
	require.NotNil(t, chunk.Choices[0].FinishReason)
	assert.Equal(t, "tool_calls", *chunk.Choices[0].FinishReason)
	require.NotNil(t, chunk.Usage)
	assert.Equal(t, 10, chunk.Usage.TotalTokens)
}

func TestStreamTranslator_ResponseCompletedAddsUnstreamedTails(t *testing.T) {
	var translator StreamTranslator
	for _, event := range []string{
		`{"type":"response.output_text.delta","response_id":"resp_partial","output_index":0,"delta":"rea"}`,
		`{"type":"response.refusal.delta","response_id":"resp_partial","output_index":0,"delta":"n"}`,
		`{"type":"response.output_item.added","response_id":"resp_partial","output_index":1,"item":{"type":"function_call","call_id":"call_1","name":"lookup"}}`,
		`{"type":"response.function_call_arguments.delta","response_id":"resp_partial","output_index":1,"delta":"{\"q\":\""}`,
	} {
		_, done, err := translator.Transform([]byte(event), "openai/gpt-5.5")
		require.NoError(t, err)
		require.False(t, done)
	}

	chunk, done, err := translator.Transform([]byte(`{
		"type":"response.completed",
		"response":{
			"id":"resp_partial",
			"model":"gpt-5.5",
			"output":[
				{"type":"message","role":"assistant","content":[
					{"type":"output_text","text":"ready"},
					{"type":"refusal","refusal":"no"}
				]},
				{"type":"function_call","call_id":"call_1","name":"lookup","arguments":"{\"q\":\"x\"}"}
			],
			"usage":{"input_tokens":7,"output_tokens":3,"total_tokens":10}
		}
	}`), "openai/gpt-5.5")

	require.NoError(t, err)
	require.True(t, done)
	require.NotNil(t, chunk)
	require.Len(t, chunk.Choices, 1)
	require.NotNil(t, chunk.Choices[0].Delta.Content)
	assert.Equal(t, "dy", *chunk.Choices[0].Delta.Content)
	require.NotNil(t, chunk.Choices[0].Delta.Refusal)
	assert.Equal(t, "o", *chunk.Choices[0].Delta.Refusal)
	require.Len(t, chunk.Choices[0].Delta.ToolCalls, 1)
	assert.Empty(t, chunk.Choices[0].Delta.ToolCalls[0].ID)
	assert.Empty(t, chunk.Choices[0].Delta.ToolCalls[0].Function.Name)
	assert.Equal(t, `x"}`, chunk.Choices[0].Delta.ToolCalls[0].Function.Arguments)
}

func TestStreamTranslatorDoneEventsEmitOnlyUnstreamedSuffix(t *testing.T) {
	var translator StreamTranslator
	for _, event := range []string{
		`{"type":"response.output_item.added","output_index":1,"item":{"id":"fc_fixture","type":"function_call","call_id":"call_fixture","name":"lookup","arguments":""}}`,
		`{"type":"response.function_call_arguments.delta","item_id":"fc_fixture","output_index":1,"delta":"{\"id\":"}`,
	} {
		chunk, done, err := translator.Transform([]byte(event), "openai/gpt-5.5")
		require.NoError(t, err)
		require.False(t, done)
		require.NotNil(t, chunk)
	}

	argumentsDone, done, err := translator.Transform([]byte(`{
		"type":"response.function_call_arguments.done",
		"item_id":"fc_fixture",
		"output_index":1,
		"name":"lookup",
		"arguments":"{\"id\":\"42\"}"
	}`), "openai/gpt-5.5")
	require.NoError(t, err)
	require.False(t, done)
	require.NotNil(t, argumentsDone)
	require.Len(t, argumentsDone.Choices[0].Delta.ToolCalls, 1)
	assert.Empty(t, argumentsDone.Choices[0].Delta.ToolCalls[0].Function.Name)
	assert.Equal(t, `"42"}`, argumentsDone.Choices[0].Delta.ToolCalls[0].Function.Arguments)

	itemDone, done, err := translator.Transform([]byte(`{
		"type":"response.output_item.done",
		"output_index":1,
		"item":{
			"id":"fc_fixture",
			"type":"function_call",
			"call_id":"call_fixture",
			"name":"lookup",
			"arguments":"{\"id\":\"42\"}"
		}
	}`), "openai/gpt-5.5")
	require.NoError(t, err)
	require.False(t, done)
	assert.Nil(t, itemDone)

	refusalDelta, done, err := translator.Transform([]byte(`{
		"type":"response.refusal.delta",
		"output_index":0,
		"delta":"cannot "
	}`), "openai/gpt-5.5")
	require.NoError(t, err)
	require.False(t, done)
	require.NotNil(t, refusalDelta)

	refusalDone, done, err := translator.Transform([]byte(`{
		"type":"response.refusal.done",
		"output_index":0,
		"refusal":"cannot comply"
	}`), "openai/gpt-5.5")
	require.NoError(t, err)
	require.False(t, done)
	require.NotNil(t, refusalDone)
	require.NotNil(t, refusalDone.Choices[0].Delta.Refusal)
	assert.Equal(t, "comply", *refusalDone.Choices[0].Delta.Refusal)

	completed, done, err := translator.Transform([]byte(`{
		"type":"response.completed",
		"response":{
			"id":"resp_fixture",
			"model":"gpt-fixture",
			"output":[
				{"type":"message","role":"assistant","content":[{"type":"refusal","refusal":"cannot comply"}]},
				{"id":"fc_fixture","type":"function_call","call_id":"call_fixture","name":"lookup","arguments":"{\"id\":\"42\"}"}
			],
			"usage":{"input_tokens":7,"output_tokens":3,"total_tokens":10}
		}
	}`), "openai/gpt-5.5")
	require.NoError(t, err)
	require.True(t, done)
	require.NotNil(t, completed)
	assert.Empty(t, completed.Choices)
	require.NotNil(t, completed.Usage)
	assert.Equal(t, 10, completed.Usage.TotalTokens)
}

func TestStreamTranslatorOutputItemDonePreservesUnannouncedToolCall(t *testing.T) {
	var translator StreamTranslator
	chunk, done, err := translator.Transform([]byte(`{
		"type":"response.output_item.done",
		"output_index":1,
		"item":{
			"id":"fc_fixture",
			"type":"function_call",
			"call_id":"call_fixture",
			"name":"lookup",
			"arguments":"{\"id\":\"42\"}"
		}
	}`), "openai/gpt-5.5")

	require.NoError(t, err)
	require.False(t, done)
	require.NotNil(t, chunk)
	require.Len(t, chunk.Choices[0].Delta.ToolCalls, 1)
	call := chunk.Choices[0].Delta.ToolCalls[0]
	assert.Equal(t, "call_fixture", call.ID)
	assert.Equal(t, "function", call.Type)
	assert.Equal(t, "lookup", call.Function.Name)
	assert.Equal(t, `{"id":"42"}`, call.Function.Arguments)
}

func TestTransformStreamChunk_PreservesToolCallAndRefusalFragments(t *testing.T) {
	tests := []struct {
		data string
		want func(*testing.T, *model.StreamChunk)
	}{
		{
			data: `{"type":"response.output_item.added","response_id":"resp_123","output_index":1,"item":{"type":"function_call","call_id":"call_123","name":"lookup"}}`,
			want: func(t *testing.T, chunk *model.StreamChunk) {
				require.Len(t, chunk.Choices[0].Delta.ToolCalls, 1)
				call := chunk.Choices[0].Delta.ToolCalls[0]
				assert.Equal(t, "call_123", call.ID)
				assert.Equal(t, "function", call.Type)
				assert.Equal(t, "lookup", call.Function.Name)
				require.NotNil(t, call.Index)
				assert.Equal(t, 0, *call.Index)
			},
		},
		{
			data: `{"type":"response.function_call_arguments.delta","response_id":"resp_123","output_index":1,"delta":"{\"q\":\"x\"}"}`,
			want: func(t *testing.T, chunk *model.StreamChunk) {
				require.Len(t, chunk.Choices[0].Delta.ToolCalls, 1)
				assert.Equal(t, `{"q":"x"}`, chunk.Choices[0].Delta.ToolCalls[0].Function.Arguments)
			},
		},
		{
			data: `{"type":"response.refusal.delta","response_id":"resp_123","delta":"cannot comply"}`,
			want: func(t *testing.T, chunk *model.StreamChunk) {
				require.NotNil(t, chunk.Choices[0].Delta.Refusal)
				assert.Equal(t, "cannot comply", *chunk.Choices[0].Delta.Refusal)
			},
		},
	}

	for _, tt := range tests {
		chunk, done, err := TransformStreamChunk([]byte(tt.data), "openai/gpt-5.5")
		require.NoError(t, err)
		assert.False(t, done)
		require.NotNil(t, chunk)
		tt.want(t, chunk)
	}
}

func TestStreamTranslatorRenumbersResponsesOutputIndexesForChatToolCalls(t *testing.T) {
	var translator StreamTranslator

	_, _, err := translator.Transform(
		[]byte(`{"type":"response.output_text.delta","response_id":"resp_123","output_index":0,"delta":"hello"}`),
		"openai/gpt-5.5",
	)
	require.NoError(t, err)

	first, _, err := translator.Transform(
		[]byte(`{"type":"response.output_item.added","response_id":"resp_123","output_index":1,"item":{"type":"function_call","call_id":"call_1","name":"first"}}`),
		"openai/gpt-5.5",
	)
	require.NoError(t, err)
	require.NotNil(t, first)
	require.NotNil(t, first.Choices[0].Delta.ToolCalls[0].Index)
	assert.Equal(t, 0, *first.Choices[0].Delta.ToolCalls[0].Index)

	second, _, err := translator.Transform(
		[]byte(`{"type":"response.output_item.added","response_id":"resp_123","output_index":2,"item":{"type":"function_call","call_id":"call_2","name":"second"}}`),
		"openai/gpt-5.5",
	)
	require.NoError(t, err)
	require.NotNil(t, second)
	require.NotNil(t, second.Choices[0].Delta.ToolCalls[0].Index)
	assert.Equal(t, 1, *second.Choices[0].Delta.ToolCalls[0].Index)
}

func TestTransformStreamChunk_MalformedOrIncompleteFails(t *testing.T) {
	for _, data := range []string{
		`{not-json}`,
		`{"type":"response.incomplete","response":{"id":"resp_123","error":{"message":"cut short","type":"server_error","code":"incomplete"}}}`,
	} {
		t.Run(data, func(t *testing.T) {
			chunk, done, err := TransformStreamChunk([]byte(data), "openai/gpt-5.5")
			require.Error(t, err)
			assert.Nil(t, chunk)
			if strings.Contains(data, "response.incomplete") {
				assert.True(t, done)
			}
		})
	}
}

func TestTransformStreamChunk_DoneMarker(t *testing.T) {
	chunk, done, err := TransformStreamChunk([]byte(`[DONE]`), "openai/gpt-5.5")

	require.NoError(t, err)
	assert.True(t, done)
	assert.Nil(t, chunk)
}

func TestTransformStreamChunk_ResponseFailedReturnsError(t *testing.T) {
	chunk, done, err := TransformStreamChunk([]byte(`{"type":"response.failed","response":{"id":"resp_123","error":{"message":"backend failed","type":"api_error","code":"upstream_failed"}}}`), "openai/gpt-5.5")

	require.Error(t, err)
	assert.True(t, done)
	assert.Nil(t, chunk)
	var tianjiErr *model.TianjiError
	require.True(t, errors.As(err, &tianjiErr))
	assert.Equal(t, "backend failed", tianjiErr.Message)
	assert.Equal(t, "api_error", tianjiErr.Type)
	assert.Equal(t, "upstream_failed", tianjiErr.Code)
}
