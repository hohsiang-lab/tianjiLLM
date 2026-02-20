package openai

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseStreamChunk_Content(t *testing.T) {
	data := []byte(`{"id":"chatcmpl-abc123","object":"chat.completion.chunk","created":1700000000,"model":"gpt-4o","choices":[{"index":0,"delta":{"content":"Hello"},"finish_reason":null}]}`)

	chunk, done, err := ParseStreamChunk(data)
	require.NoError(t, err)
	assert.False(t, done)
	assert.Equal(t, "chatcmpl-abc123", chunk.ID)
	assert.Equal(t, "chat.completion.chunk", chunk.Object)
	require.Len(t, chunk.Choices, 1)
	assert.Equal(t, "Hello", *chunk.Choices[0].Delta.Content)
	assert.Nil(t, chunk.Choices[0].FinishReason)
}

func TestParseStreamChunk_FinishReason(t *testing.T) {
	data := []byte(`{"id":"chatcmpl-abc123","object":"chat.completion.chunk","created":1700000000,"model":"gpt-4o","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`)

	chunk, done, err := ParseStreamChunk(data)
	require.NoError(t, err)
	assert.False(t, done)
	require.NotNil(t, chunk.Choices[0].FinishReason)
	assert.Equal(t, "stop", *chunk.Choices[0].FinishReason)
}

func TestParseStreamChunk_Done(t *testing.T) {
	data := []byte("[DONE]")

	chunk, done, err := ParseStreamChunk(data)
	require.NoError(t, err)
	assert.True(t, done)
	assert.Nil(t, chunk)
}

func TestParseStreamChunk_DoneWithWhitespace(t *testing.T) {
	data := []byte("  [DONE]  ")

	chunk, done, err := ParseStreamChunk(data)
	require.NoError(t, err)
	assert.True(t, done)
	assert.Nil(t, chunk)
}

func TestParseStreamChunk_RoleChunk(t *testing.T) {
	data := []byte(`{"id":"chatcmpl-abc123","object":"chat.completion.chunk","created":1700000000,"model":"gpt-4o","choices":[{"index":0,"delta":{"role":"assistant","content":""},"finish_reason":null}]}`)

	chunk, done, err := ParseStreamChunk(data)
	require.NoError(t, err)
	assert.False(t, done)
	require.NotNil(t, chunk.Choices[0].Delta.Role)
	assert.Equal(t, "assistant", *chunk.Choices[0].Delta.Role)
}

func TestParseStreamChunk_InvalidJSON(t *testing.T) {
	data := []byte(`{invalid json}`)

	_, _, err := ParseStreamChunk(data)
	assert.Error(t, err)
}
