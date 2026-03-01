package handler

// HO-82: Failing tests — chat.go does NOT parse Anthropic OAuth rate limit headers
// from the upstream HTTP response, so RateLimitStore is never populated via the
// standard /v1/chat/completions path.
//
// Root cause: handleNonStreamingCompletion and handleStreamingCompletion receive
// *http.Response from Anthropic (including unified rate-limit headers) but never
// call callback.ParseAnthropicOAuthRateLimitHeaders / RateLimitStore.Set.
//
// These tests are expected to FAIL until the fix is implemented.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/callback"
	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/praxisllmlab/tianjiLLM/internal/model"
	"github.com/praxisllmlab/tianjiLLM/internal/provider/anthropic"
)

// anthropicOAuthRateLimitServer creates a test HTTP server that mimics an Anthropic
// upstream returning unified OAuth rate limit headers (5h/7d utilization).
func anthropicOAuthRateLimitServer(t *testing.T, util5h, util7d float64) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return unified OAuth rate limit headers that Anthropic sends for OAuth tokens
		w.Header().Set("anthropic-ratelimit-unified-status", "allowed")
		w.Header().Set("anthropic-ratelimit-unified-5h-status", "allowed")
		w.Header().Set("anthropic-ratelimit-unified-5h-utilization", fmt.Sprintf("%.4f", util5h))
		w.Header().Set("anthropic-ratelimit-unified-5h-reset", "1700000000")
		w.Header().Set("anthropic-ratelimit-unified-7d-status", "allowed")
		w.Header().Set("anthropic-ratelimit-unified-7d-utilization", fmt.Sprintf("%.4f", util7d))
		w.Header().Set("anthropic-ratelimit-unified-7d-reset", "1700001000")
		w.Header().Set("anthropic-ratelimit-unified-representative-claim", "five_hour")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		// Minimal valid Anthropic response body
		resp := map[string]any{
			"id":   "msg_test",
			"type": "message",
			"role": "assistant",
			"content": []map[string]any{
				{"type": "text", "text": "Hello!"},
			},
			"model":       "claude-3-5-sonnet-20241022",
			"stop_reason": "end_turn",
			"usage": map[string]int{
				"input_tokens":  10,
				"output_tokens": 5,
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
}

// chatHandlersWithRateLimitStore creates a Handlers instance wired to a fake upstream
// and an in-memory RateLimitStore.
func chatHandlersWithRateLimitStore(upstreamURL, apiKey string) (*Handlers, *callback.InMemoryRateLimitStore) {
	store := callback.NewInMemoryRateLimitStore()
	cfg := &config.ProxyConfig{
		ModelList: []config.ModelConfig{
			{
				ModelName: "anthropic-oauth-model",
				TianjiParams: config.TianjiParams{
					Model:   "anthropic/claude-3-5-sonnet-20241022",
					APIKey:  &apiKey,
					APIBase: &upstreamURL,
				},
			},
		},
	}
	h := &Handlers{
		Config:         cfg,
		RateLimitStore: store,
	}
	return h, store
}

// TestChatCompletion_AnthropicOAuth_PopulatesRateLimitStore_NonStreaming verifies that
// when a non-streaming /v1/chat/completions request goes through the chat handler to
// an Anthropic OAuth upstream, the RateLimitStore is populated with 5h/7d utilization.
//
// EXPECTED TO FAIL: handleNonStreamingCompletion does not read resp.Header for rate limits.
func TestChatCompletion_AnthropicOAuth_PopulatesRateLimitStore_NonStreaming(t *testing.T) {
	const util5h = 0.42
	const util7d = 0.73
	const oauthToken = "sk-ant-oat01-testtoken"

	upstream := anthropicOAuthRateLimitServer(t, util5h, util7d)
	defer upstream.Close()

	h, store := chatHandlersWithRateLimitStore(upstream.URL, oauthToken)

	reqBody := model.ChatCompletionRequest{
		Model: "anthropic-oauth-model",
		Messages: []model.Message{
			{Role: "user", Content: "Hello"},
		},
		
	}
	bodyBytes, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(string(bodyBytes)))
	r.Header.Set("Content-Type", "application/json")

	p := anthropic.NewWithBaseURL(upstream.URL)

	h.handleNonStreamingCompletion(w, r, p, &reqBody, oauthToken)

	// The RateLimitStore should now contain the parsed utilization for this token
	tokenKey := callback.RateLimitCacheKey(oauthToken)
	state, ok := store.Get(tokenKey)

	if !ok {
		t.Fatalf("FAIL: RateLimitStore has no entry for token key %q after non-streaming chat completion.\n"+
			"Root cause: handleNonStreamingCompletion does not call ParseAnthropicOAuthRateLimitHeaders.\n"+
			"This causes the Usage page to display '—' for 5h/7d utilization.", tokenKey)
	}

	if state.Unified5hUtilization < 0 {
		t.Errorf("FAIL: Unified5hUtilization = %v (sentinel -1 = missing header); want %.4f.\n"+
			"Root cause: resp.Header not read in handleNonStreamingCompletion.", state.Unified5hUtilization, util5h)
	} else if state.Unified5hUtilization != util5h {
		t.Errorf("Unified5hUtilization = %v; want %v", state.Unified5hUtilization, util5h)
	}

	if state.Unified7dUtilization < 0 {
		t.Errorf("FAIL: Unified7dUtilization = %v (sentinel -1 = missing header); want %.4f.\n"+
			"Root cause: resp.Header not read in handleNonStreamingCompletion.", state.Unified7dUtilization, util7d)
	} else if state.Unified7dUtilization != util7d {
		t.Errorf("Unified7dUtilization = %v; want %v", state.Unified7dUtilization, util7d)
	}
}

// TestChatCompletion_AnthropicOAuth_PopulatesRateLimitStore_Streaming verifies that
// streaming chat completions also populate the RateLimitStore with utilization data.
//
// EXPECTED TO FAIL: handleStreamingCompletion does not read resp.Header for rate limits.
func TestChatCompletion_AnthropicOAuth_PopulatesRateLimitStore_Streaming(t *testing.T) {
	const util5h = 0.55
	const util7d = 0.88
	const oauthToken = "sk-ant-oat01-streamtest"

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("anthropic-ratelimit-unified-status", "allowed")
		w.Header().Set("anthropic-ratelimit-unified-5h-status", "allowed")
		w.Header().Set("anthropic-ratelimit-unified-5h-utilization", fmt.Sprintf("%.4f", util5h))
		w.Header().Set("anthropic-ratelimit-unified-7d-status", "allowed")
		w.Header().Set("anthropic-ratelimit-unified-7d-utilization", fmt.Sprintf("%.4f", util7d))
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		fmt.Fprintf(w, "data: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_01\",\"type\":\"message\",\"role\":\"assistant\",\"content\":[],\"model\":\"claude-3-5-sonnet-20241022\",\"stop_reason\":null,\"stop_sequence\":null,\"usage\":{\"input_tokens\":10,\"output_tokens\":0}}}\n\n")
		fmt.Fprintf(w, "data: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n\n")
		fmt.Fprintf(w, "data: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"Hello\"}}\n\n")
		fmt.Fprintf(w, "data: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\",\"stop_sequence\":null},\"usage\":{\"output_tokens\":5}}\n\n")
		fmt.Fprintf(w, "data: {\"type\":\"message_stop\"}\n\n")
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}))
	defer upstream.Close()

	h, store := chatHandlersWithRateLimitStore(upstream.URL, oauthToken)

	reqBody := model.ChatCompletionRequest{
		Model: "anthropic-oauth-model",
		Messages: []model.Message{
			{Role: "user", Content: "Hello"},
		},
		Stream: boolPtr(true),
	}
	bodyBytes, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(string(bodyBytes)))
	r.Header.Set("Content-Type", "application/json")

	p := anthropic.NewWithBaseURL(upstream.URL)
	h.handleStreamingCompletion(w, r, p, &reqBody, oauthToken)

	tokenKey := callback.RateLimitCacheKey(oauthToken)
	state, ok := store.Get(tokenKey)

	if !ok {
		t.Fatalf("FAIL: RateLimitStore has no entry for token key %q after streaming chat completion.\n"+
			"Root cause: handleStreamingCompletion does not call ParseAnthropicOAuthRateLimitHeaders.", tokenKey)
	}

	if state.Unified5hUtilization < 0 {
		t.Errorf("FAIL: Unified5hUtilization = %v (sentinel -1); want %.4f.\n"+
			"Root cause: resp.Header not read in handleStreamingCompletion.", state.Unified5hUtilization, util5h)
	}
	if state.Unified7dUtilization < 0 {
		t.Errorf("FAIL: Unified7dUtilization = %v (sentinel -1); want %.4f.\n"+
			"Root cause: resp.Header not read in handleStreamingCompletion.", state.Unified7dUtilization, util7d)
	}
}
