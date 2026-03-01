package bedrock

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseStreamEvent_MessageStart(t *testing.T) {
	event := map[string]any{
		"messageStart": map[string]any{"role": "assistant"},
	}
	data, _ := json.Marshal(event)
	chunk, done, err := ParseStreamEvent(data)
	require.NoError(t, err)
	assert.False(t, done)
	require.NotNil(t, chunk)
	require.Len(t, chunk.Choices, 1)
	assert.NotNil(t, chunk.Choices[0].Delta.Role)
	assert.Equal(t, "assistant", *chunk.Choices[0].Delta.Role)
}

func TestParseStreamEvent_ContentBlockDelta_Text(t *testing.T) {
	event := map[string]any{
		"contentBlockDelta": map[string]any{
			"contentBlockIndex": 0,
			"delta":             map[string]any{"text": "Hello world"},
		},
	}
	data, _ := json.Marshal(event)
	chunk, done, err := ParseStreamEvent(data)
	require.NoError(t, err)
	assert.False(t, done)
	require.NotNil(t, chunk)
	require.Len(t, chunk.Choices, 1)
	assert.NotNil(t, chunk.Choices[0].Delta.Content)
	assert.Equal(t, "Hello world", *chunk.Choices[0].Delta.Content)
}

func TestParseStreamEvent_ContentBlockDelta_ToolUse(t *testing.T) {
	event := map[string]any{
		"contentBlockDelta": map[string]any{
			"contentBlockIndex": 1,
			"delta":             map[string]any{"toolUse": map[string]any{"input": `{"location":"SF"}`}},
		},
	}
	data, _ := json.Marshal(event)
	chunk, done, err := ParseStreamEvent(data)
	require.NoError(t, err)
	assert.False(t, done)
	require.NotNil(t, chunk)
	require.Len(t, chunk.Choices[0].Delta.ToolCalls, 1)
	assert.Equal(t, `{"location":"SF"}`, chunk.Choices[0].Delta.ToolCalls[0].Function.Arguments)
}

func TestParseStreamEvent_ContentBlockStart_ToolUse(t *testing.T) {
	event := map[string]any{
		"contentBlockStart": map[string]any{
			"contentBlockIndex": 0,
			"start": map[string]any{
				"toolUse": map[string]any{"toolUseId": "tu_01", "name": "get_weather"},
			},
		},
	}
	data, _ := json.Marshal(event)
	chunk, done, err := ParseStreamEvent(data)
	require.NoError(t, err)
	assert.False(t, done)
	require.NotNil(t, chunk)
	require.Len(t, chunk.Choices[0].Delta.ToolCalls, 1)
	tc := chunk.Choices[0].Delta.ToolCalls[0]
	assert.Equal(t, "tu_01", tc.ID)
	assert.Equal(t, "get_weather", tc.Function.Name)
	assert.Equal(t, "function", tc.Type)
}

func TestParseStreamEvent_MessageStop(t *testing.T) {
	event := map[string]any{"messageStop": map[string]any{"stopReason": "end_turn"}}
	data, _ := json.Marshal(event)
	chunk, done, err := ParseStreamEvent(data)
	require.NoError(t, err)
	assert.True(t, done)
	require.NotNil(t, chunk)
	require.Len(t, chunk.Choices, 1)
	assert.NotNil(t, chunk.Choices[0].FinishReason)
	assert.Equal(t, "stop", *chunk.Choices[0].FinishReason)
}

func TestParseStreamEvent_Metadata(t *testing.T) {
	event := map[string]any{
		"metadata": map[string]any{
			"usage": map[string]any{"inputTokens": 10, "outputTokens": 5, "totalTokens": 15},
		},
	}
	data, _ := json.Marshal(event)
	chunk, done, err := ParseStreamEvent(data)
	require.NoError(t, err)
	assert.False(t, done)
	require.NotNil(t, chunk)
	require.NotNil(t, chunk.Usage)
	assert.Equal(t, 10, chunk.Usage.PromptTokens)
	assert.Equal(t, 5, chunk.Usage.CompletionTokens)
	assert.Equal(t, 15, chunk.Usage.TotalTokens)
}

func TestParseStreamEvent_Empty(t *testing.T) {
	data, _ := json.Marshal(map[string]any{})
	chunk, done, err := ParseStreamEvent(data)
	require.NoError(t, err)
	assert.False(t, done)
	assert.Nil(t, chunk)
}

func TestParseStreamEvent_InvalidJSON(t *testing.T) {
	_, _, err := ParseStreamEvent([]byte("not-json"))
	assert.Error(t, err)
}
