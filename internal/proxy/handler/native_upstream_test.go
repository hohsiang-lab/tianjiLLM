package handler

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

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
		u := roundRobinSelect("test-rr-deterministic", upstreams)
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
			results[idx] = roundRobinSelect("test-anthropic-rr", upstreams)
		}(i)
	}
	wg.Wait()

	// Verify no zero-value (empty) results — every call returned a valid upstream.
	for i, u := range results {
		assert.NotEmpty(t, u.APIKey, "goroutine %d got empty APIKey", i)
	}
}

// ─── 079: selectUpstreamWithThrottle ─────────────────────────────────────────

// helper: build Handlers with RateLimitStore and upstreams for throttle tests.
func makeThrottleHandlers(upstreams []nativeUpstream, threshold float64) *Handlers {
	// Build ModelList from upstreams so resolveAllNativeUpstreams works.
	var models []config.ModelConfig
	for _, u := range upstreams {
		key := u.APIKey
		base := u.BaseURL
		models = append(models, config.ModelConfig{
			ModelName: "anthropic/test",
			TianjiParams: config.TianjiParams{
				Model:   "anthropic/test",
				APIKey:  &key,
				APIBase: &base,
			},
		})
	}
	return &Handlers{
		Config: &config.ProxyConfig{
			ModelList:               models,
			RatelimitAlertThreshold: threshold,
		},
		RateLimitStore: callback.NewInMemoryRateLimitStore(),
	}
}

// oauthKey returns a fake OAuth token key for testing (sk-ant-oat prefix).
func oauthKey(id string) string { return "sk-ant-oat-test-" + id }

// --- US1: Skip high-utilization tokens ---

func TestSelectUpstreamThrottle_Skips5hOverThreshold(t *testing.T) {
	t.Parallel()
	upstreams := []nativeUpstream{
		{BaseURL: "https://api.anthropic.com", APIKey: oauthKey("A")},
		{BaseURL: "https://api.anthropic.com", APIKey: oauthKey("B")},
		{BaseURL: "https://api.anthropic.com", APIKey: oauthKey("C")},
	}
	h := makeThrottleHandlers(upstreams, 0.8)

	// Token A: 5h utilization 85% (over threshold)
	keyA := callback.RateLimitCacheKey(oauthKey("A"))
	h.RateLimitStore.Set(keyA, callback.AnthropicOAuthRateLimitState{
		TokenKey: keyA, UnifiedStatus: "allowed",
		Unified5hUtilization: 0.85, Unified7dUtilization: 0.30,
	})

	// Call multiple times — should never select token A
	for i := 0; i < 10; i++ {
		u, err := h.selectUpstreamWithThrottle("anthropic", upstreams)
		require.NoError(t, err)
		assert.NotEqual(t, oauthKey("A"), u.APIKey, "token A (5h=85%%) should be skipped")
	}
}

func TestSelectUpstreamThrottle_Skips7dOverThreshold(t *testing.T) {
	t.Parallel()
	upstreams := []nativeUpstream{
		{BaseURL: "https://api.anthropic.com", APIKey: oauthKey("X")},
		{BaseURL: "https://api.anthropic.com", APIKey: oauthKey("Y")},
	}
	h := makeThrottleHandlers(upstreams, 0.8)

	keyX := callback.RateLimitCacheKey(oauthKey("X"))
	h.RateLimitStore.Set(keyX, callback.AnthropicOAuthRateLimitState{
		TokenKey: keyX, UnifiedStatus: "allowed",
		Unified5hUtilization: 0.50, Unified7dUtilization: 0.90,
	})

	for i := 0; i < 10; i++ {
		u, err := h.selectUpstreamWithThrottle("anthropic", upstreams)
		require.NoError(t, err)
		assert.Equal(t, oauthKey("Y"), u.APIKey, "token X (7d=90%%) should be skipped")
	}
}

func TestSelectUpstreamThrottle_RecoversBelowThreshold(t *testing.T) {
	t.Parallel()
	upstreams := []nativeUpstream{
		{BaseURL: "https://api.anthropic.com", APIKey: oauthKey("R1")},
		{BaseURL: "https://api.anthropic.com", APIKey: oauthKey("R2")},
	}
	h := makeThrottleHandlers(upstreams, 0.8)

	keyR1 := callback.RateLimitCacheKey(oauthKey("R1"))
	h.RateLimitStore.Set(keyR1, callback.AnthropicOAuthRateLimitState{
		TokenKey: keyR1, UnifiedStatus: "allowed",
		Unified5hUtilization: 0.60, Unified7dUtilization: 0.40,
	})

	// Token R1 at 60% — should be available
	selected := map[string]bool{}
	for i := 0; i < 20; i++ {
		u, err := h.selectUpstreamWithThrottle("anthropic", upstreams)
		require.NoError(t, err)
		selected[u.APIKey] = true
	}
	assert.True(t, selected[oauthKey("R1")], "token R1 (60%%) should be available")
}

func TestSelectUpstreamThrottle_SkipsRateLimitedStatus(t *testing.T) {
	t.Parallel()
	upstreams := []nativeUpstream{
		{BaseURL: "https://api.anthropic.com", APIKey: oauthKey("RL1")},
		{BaseURL: "https://api.anthropic.com", APIKey: oauthKey("RL2")},
	}
	h := makeThrottleHandlers(upstreams, 0.8)

	keyRL1 := callback.RateLimitCacheKey(oauthKey("RL1"))
	h.RateLimitStore.Set(keyRL1, callback.AnthropicOAuthRateLimitState{
		TokenKey: keyRL1, UnifiedStatus: "rate_limited",
		Unified5hUtilization: 0.50, Unified7dUtilization: 0.30,
	})

	for i := 0; i < 10; i++ {
		u, err := h.selectUpstreamWithThrottle("anthropic", upstreams)
		require.NoError(t, err)
		assert.Equal(t, oauthKey("RL2"), u.APIKey, "rate_limited token should be skipped regardless of utilization")
	}
}

func TestSelectUpstreamThrottle_UnknownStateIsAvailable(t *testing.T) {
	t.Parallel()
	upstreams := []nativeUpstream{
		{BaseURL: "https://api.anthropic.com", APIKey: oauthKey("UNK")},
	}
	h := makeThrottleHandlers(upstreams, 0.8)
	// No state set in store

	u, err := h.selectUpstreamWithThrottle("anthropic", upstreams)
	require.NoError(t, err)
	assert.Equal(t, oauthKey("UNK"), u.APIKey, "unknown state token should be available")
}

func TestSelectUpstreamThrottle_SentinelNeg1IsAvailable(t *testing.T) {
	t.Parallel()
	upstreams := []nativeUpstream{
		{BaseURL: "https://api.anthropic.com", APIKey: oauthKey("S1")},
	}
	h := makeThrottleHandlers(upstreams, 0.8)

	keyS1 := callback.RateLimitCacheKey(oauthKey("S1"))
	h.RateLimitStore.Set(keyS1, callback.AnthropicOAuthRateLimitState{
		TokenKey: keyS1, UnifiedStatus: "allowed",
		Unified5hUtilization: -1, Unified7dUtilization: -1,
	})

	u, err := h.selectUpstreamWithThrottle("anthropic", upstreams)
	require.NoError(t, err)
	assert.Equal(t, oauthKey("S1"), u.APIKey, "sentinel -1 utilization should be treated as available")
}

func TestSelectUpstreamThrottle_NonOAuthNotThrottled(t *testing.T) {
	t.Parallel()
	upstreams := []nativeUpstream{
		{BaseURL: "https://api.anthropic.com", APIKey: "sk-regular-api-key"},
	}
	h := makeThrottleHandlers(upstreams, 0.8)

	// Even if we somehow have state for this key, non-OAuth should not be throttled.
	u, err := h.selectUpstreamWithThrottle("anthropic", upstreams)
	require.NoError(t, err)
	assert.Equal(t, "sk-regular-api-key", u.APIKey, "non-OAuth keys should never be throttled")
}

func TestSelectUpstreamThrottle_DeduplicatesByAPIKey(t *testing.T) {
	t.Parallel()
	sameKey := oauthKey("DUP")
	upstreams := []nativeUpstream{
		{BaseURL: "https://api.anthropic.com", APIKey: sameKey},
		{BaseURL: "https://api.anthropic.com", APIKey: sameKey},
		{BaseURL: "https://api.anthropic.com", APIKey: sameKey},
	}
	h := makeThrottleHandlers(upstreams, 0.8)

	u, err := h.selectUpstreamWithThrottle("anthropic", upstreams)
	require.NoError(t, err)
	assert.Equal(t, sameKey, u.APIKey)
}

// --- US2: All tokens throttled → error ---

func TestSelectUpstreamThrottle_AllThrottled_ReturnsError(t *testing.T) {
	t.Parallel()
	upstreams := []nativeUpstream{
		{BaseURL: "https://api.anthropic.com", APIKey: oauthKey("AT1")},
		{BaseURL: "https://api.anthropic.com", APIKey: oauthKey("AT2")},
	}
	h := makeThrottleHandlers(upstreams, 0.8)

	for _, key := range []string{oauthKey("AT1"), oauthKey("AT2")} {
		ck := callback.RateLimitCacheKey(key)
		h.RateLimitStore.Set(ck, callback.AnthropicOAuthRateLimitState{
			TokenKey: ck, UnifiedStatus: "allowed",
			Unified5hUtilization: 0.90, Unified7dUtilization: 0.50,
		})
	}

	_, err := h.selectUpstreamWithThrottle("anthropic", upstreams)
	require.Error(t, err)
	var ate *allTokensThrottledError
	assert.ErrorAs(t, err, &ate, "should return allTokensThrottledError")
}

func TestSelectUpstreamThrottle_AllThrottled_NearestReset(t *testing.T) {
	t.Parallel()
	upstreams := []nativeUpstream{
		{BaseURL: "https://api.anthropic.com", APIKey: oauthKey("NR1")},
		{BaseURL: "https://api.anthropic.com", APIKey: oauthKey("NR2")},
	}
	h := makeThrottleHandlers(upstreams, 0.8)

	now := time.Now()
	near := now.Add(30 * time.Minute) // NR1 resets in 30 min
	far := now.Add(120 * time.Minute) // NR2 resets in 2 hours

	ck1 := callback.RateLimitCacheKey(oauthKey("NR1"))
	h.RateLimitStore.Set(ck1, callback.AnthropicOAuthRateLimitState{
		TokenKey: ck1, UnifiedStatus: "allowed",
		Unified5hUtilization: 0.90, Unified5hReset: fmt.Sprintf("%d", near.Unix()),
	})
	ck2 := callback.RateLimitCacheKey(oauthKey("NR2"))
	h.RateLimitStore.Set(ck2, callback.AnthropicOAuthRateLimitState{
		TokenKey: ck2, UnifiedStatus: "allowed",
		Unified5hUtilization: 0.95, Unified5hReset: fmt.Sprintf("%d", far.Unix()),
	})

	_, err := h.selectUpstreamWithThrottle("anthropic", upstreams)
	require.Error(t, err)
	var ate *allTokensThrottledError
	require.ErrorAs(t, err, &ate)
	// Nearest reset should be ~30 min from now (NR1), not 2 hours (NR2)
	assert.WithinDuration(t, near, ate.resetAt, 2*time.Second, "should pick nearest reset time")
}

func TestSelectUpstreamThrottle_SingleTokenThrottled_Returns429(t *testing.T) {
	t.Parallel()
	upstreams := []nativeUpstream{
		{BaseURL: "https://api.anthropic.com", APIKey: oauthKey("SOLO")},
	}
	h := makeThrottleHandlers(upstreams, 0.8)

	ck := callback.RateLimitCacheKey(oauthKey("SOLO"))
	h.RateLimitStore.Set(ck, callback.AnthropicOAuthRateLimitState{
		TokenKey: ck, UnifiedStatus: "allowed",
		Unified5hUtilization: 0.85, Unified7dUtilization: 0.50,
	})

	_, err := h.selectUpstreamWithThrottle("anthropic", upstreams)
	require.Error(t, err)
	var ate *allTokensThrottledError
	assert.ErrorAs(t, err, &ate, "single throttled token should return error")
}

func TestSelectUpstreamThrottle_ConfigurableThreshold(t *testing.T) {
	t.Parallel()
	upstreams := []nativeUpstream{
		{BaseURL: "https://api.anthropic.com", APIKey: oauthKey("CFG")},
	}
	// Threshold = 0.5
	h := makeThrottleHandlers(upstreams, 0.5)

	ck := callback.RateLimitCacheKey(oauthKey("CFG"))

	// At 0.6 → over 0.5 threshold → throttled
	h.RateLimitStore.Set(ck, callback.AnthropicOAuthRateLimitState{
		TokenKey: ck, UnifiedStatus: "allowed",
		Unified5hUtilization: 0.6, Unified7dUtilization: -1,
	})
	_, err := h.selectUpstreamWithThrottle("anthropic", upstreams)
	require.Error(t, err, "0.6 utilization should be throttled at 0.5 threshold")

	// At 0.4 → under 0.5 threshold → available
	h.RateLimitStore.Set(ck, callback.AnthropicOAuthRateLimitState{
		TokenKey: ck, UnifiedStatus: "allowed",
		Unified5hUtilization: 0.4, Unified7dUtilization: -1,
	})
	u, err := h.selectUpstreamWithThrottle("anthropic", upstreams)
	require.NoError(t, err, "0.4 utilization should be available at 0.5 threshold")
	assert.Equal(t, oauthKey("CFG"), u.APIKey)
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
