package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/praxisllmlab/tianjiLLM/internal/model"
	anthropicprovider "github.com/praxisllmlab/tianjiLLM/internal/provider/anthropic"
	openaiprovider "github.com/praxisllmlab/tianjiLLM/internal/provider/openai"
	"github.com/praxisllmlab/tianjiLLM/internal/ratelimit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// AC#2: isAnthropicProvider returns false for OpenAI provider.
func TestIsAnthropicProvider_OpenAI_ReturnsFalse(t *testing.T) {
	p := openaiprovider.New()
	assert.False(t, isAnthropicProvider(p))
}

// AC#2: isAnthropicProvider returns true for Anthropic provider.
func TestIsAnthropicProvider_Anthropic_ReturnsTrue(t *testing.T) {
	p := anthropicprovider.New()
	assert.True(t, isAnthropicProvider(p))
}

// AC#2: non-Anthropic provider does not update the Store.
func TestHandleNonStreaming_NonAnthropicProvider_StoreEmpty(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("anthropic-ratelimit-tokens-limit", "800000")
		w.Header().Set("anthropic-ratelimit-tokens-remaining", "100000")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":      "chatcmpl-test",
			"object":  "chat.completion",
			"created": 1700000000,
			"model":   "gpt-4o",
			"choices": []map[string]any{
				{
					"index":         0,
					"message":       map[string]any{"role": "assistant", "content": "Hi"},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]any{"prompt_tokens": 5, "completion_tokens": 3, "total_tokens": 8},
		})
	}))
	defer upstream.Close()

	store := ratelimit.NewStore()
	apiKey := "sk-test-openai-key-1234"
	cfg := &config.ProxyConfig{}
	h := &Handlers{Config: cfg, RateLimitStore: store}

	p := openaiprovider.NewWithBaseURL(upstream.URL)
	req := &model.ChatCompletionRequest{
		Model:    "gpt-4o",
		Messages: []model.Message{{Role: "user", Content: "hi"}},
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	h.handleNonStreamingCompletion(w, r, p, req, apiKey)

	assert.Empty(t, store.All(), "non-Anthropic provider must not update RateLimitStore")
}

// AC#7: non-streaming Anthropic path updates the Store.
func TestHandleNonStreaming_AnthropicProvider_StoreUpdated(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("anthropic-ratelimit-tokens-limit", "800000")
		w.Header().Set("anthropic-ratelimit-tokens-remaining", "650000")
		w.Header().Set("anthropic-ratelimit-requests-limit", "1000")
		w.Header().Set("anthropic-ratelimit-requests-remaining", "800")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":    "msg_test",
			"type":  "message",
			"role":  "assistant",
			"model": "claude-3-5-sonnet-20241022",
			"content": []map[string]any{
				{"type": "text", "text": "Hello!"},
			},
			"stop_reason": "end_turn",
			"usage":       map[string]any{"input_tokens": 10, "output_tokens": 5},
		})
	}))
	defer upstream.Close()

	store := ratelimit.NewStore()
	apiKey := "sk-ant-test-1234"
	cfg := &config.ProxyConfig{}
	h := &Handlers{Config: cfg, RateLimitStore: store}

	p := anthropicprovider.NewWithBaseURL(upstream.URL)
	req := &model.ChatCompletionRequest{
		Model:    "claude-3-5-sonnet-20241022",
		Messages: []model.Message{{Role: "user", Content: "hi"}},
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	h.handleNonStreamingCompletion(w, r, p, req, apiKey)

	all := store.All()
	require.Len(t, all, 1, "Anthropic non-streaming must update RateLimitStore")
	for _, st := range all {
		assert.Equal(t, 800000, st.TokensLimit)
		assert.Equal(t, 650000, st.TokensRemaining)
	}
}

// AC#7: streaming Anthropic path updates the Store.
func TestHandleStreaming_AnthropicProvider_StoreUpdated(t *testing.T) {
	ssePayload := strings.Join([]string{
		`data: {"type":"message_start","message":{"id":"msg_t1","model":"claude-3-5-sonnet-20241022","usage":{"input_tokens":10}}}`,
		``,
		`data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
		``,
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hi!"}}`,
		``,
		`data: {"type":"content_block_stop","index":0}`,
		``,
		`data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":3}}`,
		``,
		`data: {"type":"message_stop"}`,
		``,
	}, "\n")

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("anthropic-ratelimit-tokens-limit", "500000")
		w.Header().Set("anthropic-ratelimit-tokens-remaining", "400000")
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(ssePayload))
	}))
	defer upstream.Close()

	store := ratelimit.NewStore()
	apiKey := "sk-ant-test-5678"
	cfg := &config.ProxyConfig{}
	h := &Handlers{Config: cfg, RateLimitStore: store}

	p := anthropicprovider.NewWithBaseURL(upstream.URL)
	streaming := true
	req := &model.ChatCompletionRequest{
		Model:    "claude-3-5-sonnet-20241022",
		Messages: []model.Message{{Role: "user", Content: "hi"}},
		Stream:   &streaming,
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	h.handleStreamingCompletion(w, r, p, req, apiKey)

	time.Sleep(50 * time.Millisecond)

	all := store.All()
	require.Len(t, all, 1, "Anthropic streaming must update RateLimitStore")
	for _, st := range all {
		assert.Equal(t, 500000, st.TokensLimit)
		assert.Equal(t, 400000, st.TokensRemaining)
	}
}

// RateLimitStatus handler — nil store returns empty providers map.
func TestRateLimitStatus_NilStore(t *testing.T) {
	handlers := &Handlers{RateLimitStore: nil}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/internal/ratelimit", nil)
	handlers.RateLimitStatus(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	providers, ok := resp["providers"].(map[string]any)
	require.True(t, ok)
	assert.Empty(t, providers)
}

// RateLimitStatus handler — returns store data with correct schema.
func TestRateLimitStatus_WithData(t *testing.T) {
	store := ratelimit.NewStore()
	h := http.Header{}
	h.Set("anthropic-ratelimit-tokens-limit", "1000000")
	h.Set("anthropic-ratelimit-tokens-remaining", "750000")
	h.Set("anthropic-ratelimit-requests-limit", "2000")
	h.Set("anthropic-ratelimit-requests-remaining", "1500")
	store.ParseAndUpdate("anthropic/test", h)

	handlers := &Handlers{RateLimitStore: store}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/internal/ratelimit", nil)
	handlers.RateLimitStatus(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	providers, ok := resp["providers"].(map[string]any)
	require.True(t, ok, "response must have 'providers' key")
	require.Len(t, providers, 1)

	entry, ok := providers["anthropic/test"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, float64(1000000), entry["tokens_limit"])
	assert.Equal(t, float64(750000), entry["tokens_remaining"])
	assert.Equal(t, float64(2000), entry["requests_limit"])
	assert.Equal(t, float64(1500), entry["requests_remaining"])
	assert.Contains(t, entry, "tokens_reset")
	assert.Contains(t, entry, "updated_at")
}

// ---- helpers ----

// testAnthropicProvider wraps the real Anthropic provider to spoof URL detection while
// still sending actual HTTP requests to the test server.
// isAnthropicProvider checks GetRequestURL for "anthropic.com"; TransformRequest
// uses inner.GetRequestURL (the real test server URL) so requests go to the test server.
type testAnthropicProvider struct {
	inner     *anthropicprovider.Provider // sends requests to test server
	detectURL string                      // returned by GetRequestURL for isAnthropicProvider
}

func newTestAnthropicProvider(testServerURL string) *testAnthropicProvider {
	return &testAnthropicProvider{
		inner:     anthropicprovider.NewWithBaseURL(testServerURL),
		detectURL: "https://api.anthropic.com/v1/messages",
	}
}

// GetRequestURL returns a URL containing "anthropic.com" so isAnthropicProvider returns true.
func (p *testAnthropicProvider) GetRequestURL(_ string) string { return p.detectURL }

func (p *testAnthropicProvider) TransformRequest(ctx context.Context, req *model.ChatCompletionRequest, apiKey string) (*http.Request, error) {
	return p.inner.TransformRequest(ctx, req, apiKey)
}
func (p *testAnthropicProvider) TransformResponse(ctx context.Context, resp *http.Response) (*model.ModelResponse, error) {
	return p.inner.TransformResponse(ctx, resp)
}
func (p *testAnthropicProvider) TransformStreamChunk(ctx context.Context, data []byte) (*model.StreamChunk, bool, error) {
	return p.inner.TransformStreamChunk(ctx, data)
}
func (p *testAnthropicProvider) GetSupportedParams() []string { return p.inner.GetSupportedParams() }
func (p *testAnthropicProvider) MapParams(params map[string]any) map[string]any {
	return p.inner.MapParams(params)
}
func (p *testAnthropicProvider) SetupHeaders(req *http.Request, apiKey string) {
	p.inner.SetupHeaders(req, apiKey)
}