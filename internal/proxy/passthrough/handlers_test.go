package passthrough

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBaseLoggingHandler(t *testing.T) {
	h := &BaseLoggingHandler{Name: "test"}
	assert.Equal(t, "test", h.ProviderName())
	p, c, cr, cc, m := h.ParseUsage([]byte(`{}`))
	assert.Equal(t, 0, p)
	assert.Equal(t, 0, c)
	assert.Equal(t, 0, cr)
	assert.Equal(t, 0, cc)
	assert.Equal(t, "", m)
}

func TestOpenAILoggingHandler(t *testing.T) {
	h := &OpenAILoggingHandler{}
	assert.Equal(t, "openai", h.ProviderName())

	body := `{"model":"gpt-4o","usage":{"prompt_tokens":10,"completion_tokens":20}}`
	p, c, cr, cc, m := h.ParseUsage([]byte(body))
	assert.Equal(t, 10, p)
	assert.Equal(t, 20, c)
	assert.Equal(t, 0, cr)
	assert.Equal(t, 0, cc)
	assert.Equal(t, "gpt-4o", m)
}

func TestOpenAILoggingHandler_Invalid(t *testing.T) {
	h := &OpenAILoggingHandler{}
	p, c, _, _, _ := h.ParseUsage([]byte("invalid"))
	assert.Equal(t, 0, p)
	assert.Equal(t, 0, c)
}

func TestAnthropicLoggingHandler_ParseUsage_Basic(t *testing.T) {
	h := &AnthropicLoggingHandler{}
	assert.Equal(t, "anthropic", h.ProviderName())

	body := `{"model":"claude-sonnet-4-6","usage":{"input_tokens":5,"output_tokens":15}}`
	p, c, cr, cc, m := h.ParseUsage([]byte(body))
	assert.Equal(t, 5, p)
	assert.Equal(t, 15, c)
	assert.Equal(t, 0, cr)
	assert.Equal(t, 0, cc)
	assert.Equal(t, "claude-sonnet-4-6", m)
}

// T005: cache tokens correctly extracted; prompt = input + cacheRead + cacheCreation
func TestAnthropicLoggingHandler_ParseUsage_CacheTokens(t *testing.T) {
	h := &AnthropicLoggingHandler{}

	body := `{
		"model": "claude-sonnet-4-6",
		"usage": {
			"input_tokens": 100,
			"output_tokens": 50,
			"cache_read_input_tokens": 30,
			"cache_creation_input_tokens": 20
		}
	}`
	p, c, cr, cc, m := h.ParseUsage([]byte(body))
	// prompt = input(100) + cacheRead(30) + cacheCreation(20) = 150
	assert.Equal(t, 150, p)
	assert.Equal(t, 50, c)
	assert.Equal(t, 30, cr)
	assert.Equal(t, 20, cc)
	assert.Equal(t, "claude-sonnet-4-6", m)
}

// T005: model from message_start, output from message_delta
func TestAnthropicLoggingHandler_ParseSSEUsage_Streaming(t *testing.T) {
	h := &AnthropicLoggingHandler{}

	raw := []byte(
		"event: message_start\n" +
			`data: {"type":"message_start","message":{"model":"claude-sonnet-4-6","usage":{"input_tokens":200,"cache_read_input_tokens":0,"cache_creation_input_tokens":0}}}` + "\n\n" +
			"event: message_delta\n" +
			`data: {"type":"message_delta","usage":{"output_tokens":75}}` + "\n\n" +
			"event: message_stop\n" +
			`data: {"type":"message_stop"}` + "\n\n",
	)
	p, c, cr, cc, m := h.ParseSSEUsage(raw)
	assert.Equal(t, 200, p)
	assert.Equal(t, 75, c)
	assert.Equal(t, 0, cr)
	assert.Equal(t, 0, cc)
	assert.Equal(t, "claude-sonnet-4-6", m)
}

// T005: cache tokens in streaming message_start
func TestAnthropicLoggingHandler_ParseSSEUsage_CacheTokens(t *testing.T) {
	h := &AnthropicLoggingHandler{}

	raw := []byte(
		"event: message_start\n" +
			`data: {"type":"message_start","message":{"model":"claude-sonnet-4-6","usage":{"input_tokens":100,"cache_read_input_tokens":40,"cache_creation_input_tokens":10}}}` + "\n\n" +
			"event: message_delta\n" +
			`data: {"type":"message_delta","usage":{"output_tokens":30}}` + "\n\n",
	)
	p, c, cr, cc, m := h.ParseSSEUsage(raw)
	// prompt = 100 + 40 + 10 = 150
	assert.Equal(t, 150, p)
	assert.Equal(t, 30, c)
	assert.Equal(t, 40, cr)
	assert.Equal(t, 10, cc)
	assert.Equal(t, "claude-sonnet-4-6", m)
}

func TestVertexAILoggingHandler(t *testing.T) {
	h := &VertexAILoggingHandler{}
	assert.Equal(t, "vertex_ai", h.ProviderName())

	body := `{"usageMetadata":{"promptTokenCount":8,"candidatesTokenCount":12}}`
	p, c, cr, cc, m := h.ParseUsage([]byte(body))
	assert.Equal(t, 8, p)
	assert.Equal(t, 12, c)
	assert.Equal(t, 0, cr)
	assert.Equal(t, 0, cc)
	assert.Equal(t, "", m)
}

// T006: Vertex AI SSE — last chunk wins
func TestVertexAILoggingHandler_ParseSSEUsage(t *testing.T) {
	h := &VertexAILoggingHandler{}

	raw := []byte(
		`data: {"usageMetadata":{"promptTokenCount":10,"candidatesTokenCount":5}}` + "\n\n" +
			`data: {"usageMetadata":{"promptTokenCount":10,"candidatesTokenCount":20}}` + "\n\n",
	)
	p, c, _, _, _ := h.ParseSSEUsage(raw)
	// last chunk wins
	assert.Equal(t, 10, p)
	assert.Equal(t, 20, c)
}

func TestCohereLoggingHandler(t *testing.T) {
	h := &CohereLoggingHandler{}
	assert.Equal(t, "cohere", h.ProviderName())

	body := `{"usage":{"prompt_tokens":3,"completion_tokens":7}}`
	p, c, _, _, _ := h.ParseUsage([]byte(body))
	assert.Equal(t, 3, p)
	assert.Equal(t, 7, c)
}

func TestGeminiLoggingHandler(t *testing.T) {
	h := &GeminiLoggingHandler{}
	assert.Equal(t, "gemini", h.ProviderName())

	body := `{"usageMetadata":{"promptTokenCount":4,"candidatesTokenCount":6}}`
	p, c, _, _, _ := h.ParseUsage([]byte(body))
	assert.Equal(t, 4, p)
	assert.Equal(t, 6, c)
}

// T006: Gemini streaming — last chunk wins
func TestGeminiLoggingHandler_ParseSSEUsage(t *testing.T) {
	h := &GeminiLoggingHandler{}

	raw := []byte(
		`data: {"modelVersion":"gemini-2.0-flash","usageMetadata":{"promptTokenCount":50,"candidatesTokenCount":10}}` + "\n\n" +
			`data: {"modelVersion":"gemini-2.0-flash","usageMetadata":{"promptTokenCount":50,"candidatesTokenCount":25}}` + "\n\n",
	)
	p, c, _, _, m := h.ParseSSEUsage(raw)
	assert.Equal(t, 50, p)
	assert.Equal(t, 25, c)
	assert.Equal(t, "gemini-2.0-flash", m)
}
