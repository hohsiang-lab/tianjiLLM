package handler

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ──────────────────────────────────────────────────────────────
// parseSSEUsage — Anthropic
// ──────────────────────────────────────────────────────────────

func TestParseSSEUsage_Anthropic_FullFlow(t *testing.T) {
	t.Parallel()
	// message_start has model + input_tokens; message_delta has output_tokens
	raw := []byte(
		"data: {\"type\":\"message_start\",\"message\":{\"model\":\"claude-sonnet-4-20250514\",\"usage\":{\"input_tokens\":42,\"output_tokens\":0}}}\n" +
			"data: {\"type\":\"content_block_delta\",\"delta\":{\"text\":\"hello\"}}\n" +
			"data: {\"type\":\"message_delta\",\"usage\":{\"input_tokens\":0,\"output_tokens\":15}}\n",
	)
	got := parseSSEUsage("anthropic", raw)

	prompt, completion, model := got.PromptTokens, got.CompletionTokens, got.ModelName
	assert.Equal(t, 42, prompt)
	assert.Equal(t, 15, completion)
	assert.Equal(t, "claude-sonnet-4-20250514", model)
}

func TestParseSSEUsage_Anthropic_OnlyMessageStart(t *testing.T) {
	t.Parallel()
	// Only message_start, no message_delta yet
	raw := []byte("data: {\"type\":\"message_start\",\"message\":{\"model\":\"claude-3-haiku\",\"usage\":{\"input_tokens\":100,\"output_tokens\":0}}}\n")
	got := parseSSEUsage("anthropic", raw)

	prompt, completion, model := got.PromptTokens, got.CompletionTokens, got.ModelName
	assert.Equal(t, 100, prompt)
	assert.Equal(t, 0, completion)
	assert.Equal(t, "claude-3-haiku", model)
}

func TestParseSSEUsage_Anthropic_EmptyPayload(t *testing.T) {
	t.Parallel()
	got := parseSSEUsage("anthropic", []byte{})

	prompt, completion, model := got.PromptTokens, got.CompletionTokens, got.ModelName
	assert.Equal(t, 0, prompt)
	assert.Equal(t, 0, completion)
	assert.Equal(t, "", model)
}

func TestParseSSEUsage_Anthropic_NoUsageFields(t *testing.T) {
	t.Parallel()
	raw := []byte("data: {\"type\":\"content_block_delta\",\"delta\":{\"text\":\"hi\"}}\n")
	got := parseSSEUsage("anthropic", raw)

	prompt, completion, model := got.PromptTokens, got.CompletionTokens, got.ModelName
	assert.Equal(t, 0, prompt)
	assert.Equal(t, 0, completion)
	assert.Equal(t, "", model)
}

func TestParseSSEUsage_Anthropic_MalformedJSON(t *testing.T) {
	t.Parallel()
	raw := []byte("data: {not valid json}\ndata: {\"type\":\"message_start\",\"message\":{\"model\":\"claude-3\",\"usage\":{\"input_tokens\":5,\"output_tokens\":0}}}\n")
	got := parseSSEUsage("anthropic", raw)

	prompt, model := got.PromptTokens, got.ModelName
	assert.Equal(t, 5, prompt, "should skip malformed line and parse valid one")
	assert.Equal(t, "claude-3", model)
}

func TestParseSSEUsage_Anthropic_NonDataLines(t *testing.T) {
	t.Parallel()
	// SSE can have event:, id:, retry: lines — should be ignored
	raw := []byte(
		"event: message_start\n" +
			"data: {\"type\":\"message_start\",\"message\":{\"model\":\"claude-3\",\"usage\":{\"input_tokens\":10,\"output_tokens\":0}}}\n" +
			": comment line\n" +
			"id: 123\n" +
			"data: {\"type\":\"message_delta\",\"usage\":{\"input_tokens\":0,\"output_tokens\":7}}\n",
	)
	got := parseSSEUsage("anthropic", raw)

	prompt, completion := got.PromptTokens, got.CompletionTokens
	assert.Equal(t, 10, prompt)
	assert.Equal(t, 7, completion)
}

func TestParseSSEUsage_Anthropic_MultipleMessageDelta_LastWins(t *testing.T) {
	t.Parallel()
	// If there are multiple message_delta events, the last one with output_tokens > 0 wins
	raw := []byte(
		"data: {\"type\":\"message_start\",\"message\":{\"model\":\"claude-3\",\"usage\":{\"input_tokens\":20,\"output_tokens\":0}}}\n" +
			"data: {\"type\":\"message_delta\",\"usage\":{\"input_tokens\":0,\"output_tokens\":5}}\n" +
			"data: {\"type\":\"message_delta\",\"usage\":{\"input_tokens\":0,\"output_tokens\":30}}\n",
	)
	got := parseSSEUsage("anthropic", raw)

	completion := got.CompletionTokens
	assert.Equal(t, 30, completion, "last message_delta should overwrite")
}

// ──────────────────────────────────────────────────────────────
// parseSSEUsage — Gemini
// ──────────────────────────────────────────────────────────────

func TestParseSSEUsage_Gemini_FullFlow(t *testing.T) {
	t.Parallel()
	raw := []byte(
		"data: {\"modelVersion\":\"gemini-2.0-flash\",\"usageMetadata\":{\"promptTokenCount\":10,\"candidatesTokenCount\":5}}\n" +
			"data: {\"modelVersion\":\"gemini-2.0-flash\",\"usageMetadata\":{\"promptTokenCount\":10,\"candidatesTokenCount\":20}}\n",
	)
	got := parseSSEUsage("gemini", raw)

	prompt, completion, model := got.PromptTokens, got.CompletionTokens, got.ModelName
	assert.Equal(t, 10, prompt)
	assert.Equal(t, 20, completion, "last chunk wins")
	assert.Equal(t, "gemini-2.0-flash", model)
}

func TestParseSSEUsage_Gemini_EmptyPayload(t *testing.T) {
	t.Parallel()
	got := parseSSEUsage("gemini", []byte{})

	prompt, completion, model := got.PromptTokens, got.CompletionTokens, got.ModelName
	assert.Equal(t, 0, prompt)
	assert.Equal(t, 0, completion)
	assert.Equal(t, "", model)
}

func TestParseSSEUsage_Gemini_NoUsageMetadata(t *testing.T) {
	t.Parallel()
	raw := []byte("data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"hi\"}]}}],\"modelVersion\":\"gemini-2.0-flash\"}\n")
	got := parseSSEUsage("gemini", raw)

	prompt, completion, model := got.PromptTokens, got.CompletionTokens, got.ModelName
	assert.Equal(t, 0, prompt)
	assert.Equal(t, 0, completion)
	assert.Equal(t, "gemini-2.0-flash", model)
}

func TestParseSSEUsage_Gemini_SingleChunk(t *testing.T) {
	t.Parallel()
	raw := []byte("data: {\"modelVersion\":\"gemini-1.5-pro\",\"usageMetadata\":{\"promptTokenCount\":50,\"candidatesTokenCount\":100}}\n")
	got := parseSSEUsage("gemini", raw)

	prompt, completion, model := got.PromptTokens, got.CompletionTokens, got.ModelName
	assert.Equal(t, 50, prompt)
	assert.Equal(t, 100, completion)
	assert.Equal(t, "gemini-1.5-pro", model)
}

// ──────────────────────────────────────────────────────────────
// parseSSEUsage — Unknown provider
// ──────────────────────────────────────────────────────────────

func TestParseSSEUsage_UnknownProvider(t *testing.T) {
	t.Parallel()
	raw := []byte("data: {\"usage\":{\"prompt_tokens\":10,\"completion_tokens\":5}}\n")
	got := parseSSEUsage("unknownprovider", raw)

	prompt, completion, model := got.PromptTokens, got.CompletionTokens, got.ModelName
	assert.Equal(t, 0, prompt, "unknown provider should not parse anything")
	assert.Equal(t, 0, completion)
	assert.Equal(t, "", model)
}

func TestParseSSEUsage_EmptyProvider(t *testing.T) {
	t.Parallel()
	raw := []byte("data: {\"type\":\"message_start\",\"message\":{\"model\":\"test\",\"usage\":{\"input_tokens\":10}}}\n")
	got := parseSSEUsage("", raw)

	prompt := got.PromptTokens
	assert.Equal(t, 0, prompt)
}
