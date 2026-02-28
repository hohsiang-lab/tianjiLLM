package handler

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// AC-1: Streaming — message_start with cache_read_input_tokens=50000
func TestParseSSEUsage_AnthropicCacheTokens(t *testing.T) {
	t.Parallel()
	raw := []byte(
		`data: {"type":"message_start","message":{"model":"claude-3-5-sonnet-20241022","usage":{"input_tokens":1000,"output_tokens":0,"cache_read_input_tokens":50000,"cache_creation_input_tokens":0}}}` + "\n" +
			`data: {"type":"message_delta","usage":{"output_tokens":100}}` + "\n",
	)
	got := parseSSEUsage("anthropic", raw)
	assert.Equal(t, 51000, got.PromptTokens, "prompt_tokens = input + cache_read")
	assert.Equal(t, 100, got.CompletionTokens)
	assert.Equal(t, "claude-3-5-sonnet-20241022", got.ModelName)
	assert.Equal(t, 50000, got.CacheReadInputTokens)
	assert.Equal(t, 0, got.CacheCreationInputTokens)
}

// Streaming with cache_creation_input_tokens
func TestParseSSEUsage_AnthropicCacheCreation(t *testing.T) {
	t.Parallel()
	raw := []byte(
		`data: {"type":"message_start","message":{"model":"claude-3-7-sonnet-20250219","usage":{"input_tokens":500,"output_tokens":0,"cache_read_input_tokens":0,"cache_creation_input_tokens":10000}}}` + "\n" +
			`data: {"type":"message_delta","usage":{"output_tokens":50}}` + "\n",
	)
	got := parseSSEUsage("anthropic", raw)
	assert.Equal(t, 10500, got.PromptTokens, "prompt_tokens = input + cache_creation")
	assert.Equal(t, 50, got.CompletionTokens)
	assert.Equal(t, 0, got.CacheReadInputTokens)
	assert.Equal(t, 10000, got.CacheCreationInputTokens)
}

// Streaming with both cache types
func TestParseSSEUsage_AnthropicBothCacheTypes(t *testing.T) {
	t.Parallel()
	raw := []byte(
		`data: {"type":"message_start","message":{"model":"claude-3-5-sonnet-20241022","usage":{"input_tokens":200,"output_tokens":0,"cache_read_input_tokens":5000,"cache_creation_input_tokens":1000}}}` + "\n",
	)
	got := parseSSEUsage("anthropic", raw)
	assert.Equal(t, 6200, got.PromptTokens)
	assert.Equal(t, 5000, got.CacheReadInputTokens)
	assert.Equal(t, 1000, got.CacheCreationInputTokens)
}

// AC-2: Non-streaming — response body with cache tokens
func TestParseUsage_AnthropicCacheTokens(t *testing.T) {
	body := []byte(`{"model":"claude-3-5-sonnet-20241022","usage":{"input_tokens":1000,"output_tokens":200,"cache_read_input_tokens":50000,"cache_creation_input_tokens":0}}`)
	got := parseUsage("anthropic", body)
	assert.Equal(t, 51000, got.PromptTokens, "prompt_tokens = input + cache_read")
	assert.Equal(t, 200, got.CompletionTokens)
	assert.Equal(t, "claude-3-5-sonnet-20241022", got.ModelName)
	assert.Equal(t, 50000, got.CacheReadInputTokens)
	assert.Equal(t, 0, got.CacheCreationInputTokens)
}

// Non-streaming with cache_creation_input_tokens
func TestParseUsage_AnthropicCacheCreation(t *testing.T) {
	body := []byte(`{"model":"claude-3-7-sonnet-20250219","usage":{"input_tokens":500,"output_tokens":100,"cache_read_input_tokens":0,"cache_creation_input_tokens":8000}}`)
	got := parseUsage("anthropic", body)
	assert.Equal(t, 8500, got.PromptTokens)
	assert.Equal(t, 100, got.CompletionTokens)
	assert.Equal(t, 8000, got.CacheCreationInputTokens)
}

// Non-streaming without cache tokens — behaves as before
func TestParseUsage_AnthropicNoCache(t *testing.T) {
	body := []byte(`{"model":"claude-3-5-sonnet-20241022","usage":{"input_tokens":42,"output_tokens":15}}`)
	got := parseUsage("anthropic", body)
	assert.Equal(t, 42, got.PromptTokens)
	assert.Equal(t, 15, got.CompletionTokens)
	assert.Equal(t, 0, got.CacheReadInputTokens)
	assert.Equal(t, 0, got.CacheCreationInputTokens)
}

// AC-7: OpenAI parse unaffected
func TestParseUsage_OpenAI_Unaffected(t *testing.T) {
	body := []byte(`{"model":"gpt-4o","usage":{"prompt_tokens":100,"completion_tokens":50}}`)
	got := parseUsage("openai", body)
	assert.Equal(t, 100, got.PromptTokens)
	assert.Equal(t, 50, got.CompletionTokens)
	assert.Equal(t, 0, got.CacheReadInputTokens)
	assert.Equal(t, 0, got.CacheCreationInputTokens)
}
