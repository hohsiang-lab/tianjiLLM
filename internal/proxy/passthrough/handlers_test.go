package passthrough

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBaseLoggingHandler(t *testing.T) {
	h := &BaseLoggingHandler{Name: "test"}
	assert.Equal(t, "test", h.ProviderName())
	p, c := h.ParseUsage([]byte(`{}`))
	assert.Equal(t, 0, p)
	assert.Equal(t, 0, c)
}

func TestOpenAILoggingHandler(t *testing.T) {
	h := &OpenAILoggingHandler{}
	assert.Equal(t, "openai", h.ProviderName())

	body := `{"usage":{"prompt_tokens":10,"completion_tokens":20}}`
	p, c := h.ParseUsage([]byte(body))
	assert.Equal(t, 10, p)
	assert.Equal(t, 20, c)
}

func TestOpenAILoggingHandler_Invalid(t *testing.T) {
	h := &OpenAILoggingHandler{}
	p, c := h.ParseUsage([]byte("invalid"))
	assert.Equal(t, 0, p)
	assert.Equal(t, 0, c)
}

func TestAnthropicLoggingHandler(t *testing.T) {
	h := &AnthropicLoggingHandler{}
	assert.Equal(t, "anthropic", h.ProviderName())

	body := `{"usage":{"input_tokens":5,"output_tokens":15}}`
	p, c := h.ParseUsage([]byte(body))
	assert.Equal(t, 5, p)
	assert.Equal(t, 15, c)
}

func TestVertexAILoggingHandler(t *testing.T) {
	h := &VertexAILoggingHandler{}
	assert.Equal(t, "vertex_ai", h.ProviderName())

	body := `{"usageMetadata":{"promptTokenCount":8,"candidatesTokenCount":12}}`
	p, c := h.ParseUsage([]byte(body))
	assert.Equal(t, 8, p)
	assert.Equal(t, 12, c)
}

func TestCohereLoggingHandler(t *testing.T) {
	h := &CohereLoggingHandler{}
	assert.Equal(t, "cohere", h.ProviderName())

	body := `{"usage":{"prompt_tokens":3,"completion_tokens":7}}`
	p, c := h.ParseUsage([]byte(body))
	assert.Equal(t, 3, p)
	assert.Equal(t, 7, c)
}

func TestGeminiLoggingHandler(t *testing.T) {
	h := &GeminiLoggingHandler{}
	assert.Equal(t, "gemini", h.ProviderName())

	body := `{"usageMetadata":{"promptTokenCount":4,"candidatesTokenCount":6}}`
	p, c := h.ParseUsage([]byte(body))
	assert.Equal(t, 4, p)
	assert.Equal(t, 6, c)
}
