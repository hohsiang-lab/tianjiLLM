package anthropic

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseStreamEvent_MessageStart(t *testing.T) {
	data := []byte(`{"type":"message_start","message":{"id":"msg_01","model":"claude-sonnet-4-5-20250929"}}`)

	chunk, done, err := ParseStreamEvent(data)
	require.NoError(t, err)
	assert.False(t, done)
	assert.Equal(t, "msg_01", chunk.ID)
	assert.Equal(t, "claude-sonnet-4-5-20250929", chunk.Model)
	assert.Equal(t, "assistant", *chunk.Choices[0].Delta.Role)
	assert.Nil(t, chunk.Usage)
}

func TestParseStreamEvent_MessageStart_WithUsage(t *testing.T) {
	// Real Anthropic: input_tokens is in message_start.message.usage, not message_delta.
	data := []byte(`{"type":"message_start","message":{"id":"msg_02","model":"claude-sonnet-4-5-20250929","usage":{"input_tokens":42}}}`)

	chunk, done, err := ParseStreamEvent(data)
	require.NoError(t, err)
	assert.False(t, done)
	assert.Equal(t, "msg_02", chunk.ID)
	require.NotNil(t, chunk.Usage)
	assert.Equal(t, 42, chunk.Usage.PromptTokens)
	assert.Equal(t, 0, chunk.Usage.CompletionTokens)
}

func TestParseStreamEvent_TextDelta(t *testing.T) {
	data := []byte(`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello"}}`)

	chunk, done, err := ParseStreamEvent(data)
	require.NoError(t, err)
	assert.False(t, done)
	assert.Equal(t, "Hello", *chunk.Choices[0].Delta.Content)
}

func TestParseStreamEvent_MessageDelta(t *testing.T) {
	// Real Anthropic: message_delta.usage only contains output_tokens.
	data := []byte(`{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":5}}`)

	chunk, done, err := ParseStreamEvent(data)
	require.NoError(t, err)
	assert.False(t, done)
	assert.Equal(t, "stop", *chunk.Choices[0].FinishReason)
	require.NotNil(t, chunk.Usage)
	assert.Equal(t, 0, chunk.Usage.PromptTokens)
	assert.Equal(t, 5, chunk.Usage.CompletionTokens)
}

func TestParseStreamEvent_MessageStop(t *testing.T) {
	data := []byte(`{"type":"message_stop"}`)

	chunk, done, err := ParseStreamEvent(data)
	require.NoError(t, err)
	assert.True(t, done)
	assert.Nil(t, chunk)
}

func TestParseStreamEvent_ToolUseStart(t *testing.T) {
	data := []byte(`{"type":"content_block_start","index":1,"content_block":{"type":"tool_use","id":"toolu_01","name":"get_weather"}}`)

	chunk, done, err := ParseStreamEvent(data)
	require.NoError(t, err)
	assert.False(t, done)
	require.Len(t, chunk.Choices[0].Delta.ToolCalls, 1)
	assert.Equal(t, "toolu_01", chunk.Choices[0].Delta.ToolCalls[0].ID)
	assert.Equal(t, "get_weather", chunk.Choices[0].Delta.ToolCalls[0].Function.Name)
}

func TestParseStreamEvent_ToolUseInputDelta(t *testing.T) {
	data := []byte(`{"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":"{\"location\":"}}`)

	chunk, done, err := ParseStreamEvent(data)
	require.NoError(t, err)
	assert.False(t, done)
	require.Len(t, chunk.Choices[0].Delta.ToolCalls, 1)
	assert.Contains(t, chunk.Choices[0].Delta.ToolCalls[0].Function.Arguments, "location")
}
