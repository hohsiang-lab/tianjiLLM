package token

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCountText_GPT4o(t *testing.T) {
	c := New()
	count := c.CountText("gpt-4o", "Hello, world!")
	assert.Greater(t, count, 0)
	// "Hello, world!" is typically 4 tokens
	assert.InDelta(t, 4, count, 2)
}

func TestCountText_GPT35Turbo(t *testing.T) {
	c := New()
	count := c.CountText("gpt-3.5-turbo", "Hello, world!")
	assert.Greater(t, count, 0)
}

func TestCountText_NonOpenAI_ReturnsNegOne(t *testing.T) {
	c := New()
	assert.Equal(t, -1, c.CountText("claude-3-sonnet", "Hello"))
	assert.Equal(t, -1, c.CountText("anthropic/claude-3", "Hello"))
}

func TestCountText_UnknownGPT_FallsBack(t *testing.T) {
	c := New()
	// Unknown GPT model should still work with o200k_base
	count := c.CountText("gpt-future", "Hello, world!")
	assert.Greater(t, count, 0)
}

func TestCountMessages(t *testing.T) {
	c := New()
	msgs := []Message{
		{Role: "system", Content: "You are a helpful assistant."},
		{Role: "user", Content: "Hello!"},
	}
	count := c.CountMessages("gpt-4o", msgs)
	assert.Greater(t, count, 0)
	// Should be more than just the text tokens due to overhead
	textOnly := c.CountText("gpt-4o", "You are a helpful assistant.Hello!")
	assert.Greater(t, count, textOnly)
}

func TestCountMessages_NonOpenAI_ReturnsNegOne(t *testing.T) {
	c := New()
	msgs := []Message{{Role: "user", Content: "Hello"}}
	assert.Equal(t, -1, c.CountMessages("claude-3", msgs))
}

func TestModelToEncoding(t *testing.T) {
	tests := []struct {
		model    string
		expected string
	}{
		{"gpt-4o", "o200k_base"},
		{"gpt-4o-mini", "o200k_base"},
		{"gpt-4.1", "o200k_base"},
		{"gpt-4.5-preview", "o200k_base"},
		{"o1-preview", "o200k_base"},
		{"o3-mini", "o200k_base"},
		{"chatgpt-4o-latest", "o200k_base"},
		{"gpt-4-turbo", "cl100k_base"},
		{"gpt-4", "cl100k_base"},
		{"gpt-3.5-turbo", "cl100k_base"},
		{"openai/gpt-4o", "o200k_base"},
		{"claude-3-sonnet", ""},
		{"llama-3", ""},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			assert.Equal(t, tt.expected, modelToEncoding(tt.model))
		})
	}
}

func TestEncoderCaching(t *testing.T) {
	c := New()
	// Call twice to test caching path
	count1 := c.CountText("gpt-4o", "Hello")
	count2 := c.CountText("gpt-4o", "Hello")
	assert.Equal(t, count1, count2)
	assert.Len(t, c.encoders, 1)
}
