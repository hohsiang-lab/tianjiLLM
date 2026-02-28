package handler

// Tests for HO-71: Integration spend tracking with cache tokens (T14–T15).
// These tests are intentionally FAILING until the feature is implemented by 魯班.

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/praxisllmlab/tianjiLLM/internal/callback"
	"github.com/stretchr/testify/assert"
)

// ─── T14: SSE with cache → spend includes cache cost ─────────────────────────

// TestSpendTracking_CacheRead_CostIncludesCacheFee verifies that when an
// Anthropic SSE response includes cache_read_input_tokens, the Cost recorded
// in LogData reflects the cache read fee, not just the base input cost.
//
// Expected (claude-sonnet-4, 1 input + 50K cache_read + 500 output):
//
//	input:      1 × 3e-06    = $0.000003
//	cache_read: 50000 × 3e-07 = $0.015000
//	output:     500 × 1.5e-05 = $0.007500
//	Total ≈ $0.022503
func TestSpendTracking_CacheRead_CostIncludesCacheFee(t *testing.T) {
	t.Parallel()

	const sseBody = "" +
		"data: {\"type\":\"message_start\",\"message\":{\"model\":\"claude-sonnet-4-20250514\",\"usage\":{\"input_tokens\":1,\"cache_read_input_tokens\":50000,\"cache_creation_input_tokens\":0}}}\n\n" +
		"data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"hello\"}}\n\n" +
		"data: {\"type\":\"message_delta\",\"usage\":{\"output_tokens\":500}}\n\n" +
		"data: {\"type\":\"message_stop\"}\n\n"

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(sseBody))
	}))
	defer upstream.Close()

	spy := newSpyLogger()
	reg := callback.NewRegistry()
	reg.Register(spy)

	h := nativeTestHandlers(upstream.URL, "anthropic", nil)
	h.Callbacks = reg

	req := httptest.NewRequest("POST", "/v1/messages", strings.NewReader(`{"model":"claude-sonnet-4-20250514"}`))
	w := httptest.NewRecorder()
	h.nativeProxy(w, req, "anthropic")

	spy.waitCalled(t, 2*time.Second)

	spy.mu.Lock()
	data := spy.calls[0]
	spy.mu.Unlock()

	// 1 input × 3e-06 + 50000 cache_read × 3e-07 + 500 completion × 1.5e-05
	// = 0.000003 + 0.015 + 0.0075 = 0.022503
	assert.InDelta(t, 0.022503, data.Cost, 1e-6, "cost should match cache pricing")

	// Also verify prompt tokens are correct
	assert.Equal(t, 50001, data.PromptTokens,
		"T14: PromptTokens must be 50001 (1 + 50000 cache_read)")
}

// ─── T15: SSE without cache → backward-compat spend ──────────────────────────

// TestSpendTracking_NoCache_BackwardCompat verifies that cost calculation for
// requests without cache tokens continues to work correctly after the fix.
func TestSpendTracking_NoCache_BackwardCompat(t *testing.T) {
	t.Parallel()

	const sseBody = "" +
		"data: {\"type\":\"message_start\",\"message\":{\"model\":\"claude-sonnet-4-20250514\",\"usage\":{\"input_tokens\":100,\"output_tokens\":0}}}\n\n" +
		"data: {\"type\":\"message_delta\",\"usage\":{\"output_tokens\":50}}\n\n"

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(sseBody))
	}))
	defer upstream.Close()

	spy := newSpyLogger()
	reg := callback.NewRegistry()
	reg.Register(spy)

	h := nativeTestHandlers(upstream.URL, "anthropic", nil)
	h.Callbacks = reg

	req := httptest.NewRequest("POST", "/v1/messages", strings.NewReader(`{"model":"claude-sonnet-4-20250514"}`))
	w := httptest.NewRecorder()
	h.nativeProxy(w, req, "anthropic")

	spy.waitCalled(t, 2*time.Second)

	spy.mu.Lock()
	data := spy.calls[0]
	spy.mu.Unlock()

	// Backward compat: prompt should be 100, no cache tokens
	assert.Equal(t, 100, data.PromptTokens,
		"T15: PromptTokens should be 100 (no cache; backward compat)")
	assert.Equal(t, 0, data.CacheReadInputTokens,
		"T15: CacheReadInputTokens should be 0")
	assert.Equal(t, 0, data.CacheCreationInputTokens,
		"T15: CacheCreationInputTokens should be 0")

	// Cost must not be negative or unreasonably large
	assert.GreaterOrEqual(t, data.Cost, 0.0,
		"T15: Cost must be >= 0 for non-cache request")
}
