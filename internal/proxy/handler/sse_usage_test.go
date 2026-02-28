package handler

import (
	"testing"
)

// Bug #1 (HO-60): Anthropic streaming — input_tokens lives in message.usage.input_tokens (nested),
// output_tokens in root-level usage on message_delta.
func TestParseSSEUsage_Anthropic_MessageStart_InputTokens(t *testing.T) {
	raw := []byte(
		"event: message_start\n" +
			`data: {"type":"message_start","message":{"model":"claude-3-5-sonnet-20241022","usage":{"input_tokens":42}}}` + "\n\n" +
			"event: message_delta\n" +
			`data: {"type":"message_delta","usage":{"output_tokens":15}}` + "\n\n",
	)

	prompt, completion, model := parseSSEUsage("anthropic", raw)

	if prompt != 42 {
		t.Errorf("prompt tokens: got %d, want 42 (nested message.usage.input_tokens)", prompt)
	}
	if completion != 15 {
		t.Errorf("completion tokens: got %d, want 15", completion)
	}
	if model != "claude-3-5-sonnet-20241022" {
		t.Errorf("model: got %q, want %q", model, "claude-3-5-sonnet-20241022")
	}
}

// Bug #2 (HO-60): Gemini streaming — switch had no "gemini" case, usage was always 0.
func TestParseSSEUsage_Gemini_UsageMetadata(t *testing.T) {
	raw := []byte(
		`data: {"candidates":[{"content":{"parts":[{"text":"hello"}]}}],"modelVersion":"gemini-2.0-flash","usageMetadata":{"promptTokenCount":100,"candidatesTokenCount":50}}` + "\n\n",
	)

	prompt, completion, model := parseSSEUsage("gemini", raw)

	if prompt != 100 {
		t.Errorf("prompt tokens: got %d, want 100", prompt)
	}
	if completion != 50 {
		t.Errorf("completion tokens: got %d, want 50", completion)
	}
	if model != "gemini-2.0-flash" {
		t.Errorf("model: got %q, want %q", model, "gemini-2.0-flash")
	}
}

// Baseline: OpenAI streaming should work correctly.
func TestParseSSEUsage_OpenAI_FinalChunk(t *testing.T) {
	raw := []byte(
		`data: {"id":"chatcmpl-abc","model":"gpt-4o","choices":[],"usage":{"prompt_tokens":200,"completion_tokens":80}}` + "\n\n",
	)

	prompt, completion, model := parseSSEUsage("openai", raw)

	if prompt != 200 {
		t.Errorf("prompt tokens: got %d, want 200", prompt)
	}
	if completion != 80 {
		t.Errorf("completion tokens: got %d, want 80", completion)
	}
	if model != "gpt-4o" {
		t.Errorf("model: got %q, want %q", model, "gpt-4o")
	}
}
