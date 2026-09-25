package handler

// Tests for HO-71: Anthropic cache token parsing (T01–T05).
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

// ─── Helpers ────────────────────────────────────────────────────────────────

func nativeHandlersWithSpy(upstreamURL, providerName string, spy *spyLogger) *Handlers {
	h := nativeTestHandlers(upstreamURL, providerName, nil)
	reg := callback.NewRegistry()
	reg.Register(spy)
	h.Callbacks = reg
	return h
}

// ─── T01: SSE 50K cache_read → prompt=50001, CacheReadInputTokens=50000 ──────────

func TestCacheSSE_CacheRead_PromptIncludesAllTokens(t *testing.T) {
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
	h := nativeHandlersWithSpy(upstream.URL, "anthropic", spy)

	req := httptest.NewRequest("POST", "/v1/messages", strings.NewReader(`{"model":"claude-sonnet-4-20250514"}`))
	w := httptest.NewRecorder()
	h.nativeProxy(w, req, "anthropic")

	spy.waitCalled(t, 2*time.Second)

	spy.mu.Lock()
	data := spy.calls[0]
	spy.mu.Unlock()

	assert.Equal(t, 50001, data.PromptTokens,
		"T01: PromptTokens must equal input_tokens + cache_read_input_tokens (1 + 50000)")
	assert.Equal(t, 50000, data.CacheReadInputTokens,
		"T01: CacheReadInputTokens must be 50000")
}

// ─── T02: SSE no cache → backward-compat ────────────────────────────────────

func TestCacheSSE_NoCache_BackwardCompat(t *testing.T) {
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
	h := nativeHandlersWithSpy(upstream.URL, "anthropic", spy)

	req := httptest.NewRequest("POST", "/v1/messages", strings.NewReader(`{}`))
	w := httptest.NewRecorder()
	h.nativeProxy(w, req, "anthropic")

	spy.waitCalled(t, 2*time.Second)

	spy.mu.Lock()
	data := spy.calls[0]
	spy.mu.Unlock()

	assert.Equal(t, 100, data.PromptTokens, "T02: PromptTokens should be 100 (no cache; backward compat)")
	assert.Equal(t, 0, data.CacheReadInputTokens, "T02: CacheReadInputTokens should be 0 when no cache")
	assert.Equal(t, 0, data.CacheCreationInputTokens, "T02: CacheCreationInputTokens should be 0 when no cache")
}

// ─── T03: SSE cache_creation → prompt=input+creation ────────────────────────

func TestCacheSSE_CacheCreation_PromptIncludesCreation(t *testing.T) {
	t.Parallel()

	const sseBody = "" +
		"data: {\"type\":\"message_start\",\"message\":{\"model\":\"claude-sonnet-4-20250514\",\"usage\":{\"input_tokens\":5,\"cache_read_input_tokens\":0,\"cache_creation_input_tokens\":2000}}}\n\n" +
		"data: {\"type\":\"message_delta\",\"usage\":{\"output_tokens\":20}}\n\n"

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(sseBody))
	}))
	defer upstream.Close()

	spy := newSpyLogger()
	h := nativeHandlersWithSpy(upstream.URL, "anthropic", spy)

	req := httptest.NewRequest("POST", "/v1/messages", strings.NewReader(`{}`))
	w := httptest.NewRecorder()
	h.nativeProxy(w, req, "anthropic")

	spy.waitCalled(t, 2*time.Second)

	spy.mu.Lock()
	data := spy.calls[0]
	spy.mu.Unlock()

	assert.Equal(t, 2005, data.PromptTokens,
		"T03: PromptTokens must equal input_tokens + cache_creation_input_tokens (5 + 2000)")
	assert.Equal(t, 2000, data.CacheCreationInputTokens,
		"T03: CacheCreationInputTokens must be 2000")
}

// ─── T04: Non-streaming with cache_read ─────────────────────────────────────

func TestCacheNonStreaming_CacheRead_PromptIncludesAll(t *testing.T) {
	t.Parallel()

	const body = `{"id":"msg_01","type":"message","model":"claude-sonnet-4-20250514","usage":{"input_tokens":1,"cache_read_input_tokens":50000,"cache_creation_input_tokens":0,"output_tokens":500}}`

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(body))
	}))
	defer upstream.Close()

	spy := newSpyLogger()
	h := nativeHandlersWithSpy(upstream.URL, "anthropic", spy)

	req := httptest.NewRequest("POST", "/v1/messages", strings.NewReader(`{}`))
	w := httptest.NewRecorder()
	h.nativeProxy(w, req, "anthropic")

	spy.waitCalled(t, 2*time.Second)

	spy.mu.Lock()
	data := spy.calls[0]
	spy.mu.Unlock()

	assert.Equal(t, 50001, data.PromptTokens,
		"T04: PromptTokens must equal input_tokens + cache_read_input_tokens (1 + 50000)")
	assert.Equal(t, 50000, data.CacheReadInputTokens,
		"T04: CacheReadInputTokens must be 50000")
}

// ─── T05: Non-streaming no cache → backward-compat ──────────────────────────

func TestCacheNonStreaming_NoCache_BackwardCompat(t *testing.T) {
	t.Parallel()

	const body = `{"id":"msg_02","type":"message","model":"claude-sonnet-4-20250514","usage":{"input_tokens":200,"output_tokens":100}}`

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(body))
	}))
	defer upstream.Close()

	spy := newSpyLogger()
	h := nativeHandlersWithSpy(upstream.URL, "anthropic", spy)

	req := httptest.NewRequest("POST", "/v1/messages", strings.NewReader(`{}`))
	w := httptest.NewRecorder()
	h.nativeProxy(w, req, "anthropic")

	spy.waitCalled(t, 2*time.Second)

	spy.mu.Lock()
	data := spy.calls[0]
	spy.mu.Unlock()

	assert.Equal(t, 200, data.PromptTokens,
		"T05: PromptTokens should be 200 (no cache; backward compat)")
	assert.Equal(t, 0, data.CacheReadInputTokens,
		"T05: CacheReadInputTokens should be 0 when cache fields absent")
	assert.Equal(t, 0, data.CacheCreationInputTokens,
		"T05: CacheCreationInputTokens should be 0 when cache fields absent")
}
