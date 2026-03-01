package handler

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/callback"
	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── helpers ────────────────────────────────────────────────────────────────

func makeHandlersWithUpstreams(entries []struct{ model, apiKey, apiBase string }) *Handlers {
	var models []config.ModelConfig
	for _, e := range entries {
		key := e.apiKey
		base := e.apiBase
		models = append(models, config.ModelConfig{
			ModelName: e.model + "-test",
			TianjiParams: config.TianjiParams{
				Model:   e.model,
				APIKey:  &key,
				APIBase: &base,
			},
		})
	}
	return &Handlers{Config: &config.ProxyConfig{ModelList: models}}
}

// ─── FR-017: resolveAllNativeUpstreams ───────────────────────────────────────

// TestResolveAllNativeUpstreams_MultipleAnthropicEntries verifies that when the config
// contains two anthropic entries, resolveAllNativeUpstreams returns both.
func TestResolveAllNativeUpstreams_MultipleAnthropicEntries(t *testing.T) {
	t.Parallel()
	h := makeHandlersWithUpstreams([]struct{ model, apiKey, apiBase string }{
		{"anthropic/claude-3-5-sonnet", "key-aaa", "https://api.anthropic.com"},
		{"anthropic/claude-3-opus", "key-bbb", "https://api.anthropic.com"},
	})

	upstreams := h.resolveAllNativeUpstreams("anthropic")

	require.Len(t, upstreams, 2, "expected 2 anthropic upstream entries")
	keys := []string{upstreams[0].APIKey, upstreams[1].APIKey}
	assert.Contains(t, keys, "key-aaa")
	assert.Contains(t, keys, "key-bbb")
}

// TestResolveAllNativeUpstreams_SingleEntry verifies that a single matching entry
// returns a slice of length 1.
func TestResolveAllNativeUpstreams_SingleEntry(t *testing.T) {
	t.Parallel()
	h := makeHandlersWithUpstreams([]struct{ model, apiKey, apiBase string }{
		{"anthropic/claude-3-5-sonnet", "key-only", "https://api.anthropic.com"},
	})

	upstreams := h.resolveAllNativeUpstreams("anthropic")

	require.Len(t, upstreams, 1, "expected 1 anthropic upstream entry")
	assert.Equal(t, "key-only", upstreams[0].APIKey)
}

// TestResolveAllNativeUpstreams_NoMatch verifies that a provider with no entries
// returns an empty (nil) slice.
func TestResolveAllNativeUpstreams_NoMatch(t *testing.T) {
	t.Parallel()
	h := makeHandlersWithUpstreams([]struct{ model, apiKey, apiBase string }{
		{"openai/gpt-4", "openai-key", "https://api.openai.com"},
	})

	upstreams := h.resolveAllNativeUpstreams("anthropic")

	assert.Empty(t, upstreams)
}

// ─── FR-018: selectUpstream round-robin ─────────────────────────────────────

// TestSelectUpstream_RoundRobin verifies that calling selectUpstream 6 times
// with 3 entries cycles through all three in order.
func TestSelectUpstream_RoundRobin(t *testing.T) {
	t.Parallel()
	// Reset global counter for deterministic test.
	// We can't reset directly since it's unexported; use a local counter pattern
	// by calling selectUpstream and just verifying all keys appear.
	upstreams := []nativeUpstream{
		{BaseURL: "https://api.anthropic.com", APIKey: "key-1"},
		{BaseURL: "https://api.anthropic.com", APIKey: "key-2"},
		{BaseURL: "https://api.anthropic.com", APIKey: "key-3"},
	}

	seen := make(map[string]int)
	for i := 0; i < 6; i++ {
		u := selectUpstream("anthropic", upstreams)
		seen[u.APIKey]++
	}

	// Each key should appear exactly twice in 6 calls (round-robin mod 3).
	assert.Equal(t, 3, len(seen), "expected all 3 keys to be selected")
	for k, count := range seen {
		assert.Equal(t, 2, count, "key %s should appear 2 times", k)
	}
}

// TestSelectUpstream_Concurrent verifies that 100 concurrent goroutines can
// call selectUpstream without panic or data race.
func TestSelectUpstream_Concurrent(t *testing.T) {
	t.Parallel()
	upstreams := []nativeUpstream{
		{BaseURL: "https://a.com", APIKey: "k1"},
		{BaseURL: "https://b.com", APIKey: "k2"},
		{BaseURL: "https://c.com", APIKey: "k3"},
	}

	var (
		wg      sync.WaitGroup
		results [100]nativeUpstream
	)
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			results[idx] = selectUpstream("anthropic", upstreams)
		}(i)
	}
	wg.Wait()

	// Verify no zero-value (empty) results — every call returned a valid upstream.
	for i, u := range results {
		assert.NotEmpty(t, u.APIKey, "goroutine %d got empty APIKey", i)
	}
}

// ─── FR-019: rate limit headers parsed on non-200 responses ─────────────────

// TestRateLimitParsed_On429Response verifies that a 429 response with rate limit
// headers still updates the RateLimitStore (FR-019: no early return before parsing).
func TestRateLimitParsed_On429Response(t *testing.T) {
	t.Parallel()

	store := callback.NewInMemoryRateLimitStore()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("anthropic-ratelimit-requests-limit", "1000")
		w.Header().Set("anthropic-ratelimit-requests-remaining", "0")
		w.Header().Set("anthropic-ratelimit-tokens-limit", "80000")
		w.Header().Set("anthropic-ratelimit-tokens-remaining", "0")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":{"type":"rate_limit_error"}}`))
	}))
	defer upstream.Close()

	apiKey := "test-oauth-token-abc"
	cfg := &config.ProxyConfig{
		ModelList: []config.ModelConfig{
			{
				ModelName: "anthropic-test",
				TianjiParams: config.TianjiParams{
					Model:   "anthropic/claude-3-5-sonnet",
					APIKey:  &apiKey,
					APIBase: &upstream.URL,
				},
			},
		},
	}
	h := &Handlers{
		Config:         cfg,
		RateLimitStore: store,
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	rr := httptest.NewRecorder()
	h.AnthropicMessages(rr, req)


	cacheKey := callback.RateLimitCacheKey(apiKey)
	state, ok := store.Get(cacheKey)
	require.True(t, ok, "RateLimitStore should have an entry for the token after 429 response")
	assert.Equal(t, 0, state.RequestsRemaining, "requests remaining should be 0 after 429")
	assert.Equal(t, 0, state.TokensRemaining, "tokens remaining should be 0 after 429")
}

// TestRateLimitParsed_On200Response verifies that a 200 response with rate limit
// headers does NOT regress (store is still updated via the 200-path flow via ParseAnthropicRateLimitHeaders).
// This test verifies the non-200 path doesn't interfere with 200 responses.
func TestRateLimitParsed_On200Response(t *testing.T) {
	t.Parallel()

	// For 200, the existing path calls ParseAnthropicRateLimitHeaders (old-style).
	// FR-019 adds store writes on non-200 only. 200 path with RateLimitStore support
	// is a separate future concern. This test verifies no panic on 200.
	var called atomic.Bool

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called.Store(true)
		w.Header().Set("anthropic-ratelimit-requests-limit", "1000")
		w.Header().Set("anthropic-ratelimit-requests-remaining", "999")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"msg_1","type":"message","role":"assistant","content":[],"model":"claude-3-5-sonnet-20241022","stop_reason":"end_turn","usage":{"input_tokens":10,"output_tokens":5}}`))
	}))
	defer upstream.Close()

	apiKey := "test-oauth-token-200"
	cfg := &config.ProxyConfig{
		ModelList: []config.ModelConfig{
			{
				ModelName: "anthropic-test",
				TianjiParams: config.TianjiParams{
					Model:   "anthropic/claude-3-5-sonnet",
					APIKey:  &apiKey,
					APIBase: &upstream.URL,
				},
			},
		},
	}
	h := &Handlers{Config: cfg}

	req := httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	rr := httptest.NewRecorder()

	// Should not panic.
	h.AnthropicMessages(rr, req)
	assert.True(t, called.Load(), "upstream should have been called")
	assert.Equal(t, http.StatusOK, rr.Code)
}
