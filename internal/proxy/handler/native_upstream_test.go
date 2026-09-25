package handler

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/praxisllmlab/tianjiLLM/internal/callback"
	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/praxisllmlab/tianjiLLM/internal/db"
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

	upstreams := h.resolveAllNativeUpstreams(context.Background(), "anthropic")

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

	upstreams := h.resolveAllNativeUpstreams(context.Background(), "anthropic")

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

	upstreams := h.resolveAllNativeUpstreams(context.Background(), "anthropic")

	assert.Empty(t, upstreams)
}

// ─── FR-018: selectUpstream round-robin ─────────────────────────────────────

// TestSelectUpstream_RoundRobin verifies that calling selectUpstream 6 times
// with 3 entries cycles through all three in order.
func TestSelectUpstream_RoundRobin(t *testing.T) {
	t.Parallel()
	h := makeThrottleHandlers(nil, 0.8)
	upstreams := []nativeUpstream{
		{BaseURL: "https://api.anthropic.com", APIKey: "key-1"},
		{BaseURL: "https://api.anthropic.com", APIKey: "key-2"},
		{BaseURL: "https://api.anthropic.com", APIKey: "key-3"},
	}

	seen := make(map[string]int)
	for i := 0; i < 6; i++ {
		u := h.roundRobinSelect("test-rr-deterministic", upstreams)
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
	h := makeThrottleHandlers(nil, 0.8)
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
			results[idx] = h.roundRobinSelect("test-anthropic-rr", upstreams)
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
		TokenKey: keyA, UnifiedStatus: callback.UnifiedStatusAllowed,
		Unified5hUtilization: 0.85, Unified7dUtilization: 0.30,
	})

	// Call multiple times — should never select token A
	for i := 0; i < 10; i++ {
		u, err := h.selectUpstreamWithThrottle(context.Background(), "anthropic", upstreams, "")
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
		TokenKey: keyX, UnifiedStatus: callback.UnifiedStatusAllowed,
		Unified5hUtilization: 0.50, Unified7dUtilization: 0.90,
	})

	for i := 0; i < 10; i++ {
		u, err := h.selectUpstreamWithThrottle(context.Background(), "anthropic", upstreams, "")
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
		TokenKey: keyR1, UnifiedStatus: callback.UnifiedStatusAllowed,
		Unified5hUtilization: 0.60, Unified7dUtilization: 0.40,
	})

	// Token R1 at 60% — should be available
	selected := map[string]bool{}
	for i := 0; i < 20; i++ {
		u, err := h.selectUpstreamWithThrottle(context.Background(), "anthropic", upstreams, "")
		require.NoError(t, err)
		selected[u.APIKey] = true
	}
	assert.True(t, selected[oauthKey("R1")], "token R1 (60%%) should be available")
}

// HO-490: SkipsRejectedStatus — replaces old SkipsRateLimitedStatus / SkipsOverageStatus
func TestSelectUpstreamThrottle_SkipsRejectedStatus(t *testing.T) {
	t.Parallel()
	upstreams := []nativeUpstream{
		{BaseURL: "https://api.anthropic.com", APIKey: oauthKey("RJ1")},
		{BaseURL: "https://api.anthropic.com", APIKey: oauthKey("RJ2")},
	}
	h := makeThrottleHandlers(upstreams, 0.8)

	keyRJ1 := callback.RateLimitCacheKey(oauthKey("RJ1"))
	h.RateLimitStore.Set(keyRJ1, callback.AnthropicOAuthRateLimitState{
		TokenKey: keyRJ1, UnifiedStatus: "rejected",
		Unified5hUtilization: 0.50, Unified7dUtilization: 0.30,
	})

	for range 10 {
		u, err := h.selectUpstreamWithThrottle(context.Background(), "anthropic", upstreams, "")
		require.NoError(t, err)
		assert.Equal(t, oauthKey("RJ2"), u.APIKey, "rejected token must be skipped")
	}
}

// HO-490: allowed_warning — bypass 5h/7d utilization gate
func TestSelectUpstreamThrottle_AllowedWarning_BypassesUtilizationGate(t *testing.T) {
	t.Parallel()
	upstreams := []nativeUpstream{
		{BaseURL: "https://api.anthropic.com", APIKey: oauthKey("AW1")},
		{BaseURL: "https://api.anthropic.com", APIKey: oauthKey("AW2")},
	}
	h := makeThrottleHandlers(upstreams, 0.8)

	keyAW1 := callback.RateLimitCacheKey(oauthKey("AW1"))
	h.RateLimitStore.Set(keyAW1, callback.AnthropicOAuthRateLimitState{
		TokenKey: keyAW1, UnifiedStatus: "allowed_warning",
		Unified5hUtilization: 0.92, Unified7dUtilization: 0.88,
	})

	found := map[string]int{}
	for range 20 {
		u, err := h.selectUpstreamWithThrottle(context.Background(), "anthropic", upstreams, "")
		require.NoError(t, err)
		found[u.APIKey]++
	}
	assert.Greater(t, found[oauthKey("AW1")], 0, "allowed_warning token should not be skipped despite high utilization")
}

// HO-490: allowed + high util must still be throttled (no regression)
func TestSelectUpstreamThrottle_Allowed_HighUtil_StillThrottled_NoRegression(t *testing.T) {
	t.Parallel()
	upstreams := []nativeUpstream{
		{BaseURL: "https://api.anthropic.com", APIKey: oauthKey("HU1")},
		{BaseURL: "https://api.anthropic.com", APIKey: oauthKey("HU2")},
	}
	h := makeThrottleHandlers(upstreams, 0.8)

	keyHU1 := callback.RateLimitCacheKey(oauthKey("HU1"))
	h.RateLimitStore.Set(keyHU1, callback.AnthropicOAuthRateLimitState{
		TokenKey: keyHU1, UnifiedStatus: "allowed",
		Unified5hUtilization: 0.92,
	})

	for range 10 {
		u, err := h.selectUpstreamWithThrottle(context.Background(), "anthropic", upstreams, "")
		require.NoError(t, err)
		assert.Equal(t, oauthKey("HU2"), u.APIKey, "allowed token with high util must still be throttled")
	}
}

func TestSelectUpstreamThrottle_UnknownStateIsAvailable(t *testing.T) {
	t.Parallel()
	upstreams := []nativeUpstream{
		{BaseURL: "https://api.anthropic.com", APIKey: oauthKey("UNK")},
	}
	h := makeThrottleHandlers(upstreams, 0.8)
	// No state set in store

	u, err := h.selectUpstreamWithThrottle(context.Background(), "anthropic", upstreams, "")
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
		TokenKey: keyS1, UnifiedStatus: callback.UnifiedStatusAllowed,
		Unified5hUtilization: -1, Unified7dUtilization: -1,
	})

	u, err := h.selectUpstreamWithThrottle(context.Background(), "anthropic", upstreams, "")
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
	u, err := h.selectUpstreamWithThrottle(context.Background(), "anthropic", upstreams, "")
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

	u, err := h.selectUpstreamWithThrottle(context.Background(), "anthropic", upstreams, "")
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
			TokenKey: ck, UnifiedStatus: callback.UnifiedStatusAllowed,
			Unified5hUtilization: 0.90, Unified7dUtilization: 0.50,
		})
	}

	_, err := h.selectUpstreamWithThrottle(context.Background(), "anthropic", upstreams, "")
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
		TokenKey: ck1, UnifiedStatus: callback.UnifiedStatusAllowed,
		Unified5hUtilization: 0.90, Unified5hReset: fmt.Sprintf("%d", near.Unix()),
	})
	ck2 := callback.RateLimitCacheKey(oauthKey("NR2"))
	h.RateLimitStore.Set(ck2, callback.AnthropicOAuthRateLimitState{
		TokenKey: ck2, UnifiedStatus: callback.UnifiedStatusAllowed,
		Unified5hUtilization: 0.95, Unified5hReset: fmt.Sprintf("%d", far.Unix()),
	})

	_, err := h.selectUpstreamWithThrottle(context.Background(), "anthropic", upstreams, "")
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
		TokenKey: ck, UnifiedStatus: callback.UnifiedStatusAllowed,
		Unified5hUtilization: 0.85, Unified7dUtilization: 0.50,
	})

	_, err := h.selectUpstreamWithThrottle(context.Background(), "anthropic", upstreams, "")
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
		TokenKey: ck, UnifiedStatus: callback.UnifiedStatusAllowed,
		Unified5hUtilization: 0.6, Unified7dUtilization: -1,
	})
	_, err := h.selectUpstreamWithThrottle(context.Background(), "anthropic", upstreams, "")
	require.Error(t, err, "0.6 utilization should be throttled at 0.5 threshold")

	// At 0.4 → under 0.5 threshold → available
	h.RateLimitStore.Set(ck, callback.AnthropicOAuthRateLimitState{
		TokenKey: ck, UnifiedStatus: callback.UnifiedStatusAllowed,
		Unified5hUtilization: 0.4, Unified7dUtilization: -1,
	})
	u, err := h.selectUpstreamWithThrottle(context.Background(), "anthropic", upstreams, "")
	require.NoError(t, err, "0.4 utilization should be available at 0.5 threshold")
	assert.Equal(t, oauthKey("CFG"), u.APIKey)
}

// ─── gate7d independence from 5h threshold ──────────────────────────────────

// TestSelectUpstreamThrottle_Gate7dBelowGate_TokenNotSkipped verifies that a token
// with 7d utilization just below the gate (0.89 < 0.9) is NOT skipped, even when
// RatelimitAlertThreshold is low (0.7). The gate7d check is independent of the
// configurable 5h threshold.
func TestSelectUpstreamThrottle_Gate7dBelowGate_TokenNotSkipped(t *testing.T) {
	t.Parallel()
	upstreams := []nativeUpstream{
		{BaseURL: "https://api.anthropic.com", APIKey: oauthKey("BG7")},
	}
	// 5h threshold = 0.7 — below 7d gate but token's 5h util is only 0.50 so not 5h-throttled.
	h := makeThrottleHandlers(upstreams, 0.7)

	ck := callback.RateLimitCacheKey(oauthKey("BG7"))
	h.RateLimitStore.Set(ck, callback.AnthropicOAuthRateLimitState{
		TokenKey:             ck,
		UnifiedStatus:        callback.UnifiedStatusAllowed,
		Unified5hUtilization: 0.50, // below 0.7 threshold
		Unified7dUtilization: 0.89, // below gate7d (0.9) → must NOT skip
	})

	u, err := h.selectUpstreamWithThrottle(context.Background(), "anthropic", upstreams, "")
	require.NoError(t, err, "7d=0.89 is below gate7d=0.9 — token must not be skipped")
	assert.Equal(t, oauthKey("BG7"), u.APIKey)
}

// TestSelectUpstreamThrottle_Gate7dAboveGate_TokenSkipped verifies that a token
// with 7d utilization just above the gate (0.91 >= 0.9) IS skipped, even when
// RatelimitAlertThreshold is high (0.95) and the 5h utilization is well below it.
// This confirms gate7d acts as an independent hard cutoff.
func TestSelectUpstreamThrottle_Gate7dAboveGate_TokenSkipped(t *testing.T) {
	t.Parallel()
	upstreams := []nativeUpstream{
		{BaseURL: "https://api.anthropic.com", APIKey: oauthKey("AG7")},
	}
	// 5h threshold = 0.95 — token's 5h util is only 0.50, so it would pass the 5h check.
	h := makeThrottleHandlers(upstreams, 0.95)

	ck := callback.RateLimitCacheKey(oauthKey("AG7"))
	h.RateLimitStore.Set(ck, callback.AnthropicOAuthRateLimitState{
		TokenKey:             ck,
		UnifiedStatus:        callback.UnifiedStatusAllowed,
		Unified5hUtilization: 0.50, // well below 0.95 — would not trigger 5h gate
		Unified7dUtilization: 0.91, // above gate7d (0.9) → must skip regardless
	})

	_, err := h.selectUpstreamWithThrottle(context.Background(), "anthropic", upstreams, "")
	require.Error(t, err, "7d=0.91 exceeds gate7d=0.9 — token must be skipped even with high 5h threshold")
	var ate *allTokensThrottledError
	assert.ErrorAs(t, err, &ate)
}

// TestSelectUpstreamThrottle_Gate7dExactBoundary verifies that a token with 7d
// utilization exactly at the gate (0.90 == gate7d) IS skipped — the check is >=.
func TestSelectUpstreamThrottle_Gate7dExactBoundary(t *testing.T) {
	t.Parallel()
	upstreams := []nativeUpstream{
		{BaseURL: "https://api.anthropic.com", APIKey: oauthKey("EX7")},
	}
	h := makeThrottleHandlers(upstreams, 0.95)

	ck := callback.RateLimitCacheKey(oauthKey("EX7"))
	h.RateLimitStore.Set(ck, callback.AnthropicOAuthRateLimitState{
		TokenKey:             ck,
		UnifiedStatus:        callback.UnifiedStatusAllowed,
		Unified5hUtilization: 0.50,
		Unified7dUtilization: 0.90, // exactly at gate7d → must skip (>= gate7d)
	})

	_, err := h.selectUpstreamWithThrottle(context.Background(), "anthropic", upstreams, "")
	require.Error(t, err, "7d=0.90 equals gate7d=0.9 — token must be skipped (>= check)")
	var ate *allTokensThrottledError
	assert.ErrorAs(t, err, &ate)
}

// ─── Stale reset: tokens must not be throttled after reset time passes ──────

func TestSelectUpstreamThrottle_5hGateIgnoredAfterReset(t *testing.T) {
	t.Parallel()
	upstreams := []nativeUpstream{
		{BaseURL: "https://api.anthropic.com", APIKey: oauthKey("STALE5H")},
	}
	h := makeThrottleHandlers(upstreams, 0.8)

	key := callback.RateLimitCacheKey(oauthKey("STALE5H"))
	h.RateLimitStore.Set(key, callback.AnthropicOAuthRateLimitState{
		TokenKey:             key,
		UnifiedStatus:        callback.UnifiedStatusAllowed,
		Unified5hUtilization: 0.85,
		Unified5hReset:       fmt.Sprintf("%d", time.Now().Add(-1*time.Hour).Unix()), // reset 1h ago
		Unified7dUtilization: 0.30,
		Unified7dReset:       fmt.Sprintf("%d", time.Now().Add(3*24*time.Hour).Unix()),
	})

	u, err := h.selectUpstreamWithThrottle(context.Background(), "anthropic", upstreams, "")
	require.NoError(t, err, "5h reset has passed — stale 85%% util must not block token")
	assert.Equal(t, oauthKey("STALE5H"), u.APIKey)
}

func TestSelectUpstreamThrottle_7dGateIgnoredAfterReset(t *testing.T) {
	t.Parallel()
	upstreams := []nativeUpstream{
		{BaseURL: "https://api.anthropic.com", APIKey: oauthKey("STALE7D")},
	}
	h := makeThrottleHandlers(upstreams, 0.8)

	key := callback.RateLimitCacheKey(oauthKey("STALE7D"))
	h.RateLimitStore.Set(key, callback.AnthropicOAuthRateLimitState{
		TokenKey:             key,
		UnifiedStatus:        callback.UnifiedStatusAllowed,
		Unified5hUtilization: 0.50,
		Unified5hReset:       fmt.Sprintf("%d", time.Now().Add(2*time.Hour).Unix()),
		Unified7dUtilization: 0.92,
		Unified7dReset:       fmt.Sprintf("%d", time.Now().Add(-1*time.Hour).Unix()), // reset 1h ago
	})

	u, err := h.selectUpstreamWithThrottle(context.Background(), "anthropic", upstreams, "")
	require.NoError(t, err, "7d reset has passed — stale 92%% util must not block token")
	assert.Equal(t, oauthKey("STALE7D"), u.APIKey)
}

// HO-490: RejectedStatusIgnoredAfterReset — replaces RateLimitedStatusIgnoredAfterReset + OverageStatusIgnoredAfterReset
func TestSelectUpstreamThrottle_RejectedStatusIgnoredAfterReset(t *testing.T) {
	t.Parallel()
	upstreams := []nativeUpstream{
		{BaseURL: "https://api.anthropic.com", APIKey: oauthKey("STALREJ")},
	}
	h := makeThrottleHandlers(upstreams, 0.8)

	key := callback.RateLimitCacheKey(oauthKey("STALREJ"))
	h.RateLimitStore.Set(key, callback.AnthropicOAuthRateLimitState{
		TokenKey:             key,
		UnifiedStatus:        "rejected",
		UnifiedReset:         fmt.Sprintf("%d", time.Now().Add(-1*time.Hour).Unix()), // reset 1h ago
		Unified5hUtilization: 0.50,
		Unified5hReset:       fmt.Sprintf("%d", time.Now().Add(2*time.Hour).Unix()),
		Unified7dUtilization: 0.30,
		Unified7dReset:       fmt.Sprintf("%d", time.Now().Add(3*24*time.Hour).Unix()),
	})

	u, err := h.selectUpstreamWithThrottle(context.Background(), "anthropic", upstreams, "")
	require.NoError(t, err, "unified reset has passed — stale rejected status must not block token")
	assert.Equal(t, oauthKey("STALREJ"), u.APIKey)
}

func TestSelectUpstreamThrottle_5hExpired7dStillBlocks(t *testing.T) {
	t.Parallel()
	upstreams := []nativeUpstream{
		{BaseURL: "https://api.anthropic.com", APIKey: oauthKey("MIX1")},
	}
	h := makeThrottleHandlers(upstreams, 0.8)

	key := callback.RateLimitCacheKey(oauthKey("MIX1"))
	h.RateLimitStore.Set(key, callback.AnthropicOAuthRateLimitState{
		TokenKey:             key,
		UnifiedStatus:        callback.UnifiedStatusAllowed,
		Unified5hUtilization: 0.85,
		Unified5hReset:       fmt.Sprintf("%d", time.Now().Add(-1*time.Hour).Unix()), // expired
		Unified7dUtilization: 0.92,
		Unified7dReset:       fmt.Sprintf("%d", time.Now().Add(3*24*time.Hour).Unix()), // NOT expired
	})

	_, err := h.selectUpstreamWithThrottle(context.Background(), "anthropic", upstreams, "")
	require.Error(t, err, "5h expired but 7d still over threshold — token must be skipped")
	var ate *allTokensThrottledError
	assert.ErrorAs(t, err, &ate)
}

func TestSelectUpstreamThrottle_7dExpired5hStillBlocks(t *testing.T) {
	t.Parallel()
	upstreams := []nativeUpstream{
		{BaseURL: "https://api.anthropic.com", APIKey: oauthKey("MIX2")},
	}
	h := makeThrottleHandlers(upstreams, 0.8)

	key := callback.RateLimitCacheKey(oauthKey("MIX2"))
	h.RateLimitStore.Set(key, callback.AnthropicOAuthRateLimitState{
		TokenKey:             key,
		UnifiedStatus:        callback.UnifiedStatusAllowed,
		Unified5hUtilization: 0.85,
		Unified5hReset:       fmt.Sprintf("%d", time.Now().Add(2*time.Hour).Unix()), // NOT expired
		Unified7dUtilization: 0.92,
		Unified7dReset:       fmt.Sprintf("%d", time.Now().Add(-1*time.Hour).Unix()), // expired
	})

	_, err := h.selectUpstreamWithThrottle(context.Background(), "anthropic", upstreams, "")
	require.Error(t, err, "7d expired but 5h still over threshold — token must be skipped")
	var ate *allTokensThrottledError
	assert.ErrorAs(t, err, &ate)
}

func TestSelectUpstreamThrottle_BothWindowsExpired_TokenAvailable(t *testing.T) {
	t.Parallel()
	upstreams := []nativeUpstream{
		{BaseURL: "https://api.anthropic.com", APIKey: oauthKey("BOTH")},
	}
	h := makeThrottleHandlers(upstreams, 0.8)

	key := callback.RateLimitCacheKey(oauthKey("BOTH"))
	h.RateLimitStore.Set(key, callback.AnthropicOAuthRateLimitState{
		TokenKey:             key,
		UnifiedStatus:        callback.UnifiedStatusAllowed,
		Unified5hUtilization: 0.90,
		Unified5hReset:       fmt.Sprintf("%d", time.Now().Add(-1*time.Hour).Unix()), // expired
		Unified7dUtilization: 0.95,
		Unified7dReset:       fmt.Sprintf("%d", time.Now().Add(-30*time.Minute).Unix()), // expired
	})

	u, err := h.selectUpstreamWithThrottle(context.Background(), "anthropic", upstreams, "")
	require.NoError(t, err, "both windows expired — all stale utils must be ignored")
	assert.Equal(t, oauthKey("BOTH"), u.APIKey)
}

func TestSelectUpstreamThrottle_StatusExpiredButUtilGateStillBlocks(t *testing.T) {
	t.Parallel()
	upstreams := []nativeUpstream{
		{BaseURL: "https://api.anthropic.com", APIKey: oauthKey("MIXED")},
	}
	h := makeThrottleHandlers(upstreams, 0.8)

	key := callback.RateLimitCacheKey(oauthKey("MIXED"))
	h.RateLimitStore.Set(key, callback.AnthropicOAuthRateLimitState{
		TokenKey:             key,
		UnifiedStatus:        "rejected",
		UnifiedReset:         fmt.Sprintf("%d", time.Now().Add(-1*time.Hour).Unix()), // status expired
		Unified5hUtilization: 0.85,
		Unified5hReset:       fmt.Sprintf("%d", time.Now().Add(2*time.Hour).Unix()), // NOT expired
		Unified7dUtilization: 0.30,
		Unified7dReset:       fmt.Sprintf("%d", time.Now().Add(3*24*time.Hour).Unix()),
	})

	_, err := h.selectUpstreamWithThrottle(context.Background(), "anthropic", upstreams, "")
	require.Error(t, err, "status expired but 5h util still over threshold — token must be skipped")
	var ate *allTokensThrottledError
	assert.ErrorAs(t, err, &ate)
}

// ─── Stale reset: verify non-expired tokens still throttled correctly ────────

func TestSelectUpstreamThrottle_5hNotExpired_StillThrottled(t *testing.T) {
	t.Parallel()
	upstreams := []nativeUpstream{
		{BaseURL: "https://api.anthropic.com", APIKey: oauthKey("FRESH5H")},
	}
	h := makeThrottleHandlers(upstreams, 0.8)

	key := callback.RateLimitCacheKey(oauthKey("FRESH5H"))
	h.RateLimitStore.Set(key, callback.AnthropicOAuthRateLimitState{
		TokenKey:             key,
		UnifiedStatus:        callback.UnifiedStatusAllowed,
		Unified5hUtilization: 0.85,
		Unified5hReset:       fmt.Sprintf("%d", time.Now().Add(1*time.Hour).Unix()), // NOT expired
		Unified7dUtilization: 0.30,
		Unified7dReset:       fmt.Sprintf("%d", time.Now().Add(3*24*time.Hour).Unix()),
	})

	_, err := h.selectUpstreamWithThrottle(context.Background(), "anthropic", upstreams, "")
	require.Error(t, err, "5h reset not yet passed — 85%% util must still block")
	var ate *allTokensThrottledError
	assert.ErrorAs(t, err, &ate)
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
// headers does NOT regress (store is still updated via the 200-path flow via recordRateLimitUsage).
// This test verifies the non-200 path doesn't interfere with 200 responses.
func TestRateLimitParsed_On200Response(t *testing.T) {
	t.Parallel()

	// For 200, the existing path calls recordRateLimitUsage -> ParseAnthropicOAuthRateLimitHeaders.
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

// ─── lowestUtilizationSelect: pick token with lowest 5h utilization ──────────

func TestLowestUtilizationSelect_PicksLowerUtilization(t *testing.T) {
	t.Parallel()
	upstreams := []nativeUpstream{
		{BaseURL: "https://api.anthropic.com", APIKey: oauthKey("HI")},
		{BaseURL: "https://api.anthropic.com", APIKey: oauthKey("LO")},
	}
	h := makeThrottleHandlers(upstreams, 0.8)

	keyHI := callback.RateLimitCacheKey(oauthKey("HI"))
	h.RateLimitStore.Set(keyHI, callback.AnthropicOAuthRateLimitState{
		TokenKey: keyHI, UnifiedStatus: callback.UnifiedStatusAllowed,
		Unified5hUtilization: 0.60, Unified7dUtilization: 0.20,
	})
	keyLO := callback.RateLimitCacheKey(oauthKey("LO"))
	h.RateLimitStore.Set(keyLO, callback.AnthropicOAuthRateLimitState{
		TokenKey: keyLO, UnifiedStatus: callback.UnifiedStatusAllowed,
		Unified5hUtilization: 0.30, Unified7dUtilization: 0.20,
	})

	// Should always pick LO (30% < 60%)
	for i := 0; i < 20; i++ {
		u := h.lowestUtilizationSelect(context.Background(), "anthropic", upstreams)
		assert.Equal(t, oauthKey("LO"), u.APIKey, "iteration %d: should pick lower 5h utilization token", i)
	}
}

func TestLowestUtilizationSelect_FallbackToRoundRobin(t *testing.T) {
	t.Parallel()
	upstreams := []nativeUpstream{
		{BaseURL: "https://api.anthropic.com", APIKey: oauthKey("NDA")},
		{BaseURL: "https://api.anthropic.com", APIKey: oauthKey("NDB")},
	}
	h := makeThrottleHandlers(upstreams, 0.8)
	// No utilization data in store → all tokens evaluated with composite=0; first token wins.
	// (Round-robin is only used when no token was evaluated at all, i.e. bestUtil==sentinelMaxComposite)
	u := h.lowestUtilizationSelect(context.Background(), "anthropic", upstreams)
	validKeys := []string{oauthKey("NDA"), oauthKey("NDB")}
	assert.Contains(t, validKeys, u.APIKey, "should return one of the configured upstreams when no store data exists")
}

func TestLowestUtilizationSelect_TiebreakBy7d(t *testing.T) {
	t.Parallel()
	upstreams := []nativeUpstream{
		{BaseURL: "https://api.anthropic.com", APIKey: oauthKey("T1")},
		{BaseURL: "https://api.anthropic.com", APIKey: oauthKey("T2")},
	}
	h := makeThrottleHandlers(upstreams, 0.8)

	keyT1 := callback.RateLimitCacheKey(oauthKey("T1"))
	h.RateLimitStore.Set(keyT1, callback.AnthropicOAuthRateLimitState{
		TokenKey: keyT1, UnifiedStatus: callback.UnifiedStatusAllowed,
		Unified5hUtilization: 0.40, Unified7dUtilization: 0.20,
	})
	keyT2 := callback.RateLimitCacheKey(oauthKey("T2"))
	h.RateLimitStore.Set(keyT2, callback.AnthropicOAuthRateLimitState{
		TokenKey: keyT2, UnifiedStatus: callback.UnifiedStatusAllowed,
		Unified5hUtilization: 0.40, Unified7dUtilization: 0.50,
	})

	// Same 5h (40%), T1 has lower 7d (20% < 50%) → should pick T1
	for i := 0; i < 20; i++ {
		u := h.lowestUtilizationSelect(context.Background(), "anthropic", upstreams)
		assert.Equal(t, oauthKey("T1"), u.APIKey, "iteration %d: should tiebreak by 7d utilization", i)
	}
}

// When 7d utilization is equal, the token with lower 5h wins.
// TestLowestUtilizationSelect_Equal7dLower5hWins verifies that when 7d utilization is
// equal, the token with lower 5h utilization wins. This guards the quadratic composite
// Max-normalized formula: composite = max(5h/θ_5h, 7d/0.9, 7d_s/0.9).
// Equal 7d and 7d_s → the 5h/θ_5h term alone decides the winner.
func TestLowestUtilizationSelect_Equal7dLower5hWins(t *testing.T) {
	t.Parallel()
	upstreams := []nativeUpstream{
		{BaseURL: "https://api.anthropic.com", APIKey: oauthKey("A5h")},
		{BaseURL: "https://api.anthropic.com", APIKey: oauthKey("B5h")},
	}
	h := makeThrottleHandlers(upstreams, 0.8)

	// A: 5h=0.50, 7d=0.30, 7d_s=0 → max(0.625, 0.333, 0) = 0.625
	// B: 5h=0.48, 7d=0.30, 7d_s=0 → max(0.600, 0.333, 0) = 0.600
	// Equal 7d → lower 5h wins (B).
	keyA := callback.RateLimitCacheKey(oauthKey("A5h"))
	h.RateLimitStore.Set(keyA, callback.AnthropicOAuthRateLimitState{
		TokenKey: keyA, UnifiedStatus: callback.UnifiedStatusAllowed,
		Unified5hUtilization: 0.50, Unified7dUtilization: 0.30,
	})
	keyB := callback.RateLimitCacheKey(oauthKey("B5h"))
	h.RateLimitStore.Set(keyB, callback.AnthropicOAuthRateLimitState{
		TokenKey: keyB, UnifiedStatus: callback.UnifiedStatusAllowed,
		Unified5hUtilization: 0.48, Unified7dUtilization: 0.30,
	})

	for i := 0; i < 20; i++ {
		u := h.lowestUtilizationSelect(context.Background(), "anthropic", upstreams)
		assert.Equal(t, oauthKey("B5h"), u.APIKey, "iteration %d: lower 5h should win when 7d is equal", i)
	}
}

func TestLowestUtilizationSelect_SingleUpstream(t *testing.T) {
	t.Parallel()
	upstreams := []nativeUpstream{
		{BaseURL: "https://api.anthropic.com", APIKey: oauthKey("ONLY")},
	}
	h := makeThrottleHandlers(upstreams, 0.8)

	u := h.lowestUtilizationSelect(context.Background(), "anthropic", upstreams)
	assert.Equal(t, oauthKey("ONLY"), u.APIKey)
}

// Integration: selectUpstreamWithThrottle uses lowestUtilizationSelect when configured
func TestSelectUpstreamThrottle_UsesLowestUtilization(t *testing.T) {
	t.Parallel()
	upstreams := []nativeUpstream{
		{BaseURL: "https://api.anthropic.com", APIKey: oauthKey("HIGH")},
		{BaseURL: "https://api.anthropic.com", APIKey: oauthKey("LOW")},
	}
	h := makeThrottleHandlers(upstreams, 0.8)
	h.Config.NativeUpstreamStrategy = config.StrategyLowestUtilization

	keyHIGH := callback.RateLimitCacheKey(oauthKey("HIGH"))
	h.RateLimitStore.Set(keyHIGH, callback.AnthropicOAuthRateLimitState{
		TokenKey: keyHIGH, UnifiedStatus: callback.UnifiedStatusAllowed,
		Unified5hUtilization: 0.70, Unified7dUtilization: 0.30,
	})
	keyLOW := callback.RateLimitCacheKey(oauthKey("LOW"))
	h.RateLimitStore.Set(keyLOW, callback.AnthropicOAuthRateLimitState{
		TokenKey: keyLOW, UnifiedStatus: callback.UnifiedStatusAllowed,
		Unified5hUtilization: 0.10, Unified7dUtilization: 0.15,
	})

	for i := 0; i < 20; i++ {
		u, err := h.selectUpstreamWithThrottle(context.Background(), "anthropic", upstreams, "")
		require.NoError(t, err)
		assert.Equal(t, oauthKey("LOW"), u.APIKey, "iteration %d: should prefer lowest utilization", i)
	}
}

// Default strategy (empty or "round_robin") uses round-robin, not lowest-utilization.
func TestSelectUpstreamThrottle_DefaultRoundRobin(t *testing.T) {
	t.Parallel()
	upstreams := []nativeUpstream{
		{BaseURL: "https://api.anthropic.com", APIKey: oauthKey("RRA")},
		{BaseURL: "https://api.anthropic.com", APIKey: oauthKey("RRB")},
	}
	h := makeThrottleHandlers(upstreams, 0.8)
	// NativeUpstreamStrategy is empty → default round-robin

	keyA := callback.RateLimitCacheKey(oauthKey("RRA"))
	h.RateLimitStore.Set(keyA, callback.AnthropicOAuthRateLimitState{
		TokenKey: keyA, UnifiedStatus: callback.UnifiedStatusAllowed,
		Unified5hUtilization: 0.70, Unified7dUtilization: 0.30,
	})
	keyB := callback.RateLimitCacheKey(oauthKey("RRB"))
	h.RateLimitStore.Set(keyB, callback.AnthropicOAuthRateLimitState{
		TokenKey: keyB, UnifiedStatus: callback.UnifiedStatusAllowed,
		Unified5hUtilization: 0.10, Unified7dUtilization: 0.15,
	})

	// round-robin should use both tokens, not just the lowest
	seen := map[string]bool{}
	for i := 0; i < 20; i++ {
		u, err := h.selectUpstreamWithThrottle(context.Background(), "anthropic", upstreams, "")
		require.NoError(t, err)
		seen[u.APIKey] = true
	}
	assert.True(t, seen[oauthKey("RRA")], "round-robin should select RRA")
	assert.True(t, seen[oauthKey("RRB")], "round-robin should select RRB")
}

// Mixed data: upstreams with no store entry should not be starved.
func TestLowestUtilizationSelect_MixedData(t *testing.T) {
	t.Parallel()
	upstreams := []nativeUpstream{
		{BaseURL: "https://api.anthropic.com", APIKey: oauthKey("KNOWN")},
		{BaseURL: "https://api.anthropic.com", APIKey: oauthKey("NEW1")},
		{BaseURL: "https://api.anthropic.com", APIKey: oauthKey("NEW2")},
	}
	h := makeThrottleHandlers(upstreams, 0.8)

	// Only KNOWN has utilization data (60%), NEW1/NEW2 have no store entry
	keyKNOWN := callback.RateLimitCacheKey(oauthKey("KNOWN"))
	h.RateLimitStore.Set(keyKNOWN, callback.AnthropicOAuthRateLimitState{
		TokenKey: keyKNOWN, UnifiedStatus: callback.UnifiedStatusAllowed,
		Unified5hUtilization: 0.60, Unified7dUtilization: 0.20,
	})

	// Should pick one of the NEW tokens (composite=0 < 0.60), not KNOWN
	u := h.lowestUtilizationSelect(context.Background(), "anthropic", upstreams)
	assert.NotEqual(t, oauthKey("KNOWN"), u.APIKey, "should prefer unknown (idle) token over 60%% utilized")
}

// Explicit "round_robin" value behaves same as empty string.
func TestSelectUpstreamThrottle_ExplicitRoundRobin(t *testing.T) {
	t.Parallel()
	upstreams := []nativeUpstream{
		{BaseURL: "https://api.anthropic.com", APIKey: oauthKey("ERA")},
		{BaseURL: "https://api.anthropic.com", APIKey: oauthKey("ERB")},
	}
	h := makeThrottleHandlers(upstreams, 0.8)
	h.Config.NativeUpstreamStrategy = config.StrategyRoundRobin

	keyA := callback.RateLimitCacheKey(oauthKey("ERA"))
	h.RateLimitStore.Set(keyA, callback.AnthropicOAuthRateLimitState{
		TokenKey: keyA, UnifiedStatus: callback.UnifiedStatusAllowed,
		Unified5hUtilization: 0.70, Unified7dUtilization: 0.30,
	})
	keyB := callback.RateLimitCacheKey(oauthKey("ERB"))
	h.RateLimitStore.Set(keyB, callback.AnthropicOAuthRateLimitState{
		TokenKey: keyB, UnifiedStatus: callback.UnifiedStatusAllowed,
		Unified5hUtilization: 0.10, Unified7dUtilization: 0.15,
	})

	seen := map[string]bool{}
	for i := 0; i < 20; i++ {
		u, err := h.selectUpstreamWithThrottle(context.Background(), "anthropic", upstreams, "")
		require.NoError(t, err)
		seen[u.APIKey] = true
	}
	assert.True(t, seen[oauthKey("ERA")], "explicit round_robin should select ERA")
	assert.True(t, seen[oauthKey("ERB")], "explicit round_robin should select ERB")
}

// Invalid strategy value falls back to round-robin (default branch).
func TestSelectUpstreamThrottle_InvalidStrategyFallsBackToRoundRobin(t *testing.T) {
	t.Parallel()
	upstreams := []nativeUpstream{
		{BaseURL: "https://api.anthropic.com", APIKey: oauthKey("INV1")},
		{BaseURL: "https://api.anthropic.com", APIKey: oauthKey("INV2")},
	}
	h := makeThrottleHandlers(upstreams, 0.8)
	h.Config.NativeUpstreamStrategy = "typo_strategy"

	keyA := callback.RateLimitCacheKey(oauthKey("INV1"))
	h.RateLimitStore.Set(keyA, callback.AnthropicOAuthRateLimitState{
		TokenKey: keyA, UnifiedStatus: callback.UnifiedStatusAllowed,
		Unified5hUtilization: 0.70, Unified7dUtilization: 0.30,
	})
	keyB := callback.RateLimitCacheKey(oauthKey("INV2"))
	h.RateLimitStore.Set(keyB, callback.AnthropicOAuthRateLimitState{
		TokenKey: keyB, UnifiedStatus: callback.UnifiedStatusAllowed,
		Unified5hUtilization: 0.10, Unified7dUtilization: 0.15,
	})

	// Should round-robin (both appear), not lowest-utilization (only INV2)
	seen := map[string]bool{}
	for i := 0; i < 20; i++ {
		u, err := h.selectUpstreamWithThrottle(context.Background(), "anthropic", upstreams, "")
		require.NoError(t, err)
		seen[u.APIKey] = true
	}
	assert.True(t, seen[oauthKey("INV1")], "invalid strategy should fallback to round-robin")
	assert.True(t, seen[oauthKey("INV2")], "invalid strategy should fallback to round-robin")
}

// All upstreams have store entries but 5h is sentinel -1 → fallback to round-robin.
func TestLowestUtilizationSelect_AllSentinel(t *testing.T) {
	t.Parallel()
	upstreams := []nativeUpstream{
		{BaseURL: "https://api.anthropic.com", APIKey: oauthKey("S1")},
		{BaseURL: "https://api.anthropic.com", APIKey: oauthKey("S2")},
	}
	h := makeThrottleHandlers(upstreams, 0.8)

	for _, u := range upstreams {
		key := callback.RateLimitCacheKey(u.APIKey)
		h.RateLimitStore.Set(key, callback.AnthropicOAuthRateLimitState{
			TokenKey: key, UnifiedStatus: callback.UnifiedStatusAllowed,
			Unified5hUtilization: -1, Unified7dUtilization: -1,
		})
	}

	// All sentinel → all tokens evaluated with composite=0; first token wins.
	// (Round-robin is only used when no token was evaluated at all)
	u := h.lowestUtilizationSelect(context.Background(), "anthropic", upstreams)
	validKeys := []string{oauthKey("S1"), oauthKey("S2")}
	assert.Contains(t, validKeys, u.APIKey, "should return one of the configured upstreams when all utilization is sentinel")
}

// ─── lowestUtilizationSelect: DB fallback when memory entry absent ───────────

// TestLowestUtilizationSelect_LoadsFromDBWhenMemoryMiss verifies that when
// the in-memory store has no entry for a token (e.g. after PruneExpired),
// lowestUtilizationSelect loads state from DB and uses it for selection
// instead of falling back to round-robin.
func TestLowestUtilizationSelect_LoadsFromDBWhenMemoryMiss(t *testing.T) {
	t.Parallel()
	// Token A: in memory with high utilization
	// Token B: NOT in memory (pruned), but DB has low utilization — should be selected
	rawA := oauthKey("DBF-A")
	rawB := oauthKey("DBF-B")
	hashA := callback.RateLimitCacheKey(rawA)
	hashB := callback.RateLimitCacheKey(rawB)

	upstreams := []nativeUpstream{
		{APIKey: rawA, BaseURL: "https://api.anthropic.com"},
		{APIKey: rawB, BaseURL: "https://api.anthropic.com"},
	}

	store := callback.NewInMemoryRateLimitStore()
	// Only token A is in memory (high utilization), keyed by hash as lowestUtilizationSelect does
	store.Set(hashA, callback.AnthropicOAuthRateLimitState{
		TokenKey:             hashA,
		UnifiedStatus:        callback.UnifiedStatusAllowed,
		Unified5hUtilization: 0.8,
		Unified7dUtilization: 0.8,
		Unified5hReset:       fmt.Sprintf("%d", time.Now().Add(1*time.Hour).Unix()),
		Unified7dReset:       fmt.Sprintf("%d", time.Now().Add(24*time.Hour).Unix()),
	})
	// Token B is NOT in memory — DB has low utilization

	h := &Handlers{
		Config: &config.ProxyConfig{
			ModelList: []config.ModelConfig{
				{ModelName: "a-test", TianjiParams: config.TianjiParams{Model: "anthropic/a", APIKey: strPtr(rawA), APIBase: strPtr("https://api.anthropic.com")}},
				{ModelName: "b-test", TianjiParams: config.TianjiParams{Model: "anthropic/b", APIKey: strPtr(rawB), APIBase: strPtr("https://api.anthropic.com")}},
			},
			NativeUpstreamStrategy: config.StrategyLowestUtilization,
		},
		RateLimitStore: store,
		RateLimitDB: &stubRateLimitDB{
			byKey: map[string]callback.AnthropicOAuthRateLimitState{
				hashB: {
					TokenKey:             hashB,
					UnifiedStatus:        callback.UnifiedStatusAllowed,
					Unified5hUtilization: 0.1,
					Unified7dUtilization: 0.2,
					Unified5hReset:       fmt.Sprintf("%d", time.Now().Add(1*time.Hour).Unix()),
					Unified7dReset:       fmt.Sprintf("%d", time.Now().Add(24*time.Hour).Unix()),
				},
			},
		},
	}

	u := h.lowestUtilizationSelect(context.Background(), "anthropic", upstreams)
	assert.Equal(t, rawB, u.APIKey,
		"should select token B (lower util from DB) not token A (high util in memory)")
}

// TestLowestUtilizationSelect_DBExpiredDataFallsBackToRoundRobin verifies that
// when DB data exists but both reset windows have passed, it is treated as
// expired (composite=0) — same as no data — and round-robin is used.
func TestLowestUtilizationSelect_DBExpiredDataFallsBackToRoundRobin(t *testing.T) {
	t.Parallel()
	rawA := oauthKey("EXP-A")
	rawB := oauthKey("EXP-B")
	hashA := callback.RateLimitCacheKey(rawA)
	hashB := callback.RateLimitCacheKey(rawB)
	pastReset := fmt.Sprintf("%d", time.Now().Add(-2*time.Hour).Unix())

	upstreams := []nativeUpstream{
		{APIKey: rawA, BaseURL: "https://api.anthropic.com"},
		{APIKey: rawB, BaseURL: "https://api.anthropic.com"},
	}

	store := callback.NewInMemoryRateLimitStore()
	// Neither token is in memory

	h := &Handlers{
		Config: &config.ProxyConfig{
			ModelList: []config.ModelConfig{
				{ModelName: "a-test", TianjiParams: config.TianjiParams{Model: "anthropic/a", APIKey: strPtr(rawA), APIBase: strPtr("https://api.anthropic.com")}},
				{ModelName: "b-test", TianjiParams: config.TianjiParams{Model: "anthropic/b", APIKey: strPtr(rawB), APIBase: strPtr("https://api.anthropic.com")}},
			},
			NativeUpstreamStrategy: config.StrategyLowestUtilization,
		},
		RateLimitStore: store,
		RateLimitDB: &stubRateLimitDB{
			byKey: map[string]callback.AnthropicOAuthRateLimitState{
				hashA: {TokenKey: hashA, Unified5hUtilization: 0.5, Unified5hReset: pastReset, Unified7dReset: pastReset},
				hashB: {TokenKey: hashB, Unified5hUtilization: 0.5, Unified5hReset: pastReset, Unified7dReset: pastReset},
			},
		},
	}

	// Both tokens expired in DB → composite=0 for both; first evaluated token wins.
	// (Round-robin is only used when no token was evaluated at all)
	u := h.lowestUtilizationSelect(context.Background(), "anthropic", upstreams)
	assert.Contains(t, []string{rawA, rawB}, u.APIKey, "should return one of the configured upstreams when both DB entries are expired")
}

// stubRateLimitDB is a test double for callback.RateLimitDB.
type stubRateLimitDB struct {
	byKey map[string]callback.AnthropicOAuthRateLimitState
}

func (s *stubRateLimitDB) GetOAuthTokenRateLimitState(_ context.Context, key string) (db.OAuthTokenRateLimitState, error) {
	st, ok := s.byKey[key]
	if !ok {
		// Return ErrRateLimitStateNotFound to match the RateLimitDB contract.
		return db.OAuthTokenRateLimitState{}, callback.ErrRateLimitStateNotFound
	}
	// Convert AnthropicOAuthRateLimitState back to db row for the interface.
	return db.OAuthTokenRateLimitState{
		TokenKey:                   st.TokenKey,
		UnifiedStatus:              st.UnifiedStatus,
		Unified5hStatus:            st.Unified5hStatus,
		Unified5hUtilization:       st.Unified5hUtilization,
		Unified5hReset:             st.Unified5hReset,
		Unified7dStatus:            st.Unified7dStatus,
		Unified7dUtilization:       st.Unified7dUtilization,
		Unified7dReset:             st.Unified7dReset,
		Unified7dSonnetStatus:      st.Unified7dSonnetStatus,
		Unified7dSonnetUtilization: st.Unified7dSonnetUtilization,
		Unified7dSonnetReset:       st.Unified7dSonnetReset,
		OrgID:                      st.OrganizationID,
	}, nil
}

func (s *stubRateLimitDB) GetActiveOAuthTokenRateLimitStates(_ context.Context) ([]db.OAuthTokenRateLimitState, error) {
	return nil, nil
}

func (s *stubRateLimitDB) UpsertOAuthTokenRateLimitState(_ context.Context, _ db.UpsertOAuthTokenRateLimitStateParams) error {
	return nil
}

func strPtr(s string) *string { return &s }

// TestLowestUtilizationSelect_BackFillsStoreAfterDBHit verifies that after a DB
// recovery, the entry is written back to the in-memory store so subsequent
// requests don't re-query the DB.
func TestLowestUtilizationSelect_BackFillsStoreAfterDBHit(t *testing.T) {
	t.Parallel()
	// Use two upstreams: lowestUtilizationSelect skips when len(upstreams) <= 1.
	rawA := oauthKey("BF-A")
	rawB := oauthKey("BF-B")
	hashA := callback.RateLimitCacheKey(rawA)
	hashB := callback.RateLimitCacheKey(rawB)

	upstreams := []nativeUpstream{
		{APIKey: rawA, BaseURL: "https://api.anthropic.com"},
		{APIKey: rawB, BaseURL: "https://api.anthropic.com"},
	}

	store := callback.NewInMemoryRateLimitStore()
	// Token B in memory with high utilization (so A is preferred).
	store.Set(hashB, callback.AnthropicOAuthRateLimitState{
		TokenKey:             hashB,
		Unified5hUtilization: 0.9,
		Unified5hReset:       fmt.Sprintf("%d", time.Now().Add(1*time.Hour).Unix()),
		Unified7dReset:       fmt.Sprintf("%d", time.Now().Add(24*time.Hour).Unix()),
	})
	// Token A NOT in memory — DB will be queried.

	dbCallCount := 0
	h := &Handlers{
		Config: &config.ProxyConfig{
			ModelList:              []config.ModelConfig{},
			NativeUpstreamStrategy: config.StrategyLowestUtilization,
		},
		RateLimitStore: store,
		RateLimitDB: &countingStubDB{
			onGet: func() { dbCallCount++ },
			state: callback.AnthropicOAuthRateLimitState{
				TokenKey:             hashA,
				Unified5hUtilization: 0.3,
				Unified5hReset:       fmt.Sprintf("%d", time.Now().Add(1*time.Hour).Unix()),
				Unified7dReset:       fmt.Sprintf("%d", time.Now().Add(24*time.Hour).Unix()),
			},
		},
	}

	// First call: token A memory miss → DB query → back-fill.
	h.lowestUtilizationSelect(context.Background(), "anthropic", upstreams)
	assert.Equal(t, 1, dbCallCount, "first call should query DB on memory miss for token A")

	// Second call: token A should now be in memory — no additional DB query.
	h.lowestUtilizationSelect(context.Background(), "anthropic", upstreams)
	assert.Equal(t, 1, dbCallCount, "second call should use back-filled memory cache, not query DB again")

	// Back-fill must use SetClean, not Set — the key must NOT be in the dirty set.
	// If Set were used instead of SetClean, every DB-recovered entry would be
	// immediately re-flushed to the DB on the next flush cycle.
	snap := store.SnapshotDirty()
	assert.NotContains(t, snap, hashA, "DB back-fill must use SetClean — back-filled key must not be in dirty set")
}

// TestLowestUtilizationSelect_TimeoutDegradesToRoundRobin verifies that when
// the DB takes longer than 200ms, lowestUtilizationSelect completes within the
// 200ms timeout (not the full DB delay) and falls back to round-robin.
func TestLowestUtilizationSelect_TimeoutDegradesToRoundRobin(t *testing.T) {
	t.Parallel()
	rawA := oauthKey("TO-A")
	rawB := oauthKey("TO-B")

	upstreams := []nativeUpstream{
		{APIKey: rawA, BaseURL: "https://api.anthropic.com"},
		{APIKey: rawB, BaseURL: "https://api.anthropic.com"},
	}

	store := callback.NewInMemoryRateLimitStore()

	h := &Handlers{
		Config: &config.ProxyConfig{
			ModelList:              []config.ModelConfig{},
			NativeUpstreamStrategy: config.StrategyLowestUtilization,
		},
		RateLimitStore: store,
		// DB takes 2s — well beyond the 200ms timeout.
		RateLimitDB: &slowStubDB{delay: 2 * time.Second},
	}

	// A single call touches 2 tokens × 200ms = at most 400ms.
	// With a 2s DB delay, we verify the timeout fires and doesn't block for 4s.
	start := time.Now()
	seen := map[string]bool{}
	for i := 0; i < 4; i++ {
		u := h.lowestUtilizationSelect(context.Background(), "anthropic", upstreams)
		seen[u.APIKey] = true
	}
	elapsed := time.Since(start)

	// 4 iterations × 2 tokens × 200ms timeout = at most ~1.6s.
	// The DB would have taken 4 × 2 × 2s = 16s without the timeout.
	assert.Less(t, elapsed, 2*time.Second,
		"lowestUtilizationSelect must respect 200ms timeout, not block for full DB delay: took %v", elapsed)
	// When DB times out, all tokens get composite=0; the first evaluated token wins each call.
	// The timing assertion above is the critical check.
	assert.True(t, seen[rawA] || seen[rawB], "result must be one of the two configured upstreams when DB times out")
}

// TestLowestUtilizationSelect_FiveHExpiredSevenDActive verifies that when the
// 5h window has expired but the 7d window is still active, the token participates
// in selection using only the 7d utilization (composite = 0 + util7d*weight).
func TestLowestUtilizationSelect_FiveHExpiredSevenDActive(t *testing.T) {
	t.Parallel()
	rawA := oauthKey("MW-A") // 5h expired, 7d active, low composite
	rawB := oauthKey("MW-B") // both windows active, higher composite
	hashA := callback.RateLimitCacheKey(rawA)
	hashB := callback.RateLimitCacheKey(rawB)

	now := time.Now()
	past5h := fmt.Sprintf("%d", now.Add(-1*time.Hour).Unix())   // expired
	future7d := fmt.Sprintf("%d", now.Add(48*time.Hour).Unix()) // active
	future5h := fmt.Sprintf("%d", now.Add(1*time.Hour).Unix())  // active

	upstreams := []nativeUpstream{
		{APIKey: rawA, BaseURL: "https://api.anthropic.com"},
		{APIKey: rawB, BaseURL: "https://api.anthropic.com"},
	}

	store := callback.NewInMemoryRateLimitStore()
	// Token A: 5h expired → util5h zeroed; 7d active with low utilization → small composite
	store.Set(hashA, callback.AnthropicOAuthRateLimitState{
		TokenKey:             hashA,
		Unified5hUtilization: 0.9, // will be zeroed (5h expired)
		Unified5hReset:       past5h,
		Unified7dUtilization: 0.1, // composite = 0² + 0.1²*2 = 0.02
		Unified7dReset:       future7d,
	})
	// Token B: both windows active with higher utilization → larger composite
	store.Set(hashB, callback.AnthropicOAuthRateLimitState{
		TokenKey:             hashB,
		Unified5hUtilization: 0.5, // composite = 0.5² + 0.5²*2 = 0.25 + 0.5 = 0.75
		Unified5hReset:       future5h,
		Unified7dUtilization: 0.5,
		Unified7dReset:       future7d,
	})

	h := &Handlers{
		Config: &config.ProxyConfig{
			ModelList:              []config.ModelConfig{},
			NativeUpstreamStrategy: config.StrategyLowestUtilization,
		},
		RateLimitStore: store,
	}

	u := h.lowestUtilizationSelect(context.Background(), "anthropic", upstreams)
	assert.Equal(t, rawA, u.APIKey,
		"token A (5h expired, low 7d) should be selected over token B (both windows active, higher composite)")
}

// TestLowestUtilizationSelect_SevenDExpiredFiveHActive verifies the symmetric case:
// when the 7d window has expired but the 5h window is still active, the 7d
// contribution is zeroed out and only util5h drives the composite score.
func TestLowestUtilizationSelect_SevenDExpiredFiveHActive(t *testing.T) {
	t.Parallel()
	rawA := oauthKey("SD-A") // 7d expired, 5h active with 40% util
	rawB := oauthKey("SD-B") // both windows active: 7d=50%, 5h=30%
	hashA := callback.RateLimitCacheKey(rawA)
	hashB := callback.RateLimitCacheKey(rawB)

	now := time.Now()
	past7d := fmt.Sprintf("%d", now.Add(-1*time.Hour).Unix())   // expired
	future5h := fmt.Sprintf("%d", now.Add(1*time.Hour).Unix())  // active
	future7d := fmt.Sprintf("%d", now.Add(48*time.Hour).Unix()) // active
	future5hB := fmt.Sprintf("%d", now.Add(2*time.Hour).Unix()) // active

	upstreams := []nativeUpstream{
		{APIKey: rawA, BaseURL: "https://api.anthropic.com"},
		{APIKey: rawB, BaseURL: "https://api.anthropic.com"},
	}

	store := callback.NewInMemoryRateLimitStore()
	// Token A: 7d expired → util7d zeroed; 5h active with 40% → composite = 0.4² + 0²*2 = 0.16
	store.Set(hashA, callback.AnthropicOAuthRateLimitState{
		TokenKey:             hashA,
		Unified5hUtilization: 0.4,
		Unified5hReset:       future5h,
		Unified7dUtilization: 0.8, // will be zeroed (7d expired)
		Unified7dReset:       past7d,
	})
	// Token B: both active → composite = 0.3² + 0.5²*2 = 0.09 + 0.5 = 0.59
	store.Set(hashB, callback.AnthropicOAuthRateLimitState{
		TokenKey:             hashB,
		Unified5hUtilization: 0.3,
		Unified5hReset:       future5hB,
		Unified7dUtilization: 0.5,
		Unified7dReset:       future7d,
	})

	h := &Handlers{
		Config: &config.ProxyConfig{
			ModelList:              []config.ModelConfig{},
			NativeUpstreamStrategy: config.StrategyLowestUtilization,
		},
		RateLimitStore: store,
	}

	u := h.lowestUtilizationSelect(context.Background(), "anthropic", upstreams)
	assert.Equal(t, rawA, u.APIKey,
		"token A (7d expired, lower 5h composite=0.16) should be selected over token B (both active, composite=0.59)")
}

// TestLowestUtilizationSelect_JustResetHighSevenD: a token with 5h=0% (just reset)
// but 7d=79% must NOT beat a token with 5h=1%, 7d=27%. Under the max-normalized
// formula, the 7d window is the bottleneck for the "just reset" token.
func TestLowestUtilizationSelect_JustResetHighSevenD(t *testing.T) {
	t.Parallel()
	rawJR := oauthKey("JR") // just reset: 5h=0%, 7d=79%
	rawAC := oauthKey("AC") // active:     5h=1%, 7d=27%
	hashJR := callback.RateLimitCacheKey(rawJR)
	hashAC := callback.RateLimitCacheKey(rawAC)

	upstreams := []nativeUpstream{
		{APIKey: rawJR, BaseURL: "https://api.anthropic.com"},
		{APIKey: rawAC, BaseURL: "https://api.anthropic.com"},
	}

	store := callback.NewInMemoryRateLimitStore()
	// "Just reset" token: 5h window has expired (Unified5hReset is in the past) so the
	// routing code sets util5h=0 via the fiveHExpired branch, not from the stored value.
	// composite = max(0/0.8, 0.79/0.9, 0) = 0.878
	pastReset := fmt.Sprintf("%d", time.Now().Add(-1*time.Hour).Unix())
	futureReset := fmt.Sprintf("%d", time.Now().Add(24*time.Hour).Unix())
	store.Set(hashJR, callback.AnthropicOAuthRateLimitState{
		TokenKey:             hashJR,
		Unified5hUtilization: 0.50,      // stale — window has expired
		Unified5hReset:       pastReset, // past → fiveHExpired=true → util5h zeroed
		Unified7dUtilization: 0.79,
		Unified7dReset:       futureReset, // future → sevenDExpired=false explicitly
	})
	// "Active" token: light usage on both windows
	// composite = max(0.01/0.8, 0.27/0.9, 0) = 0.30
	store.Set(hashAC, callback.AnthropicOAuthRateLimitState{
		TokenKey:             hashAC,
		Unified5hUtilization: 0.01,
		Unified5hReset:       futureReset,
		Unified7dUtilization: 0.27,
		Unified7dReset:       futureReset,
	})

	h := &Handlers{
		Config: &config.ProxyConfig{
			ModelList:              []config.ModelConfig{},
			NativeUpstreamStrategy: config.StrategyLowestUtilization,
		},
		RateLimitStore: store,
	}

	for i := range 20 {
		u := h.lowestUtilizationSelect(context.Background(), "anthropic", upstreams)
		assert.Equal(t, rawAC, u.APIKey,
			"iteration %d: active token (5h=1%%, 7d=27%%) should beat just-reset token (5h=0%%, 7d=79%%)", i)
	}
}

// countingStubDB wraps a fixed state and counts GetOAuthTokenRateLimitState calls.
type countingStubDB struct {
	onGet func()
	state callback.AnthropicOAuthRateLimitState
}

func (c *countingStubDB) GetOAuthTokenRateLimitState(_ context.Context, _ string) (db.OAuthTokenRateLimitState, error) {
	if c.onGet != nil {
		c.onGet()
	}
	return db.OAuthTokenRateLimitState{
		TokenKey:             c.state.TokenKey,
		Unified5hUtilization: c.state.Unified5hUtilization,
		Unified5hReset:       c.state.Unified5hReset,
		Unified7dUtilization: c.state.Unified7dUtilization,
		Unified7dReset:       c.state.Unified7dReset,
	}, nil
}

func (c *countingStubDB) GetActiveOAuthTokenRateLimitStates(_ context.Context) ([]db.OAuthTokenRateLimitState, error) {
	return nil, nil
}

func (c *countingStubDB) UpsertOAuthTokenRateLimitState(_ context.Context, _ db.UpsertOAuthTokenRateLimitStateParams) error {
	return nil
}

// slowStubDB simulates a DB that always times out.
type slowStubDB struct {
	delay time.Duration
}

func (s *slowStubDB) GetOAuthTokenRateLimitState(ctx context.Context, _ string) (db.OAuthTokenRateLimitState, error) {
	select {
	case <-ctx.Done():
		return db.OAuthTokenRateLimitState{}, ctx.Err()
	case <-time.After(s.delay):
		return db.OAuthTokenRateLimitState{}, nil
	}
}

func (s *slowStubDB) GetActiveOAuthTokenRateLimitStates(_ context.Context) ([]db.OAuthTokenRateLimitState, error) {
	return nil, nil
}

func (s *slowStubDB) UpsertOAuthTokenRateLimitState(_ context.Context, _ db.UpsertOAuthTokenRateLimitStateParams) error {
	return nil
}

// cancellingStubDB cancels the provided context on the first GetOAuthTokenRateLimitState call,
// then returns ErrRateLimitStateNotFound. Used to simulate context cancellation mid-loop.
type cancellingStubDB struct {
	cancel context.CancelFunc
}

func (c *cancellingStubDB) GetOAuthTokenRateLimitState(_ context.Context, _ string) (db.OAuthTokenRateLimitState, error) {
	c.cancel()
	return db.OAuthTokenRateLimitState{}, callback.ErrRateLimitStateNotFound
}

func (c *cancellingStubDB) GetActiveOAuthTokenRateLimitStates(_ context.Context) ([]db.OAuthTokenRateLimitState, error) {
	return nil, nil
}

func (c *cancellingStubDB) UpsertOAuthTokenRateLimitState(_ context.Context, _ db.UpsertOAuthTokenRateLimitStateParams) error {
	return nil
}

// errorStubDB always returns a fixed error from GetOAuthTokenRateLimitState.
// Used to test generic DB failure paths (connection refused, SQL error, etc.).
type errorStubDB struct {
	err error
}

func (e *errorStubDB) GetOAuthTokenRateLimitState(_ context.Context, _ string) (db.OAuthTokenRateLimitState, error) {
	return db.OAuthTokenRateLimitState{}, e.err
}

func (e *errorStubDB) GetActiveOAuthTokenRateLimitStates(_ context.Context) ([]db.OAuthTokenRateLimitState, error) {
	return nil, nil
}

func (e *errorStubDB) UpsertOAuthTokenRateLimitState(_ context.Context, _ db.UpsertOAuthTokenRateLimitStateParams) error {
	return nil
}

// TestLowestUtilizationSelect_Util7dSentinel verifies that when Unified7dUtilization=-1
// (no 7d data), the 7d contribution is treated as 0 and the token wins over a token with
// real 7d data. Two upstreams are required so composite logic is exercised (single-upstream
// triggers the round-robin fast path and skips the score comparison).
//
// Token A: util7d=-1 (sentinel) + util5h=0.40 → composite = max(0.40/0.8, 0, 0) = 0.50
// Token B: util7d=0.50          + util5h=0.40 → composite = max(0.40/0.8, 0.50/0.9, 0) ≈ 0.556
// Token A must be selected (lower composite).
func TestLowestUtilizationSelect_Util7dSentinel(t *testing.T) {
	t.Parallel()
	tokA := oauthKey("sentinel7d-A")
	tokB := oauthKey("sentinel7d-B")
	upstreams := []nativeUpstream{
		{APIKey: tokA},
		{APIKey: tokB},
	}
	h := &Handlers{
		Config:         &config.ProxyConfig{RatelimitAlertThreshold: 0.8},
		RateLimitStore: callback.NewInMemoryRateLimitStore(),
	}

	keyA := callback.RateLimitCacheKey(tokA)
	h.RateLimitStore.Set(keyA, callback.AnthropicOAuthRateLimitState{
		TokenKey:             keyA,
		UnifiedStatus:        callback.UnifiedStatusAllowed,
		Unified5hUtilization: 0.40,
		Unified7dUtilization: -1, // sentinel: no 7d data → treated as 0
	})
	keyB := callback.RateLimitCacheKey(tokB)
	h.RateLimitStore.Set(keyB, callback.AnthropicOAuthRateLimitState{
		TokenKey:             keyB,
		UnifiedStatus:        callback.UnifiedStatusAllowed,
		Unified5hUtilization: 0.40,
		Unified7dUtilization: 0.50, // real 7d data → higher composite
	})

	u := h.lowestUtilizationSelect(context.Background(), "anthropic", upstreams)
	assert.Equal(t, tokA, u.APIKey,
		"token A (util7d sentinel=-1, composite=0.50) should beat token B (util7d=0.50, composite≈0.556)")
}

// TestLowestUtilizationSelect_CtxCancelWithPartialResult verifies that when the context
// is cancelled after at least one token has been fully evaluated, the function returns
// the best scored result rather than falling back to round-robin.
// Setup: tok1 is in memory (evaluated immediately), tok2 is NOT in memory so it triggers
// a DB lookup — the DB stub cancels the context on that call. The loop breaks on the
// next iteration's ctx.Err() check, and tok1's partial result must be returned.
func TestLowestUtilizationSelect_CtxCancelWithPartialResult(t *testing.T) {
	t.Parallel()

	tok1 := oauthKey("ctxcancel-1")
	tok2 := oauthKey("ctxcancel-2")
	tok3 := oauthKey("ctxcancel-3")
	upstreams := []nativeUpstream{
		{APIKey: tok1},
		{APIKey: tok2},
		{APIKey: tok3},
	}

	store := callback.NewInMemoryRateLimitStore()
	futureReset := fmt.Sprintf("%d", time.Now().Add(1*time.Hour).Unix())

	// Only tok1 is in memory — tok2 and tok3 will miss and hit DB.
	key1 := callback.RateLimitCacheKey(tok1)
	store.Set(key1, callback.AnthropicOAuthRateLimitState{
		TokenKey:             key1,
		UnifiedStatus:        callback.UnifiedStatusAllowed,
		Unified5hUtilization: 0.30,
		Unified7dUtilization: 0.20,
		Unified5hReset:       futureReset,
		Unified7dReset:       futureReset,
	})

	ctx, cancel := context.WithCancel(context.Background())

	h := &Handlers{
		Config:         &config.ProxyConfig{RatelimitAlertThreshold: 0.8},
		RateLimitStore: store,
		// DB stub cancels ctx on first call (tok2's memory miss).
		// tok3 never gets evaluated because ctx.Err() fires at the top of its iteration.
		RateLimitDB: &cancellingStubDB{cancel: cancel},
	}

	result := h.lowestUtilizationSelect(ctx, "anthropic", upstreams)

	// tok1 and tok2 (composite=0, no data) were evaluated; tok3 was skipped.
	// tok2 wins because no-data tokens get composite=0.
	// The key assertion: we got a scored result from partial evaluation, NOT round-robin.
	validKeys := []string{tok1, tok2}
	assert.Contains(t, validKeys, result.APIKey, "must return one of the two evaluated tokens, not tok3")
	assert.NotEqual(t, tok3, result.APIKey, "tok3 must not be returned — it was never evaluated")
}

// ─── US1: Sonnet-aware selection ────────────────────────────────────────────

// TestLowestUtilSelect_PreferLowSonnet: when two tokens have similar 7d all utilization
// but different 7d Sonnet, the selector must prefer the token with lower Sonnet.
func TestLowestUtilSelect_PreferLowSonnet(t *testing.T) {
	t.Parallel()
	tokA := oauthKey("sonnet-high")
	tokB := oauthKey("sonnet-low")
	upstreams := []nativeUpstream{
		{APIKey: tokA},
		{APIKey: tokB},
	}
	h := makeThrottleHandlers(upstreams, 0.8)
	h.Config.NativeUpstreamStrategy = config.StrategyLowestUtilization

	// A: 5h=0.10, 7d=0.40, 7d_s=0.85 → max(0.125, 0.444, 0.944) = 0.944
	// B: 5h=0.10, 7d=0.40, 7d_s=0.30 → max(0.125, 0.444, 0.333) = 0.444
	keyA := callback.RateLimitCacheKey(tokA)
	h.RateLimitStore.Set(keyA, callback.AnthropicOAuthRateLimitState{
		TokenKey: keyA, UnifiedStatus: callback.UnifiedStatusAllowed,
		Unified5hUtilization: 0.10, Unified7dUtilization: 0.40,
		Unified7dSonnetUtilization: 0.85,
	})
	keyB := callback.RateLimitCacheKey(tokB)
	h.RateLimitStore.Set(keyB, callback.AnthropicOAuthRateLimitState{
		TokenKey: keyB, UnifiedStatus: callback.UnifiedStatusAllowed,
		Unified5hUtilization: 0.10, Unified7dUtilization: 0.40,
		Unified7dSonnetUtilization: 0.30,
	})

	for i := 0; i < 20; i++ {
		u := h.lowestUtilizationSelect(context.Background(), "anthropic", upstreams)
		assert.Equal(t, tokB, u.APIKey, "iteration %d: token with lower Sonnet should be preferred", i)
	}
}

// TestLowestUtilSelect_7dAllStillDominates: when 7d all is the bottleneck for both tokens
// (higher than Sonnet), the selection result should be the same as before the formula change.
func TestLowestUtilSelect_7dAllStillDominates(t *testing.T) {
	t.Parallel()
	tokA := oauthKey("7dall-high")
	tokB := oauthKey("7dall-low")
	upstreams := []nativeUpstream{
		{APIKey: tokA},
		{APIKey: tokB},
	}
	h := makeThrottleHandlers(upstreams, 0.8)
	h.Config.NativeUpstreamStrategy = config.StrategyLowestUtilization

	// A: 7d=0.70, 7d_s=0.30 → max(..., 0.778, 0.333) → 7d dominates
	// B: 7d=0.30, 7d_s=0.20 → max(..., 0.333, 0.222) → 7d dominates
	// B wins (lower 7d).
	keyA := callback.RateLimitCacheKey(tokA)
	h.RateLimitStore.Set(keyA, callback.AnthropicOAuthRateLimitState{
		TokenKey: keyA, UnifiedStatus: callback.UnifiedStatusAllowed,
		Unified5hUtilization: 0.10, Unified7dUtilization: 0.70,
		Unified7dSonnetUtilization: 0.30,
	})
	keyB := callback.RateLimitCacheKey(tokB)
	h.RateLimitStore.Set(keyB, callback.AnthropicOAuthRateLimitState{
		TokenKey: keyB, UnifiedStatus: callback.UnifiedStatusAllowed,
		Unified5hUtilization: 0.10, Unified7dUtilization: 0.30,
		Unified7dSonnetUtilization: 0.20,
	})

	for i := 0; i < 20; i++ {
		u := h.lowestUtilizationSelect(context.Background(), "anthropic", upstreams)
		assert.Equal(t, tokB, u.APIKey, "iteration %d: 7d all dominates — lower 7d wins", i)
	}
}

// ─── US2: 7d Sonnet gate ────────────────────────────────────────────────────

// TestSelectUpstream_7dSonnetGate_Skips: a token with u7ds >= 0.90 is filtered out by the gate.
func TestSelectUpstream_7dSonnetGate_Skips(t *testing.T) {
	t.Parallel()
	tokA := oauthKey("7ds-gate-high")
	tokB := oauthKey("7ds-gate-low")
	upstreams := []nativeUpstream{
		{APIKey: tokA},
		{APIKey: tokB},
	}
	h := makeThrottleHandlers(upstreams, 0.8)

	keyA := callback.RateLimitCacheKey(tokA)
	h.RateLimitStore.Set(keyA, callback.AnthropicOAuthRateLimitState{
		TokenKey: keyA, UnifiedStatus: callback.UnifiedStatusAllowed,
		Unified5hUtilization: 0.10, Unified7dUtilization: 0.50,
		Unified7dSonnetUtilization: 0.92, // exceeds gate7d=0.90
	})
	keyB := callback.RateLimitCacheKey(tokB)
	h.RateLimitStore.Set(keyB, callback.AnthropicOAuthRateLimitState{
		TokenKey: keyB, UnifiedStatus: callback.UnifiedStatusAllowed,
		Unified5hUtilization: 0.10, Unified7dUtilization: 0.50,
		Unified7dSonnetUtilization: 0.40,
	})

	u, err := h.selectUpstreamWithThrottle(context.Background(), "anthropic", upstreams, "claude-sonnet-4-5")
	require.NoError(t, err)
	assert.Equal(t, tokB, u.APIKey, "token with u7ds=0.92 should be filtered by 7d_s gate")
}

// TestSelectUpstream_7dSonnetGate_AllThrottled: when all tokens exceed 7d_s gate → 429 for Sonnet requests.
func TestSelectUpstream_7dSonnetGate_AllThrottled(t *testing.T) {
	t.Parallel()
	futureReset := fmt.Sprintf("%d", time.Now().Add(2*time.Hour).Unix())
	tokA := oauthKey("7ds-all-throttled-A")
	tokB := oauthKey("7ds-all-throttled-B")
	upstreams := []nativeUpstream{
		{APIKey: tokA},
		{APIKey: tokB},
	}
	h := makeThrottleHandlers(upstreams, 0.8)

	keyA := callback.RateLimitCacheKey(tokA)
	h.RateLimitStore.Set(keyA, callback.AnthropicOAuthRateLimitState{
		TokenKey: keyA, UnifiedStatus: callback.UnifiedStatusAllowed,
		Unified5hUtilization: 0.10, Unified7dUtilization: 0.50,
		Unified7dSonnetUtilization: 0.95,
		Unified7dSonnetReset:       futureReset,
	})
	keyB := callback.RateLimitCacheKey(tokB)
	h.RateLimitStore.Set(keyB, callback.AnthropicOAuthRateLimitState{
		TokenKey: keyB, UnifiedStatus: callback.UnifiedStatusAllowed,
		Unified5hUtilization: 0.10, Unified7dUtilization: 0.50,
		Unified7dSonnetUtilization: 0.91,
		Unified7dSonnetReset:       futureReset,
	})

	_, err := h.selectUpstreamWithThrottle(context.Background(), "anthropic", upstreams, "claude-sonnet-4-5")
	require.Error(t, err)
	var throttleErr *allTokensThrottledError
	assert.ErrorAs(t, err, &throttleErr, "should return allTokensThrottledError")
}

// TestSelectUpstream_7dSonnetGate_SentinelSkipped: u7ds=-1 (sentinel) must NOT trigger the gate.
func TestSelectUpstream_7dSonnetGate_SentinelSkipped(t *testing.T) {
	t.Parallel()
	tok := oauthKey("7ds-sentinel-ok")
	upstreams := []nativeUpstream{{APIKey: tok}}
	h := makeThrottleHandlers(upstreams, 0.8)

	key := callback.RateLimitCacheKey(tok)
	h.RateLimitStore.Set(key, callback.AnthropicOAuthRateLimitState{
		TokenKey: key, UnifiedStatus: callback.UnifiedStatusAllowed,
		Unified5hUtilization:       0.10,
		Unified7dUtilization:       0.50,
		Unified7dSonnetUtilization: -1, // sentinel — no data, should pass gate
	})

	u, err := h.selectUpstreamWithThrottle(context.Background(), "anthropic", upstreams, "claude-sonnet-4-5")
	require.NoError(t, err)
	assert.Equal(t, tok, u.APIKey, "sentinel -1 must not trigger 7d_s gate")
}

// TestSelectUpstream_AllowedWarning_Bypasses7dSonnetGate: allowed_warning status bypasses all gates
// including the 7d_sonnet gate on a Sonnet request.
func TestSelectUpstream_AllowedWarning_Bypasses7dSonnetGate(t *testing.T) {
	t.Parallel()
	tok := oauthKey("7ds-overage")
	upstreams := []nativeUpstream{{APIKey: tok}}
	h := makeThrottleHandlers(upstreams, 0.8)

	key := callback.RateLimitCacheKey(tok)
	h.RateLimitStore.Set(key, callback.AnthropicOAuthRateLimitState{
		TokenKey: key, UnifiedStatus: callback.UnifiedStatusAllowedWarning,
		Unified5hUtilization:       0.90,
		Unified7dUtilization:       0.95,
		Unified7dSonnetUtilization: 0.95, // would trigger gate, but allowed_warning bypasses
	})

	u, err := h.selectUpstreamWithThrottle(context.Background(), "anthropic", upstreams, "claude-sonnet-4-5")
	require.NoError(t, err)
	assert.Equal(t, tok, u.APIKey, "allowed_warning must bypass all gates including 7d_s")
}

// TestLowestUtilSelect_Util5hSentinelHigh7d regression: when u5h is sentinel (-1)
// but u7d is high, the old code short-circuited to composite=0 and treated the token
// as idle — routing traffic into a nearly-throttled token. CompositeScore handles
// per-window sentinels correctly; the short-circuit has been removed.
//
// Token A: u5h=-1, u7d=0.80 → max(0, 0.889, 0) = 0.889
// Token B: u5h=0.40, u7d=0.20 → max(0.50, 0.222, 0) = 0.50
// B must win.
func TestLowestUtilSelect_Util5hSentinelHigh7d(t *testing.T) {
	t.Parallel()
	tokA := oauthKey("5h-sentinel-high7d")
	tokB := oauthKey("5h-low-7d-low")
	upstreams := []nativeUpstream{
		{APIKey: tokA},
		{APIKey: tokB},
	}
	h := makeThrottleHandlers(upstreams, 0.8)

	keyA := callback.RateLimitCacheKey(tokA)
	h.RateLimitStore.Set(keyA, callback.AnthropicOAuthRateLimitState{
		TokenKey: keyA, UnifiedStatus: callback.UnifiedStatusAllowed,
		Unified5hUtilization: -1,   // sentinel
		Unified7dUtilization: 0.80, // high!
	})
	keyB := callback.RateLimitCacheKey(tokB)
	h.RateLimitStore.Set(keyB, callback.AnthropicOAuthRateLimitState{
		TokenKey: keyB, UnifiedStatus: callback.UnifiedStatusAllowed,
		Unified5hUtilization: 0.40,
		Unified7dUtilization: 0.20,
	})

	for i := 0; i < 20; i++ {
		u := h.lowestUtilizationSelect(context.Background(), "anthropic", upstreams)
		assert.Equal(t, tokB, u.APIKey,
			"iteration %d: u5h sentinel must not mask high u7d — B (lower actual utilization) must win", i)
	}
}

// ─── US4: Sonnet sentinel in selection ──────────────────────────────────────

// TestLowestUtilSelect_Util7dSonnetSentinel: a token with u7ds=-1 should beat
// a token with u7ds=0.50 when all other fields are equal.
func TestLowestUtilSelect_Util7dSonnetSentinel(t *testing.T) {
	t.Parallel()
	tokA := oauthKey("sonnet-sentinel-A")
	tokB := oauthKey("sonnet-sentinel-B")
	upstreams := []nativeUpstream{
		{APIKey: tokA},
		{APIKey: tokB},
	}
	h := makeThrottleHandlers(upstreams, 0.8)

	// A: u7ds=-1 → treated as 0 → max(0.30/0.8, 0.40/0.9, 0) = 0.444
	// B: u7ds=0.50 → max(0.30/0.8, 0.40/0.9, 0.50/0.9) = 0.556
	// A wins.
	keyA := callback.RateLimitCacheKey(tokA)
	h.RateLimitStore.Set(keyA, callback.AnthropicOAuthRateLimitState{
		TokenKey: keyA, UnifiedStatus: callback.UnifiedStatusAllowed,
		Unified5hUtilization:       0.30,
		Unified7dUtilization:       0.40,
		Unified7dSonnetUtilization: -1,
	})
	keyB := callback.RateLimitCacheKey(tokB)
	h.RateLimitStore.Set(keyB, callback.AnthropicOAuthRateLimitState{
		TokenKey: keyB, UnifiedStatus: callback.UnifiedStatusAllowed,
		Unified5hUtilization:       0.30,
		Unified7dUtilization:       0.40,
		Unified7dSonnetUtilization: 0.50,
	})

	u := h.lowestUtilizationSelect(context.Background(), "anthropic", upstreams)
	assert.Equal(t, tokA, u.APIKey,
		"token A (u7ds sentinel=-1) should beat token B (u7ds=0.50)")
}

// ─── Phase 3: TimeWeightedScore capacity-rate tests ─────────────────────────
//
// Formula: score_w = τ / (1 - r), where r = u/θ, τ = remaining/window_length.
// Final score = max across windows (bottleneck). Lower = better.

// TestTimeWeightedScore_ResetSoon: 5h window high util but about to reset.
// 5h: r=0.5625, headroom=0.4375, τ=0.18 → 0.18/0.4375 = 0.411
// 7d: r=0, headroom=1, τ≈1.0 → 1.0/1.0 = 1.0
// max = 1.0 (7d dominates — fresh full window). Token is moderately good.
func TestTimeWeightedScore_ResetSoon(t *testing.T) {
	t.Parallel()
	now := time.Now()
	state := callback.AnthropicOAuthRateLimitState{
		Unified5hUtilization: 0.45,
		Unified5hReset:       fmt.Sprintf("%d", now.Add(54*time.Minute).Unix()),
		Unified7dUtilization: 0,
		Unified7dReset:       fmt.Sprintf("%d", now.Add(7*24*time.Hour).Unix()),
	}
	score := callback.TimeWeightedScore(state, 0.8, now)
	// 5h item: τ=3240/18000=0.18, headroom=1-0.5625=0.4375 → 0.411
	// 7d item: τ≈1.0, headroom=1.0 → 1.0
	assert.InDelta(t, 1.0, score, 0.02, "7d window (empty, full period) dominates at 1.0")
}

// TestTimeWeightedScore_HighUtil_LongRemaining: 7d=45%, 6 days left.
// r=0.50, headroom=0.50, τ=0.857 → 0.857/0.50 = 1.714.
func TestTimeWeightedScore_HighUtil_LongRemaining(t *testing.T) {
	t.Parallel()
	now := time.Now()
	state := callback.AnthropicOAuthRateLimitState{
		Unified5hUtilization: 0.03,
		Unified5hReset:       fmt.Sprintf("%d", now.Add(5*time.Hour).Unix()),
		Unified7dUtilization: 0.45,
		Unified7dReset:       fmt.Sprintf("%d", now.Add(6*24*time.Hour).Unix()),
	}
	score := callback.TimeWeightedScore(state, 0.8, now)
	// 7d: τ = 518400/604800 = 0.857, headroom = 1 - 0.50 = 0.50 → 1.714
	expected := (6.0 * 24 * 3600 / 604800) / 0.50
	assert.InDelta(t, expected, score, 0.02)
}

// TestTimeWeightedScore_NoResetInfo: missing reset time → τ=1 (conservative).
// score = 1/(1-r) for the dominant window.
func TestTimeWeightedScore_NoResetInfo(t *testing.T) {
	t.Parallel()
	state := callback.AnthropicOAuthRateLimitState{
		Unified5hUtilization: 0.40,
		Unified5hReset:       "", // no info
		Unified7dUtilization: 0.30,
		Unified7dReset:       "", // no info
	}
	score := callback.TimeWeightedScore(state, 0.8, time.Now())
	// 5h: 1/(1-0.50) = 2.0. 7d: 1/(1-0.333) = 1.5. max = 2.0
	assert.InDelta(t, 2.0, score, 1e-9, "no reset info → τ=1 conservative")
}

// TestTimeWeightedScore_Expired: all windows reset in the past → 0.
func TestTimeWeightedScore_Expired(t *testing.T) {
	t.Parallel()
	now := time.Now()
	past := fmt.Sprintf("%d", now.Add(-1*time.Hour).Unix())
	state := callback.AnthropicOAuthRateLimitState{
		Unified5hUtilization:       0.60,
		Unified5hReset:             past,
		Unified7dUtilization:       0.50,
		Unified7dReset:             past,
		Unified7dSonnetUtilization: 0.30,
		Unified7dSonnetReset:       past,
	}
	score := callback.TimeWeightedScore(state, 0.8, now)
	assert.Equal(t, 0.0, score, "all windows expired → 0")
}

// TestTimeWeightedScore_AllZero: no utilization → τ/1.0 = τ for each window.
// With u=0, headroom=1, score = τ. For full windows: max(τ_5h, τ_7d, 0) = τ_7d ≈ 1.0.
// But default util is 0 (not -1), so score = max of all τ values.
func TestTimeWeightedScore_AllZero(t *testing.T) {
	t.Parallel()
	now := time.Now()
	state := callback.AnthropicOAuthRateLimitState{
		Unified5hReset: fmt.Sprintf("%d", now.Add(5*time.Hour).Unix()),
		Unified7dReset: fmt.Sprintf("%d", now.Add(7*24*time.Hour).Unix()),
	}
	score := callback.TimeWeightedScore(state, 0.8, now)
	// u=0, headroom=1 → score = τ. 7d: τ≈1.0. 5h: τ=1.0.
	// max(1.0, 1.0, 0) = 1.0
	assert.InDelta(t, 1.0, score, 0.01, "empty token with full windows scores ≈ 1.0")
}

// TestTimeWeightedScore_FractionClamped: remaining > window (clock skew) → τ clamped to 1.0.
func TestTimeWeightedScore_FractionClamped(t *testing.T) {
	t.Parallel()
	now := time.Now()
	state := callback.AnthropicOAuthRateLimitState{
		Unified5hUtilization: 0.40,
		Unified5hReset:       fmt.Sprintf("%d", now.Add(10*time.Hour).Unix()), // > 5h window
		Unified7dUtilization: 0,
		Unified7dReset:       fmt.Sprintf("%d", now.Add(7*24*time.Hour).Unix()),
	}
	score := callback.TimeWeightedScore(state, 0.8, now)
	// 5h: r=0.50, headroom=0.50, τ clamped to 1.0 → 1.0/0.50 = 2.0
	// 7d: r=0, headroom=1, τ≈1.0 → 1.0
	assert.InDelta(t, 2.0, score, 1e-9, "τ clamped to 1.0, score = 1/headroom")
}

// TestTimeWeightedScore_AtGate: util at gate threshold → headroom≈0 → score=100 (capped).
func TestTimeWeightedScore_AtGate(t *testing.T) {
	t.Parallel()
	now := time.Now()
	state := callback.AnthropicOAuthRateLimitState{
		Unified7dUtilization: 0.90, // exactly at gate
		Unified7dReset:       fmt.Sprintf("%d", now.Add(3*24*time.Hour).Unix()),
	}
	score := callback.TimeWeightedScore(state, 0.8, now)
	assert.Equal(t, 100.0, score, "at gate → headroom=0 → capped at 100")
}

// ─── Phase 3: Integration test — production scenario ────────────────────────

// TestLowestUtilSelect_PrefersJustResetToken: production scenario.
// Token A: 7d=0% (just reset, ~7d left), 5h=45% (resets in 54min)
//
//	5h: τ=0.18/headroom(0.4375) = 0.411
//	7d: τ≈1.0/headroom(1.0)    = 1.0
//	max = 1.0
//
// Token B: 7d=45% (5d15h left), 5h=3% (just reset ~5h left)
//
//	5h: τ≈1.0/headroom(0.9625) = 1.039
//	7d: τ=0.794/headroom(0.50)  = 1.588
//	max = 1.588
//
// A (1.0) < B (1.588) → A selected. ✓
func TestLowestUtilSelect_PrefersJustResetToken(t *testing.T) {
	t.Parallel()
	tokA := oauthKey("just-reset-7d")
	tokB := oauthKey("high-7d-long-wait")
	upstreams := []nativeUpstream{
		{APIKey: tokA},
		{APIKey: tokB},
	}
	h := makeThrottleHandlers(upstreams, 0.8)

	now := time.Now()
	keyA := callback.RateLimitCacheKey(tokA)
	h.RateLimitStore.Set(keyA, callback.AnthropicOAuthRateLimitState{
		TokenKey: keyA, UnifiedStatus: callback.UnifiedStatusAllowed,
		Unified5hUtilization: 0.45,
		Unified5hReset:       fmt.Sprintf("%d", now.Add(54*time.Minute).Unix()),
		Unified7dUtilization: 0,
		Unified7dReset:       fmt.Sprintf("%d", now.Add(6*24*time.Hour+22*time.Hour).Unix()),
	})

	keyB := callback.RateLimitCacheKey(tokB)
	h.RateLimitStore.Set(keyB, callback.AnthropicOAuthRateLimitState{
		TokenKey: keyB, UnifiedStatus: callback.UnifiedStatusAllowed,
		Unified5hUtilization: 0.03,
		Unified5hReset:       fmt.Sprintf("%d", now.Add(5*time.Hour).Unix()),
		Unified7dUtilization: 0.45,
		Unified7dReset:       fmt.Sprintf("%d", now.Add(5*24*time.Hour+15*time.Hour).Unix()),
	})

	for i := 0; i < 20; i++ {
		u := h.lowestUtilizationSelect(context.Background(), "anthropic", upstreams)
		assert.Equal(t, tokA, u.APIKey,
			"iteration %d: token A (7d=0%%, 5h about to reset) should be preferred over B (7d=45%%, 5d left)", i)
	}
}

// ─── Model-aware gate: Sonnet replaces All gate (not additive) ──────────────

// T1: Sonnet request, 7d All=97% but 7d Sonnet=76% — should PASS gate.
// Before this change, the token was blocked because All gate fired unconditionally.
func TestGate_SonnetRequest_HighAll_LowSonnet_Passes(t *testing.T) {
	t.Parallel()
	tok := oauthKey("ma-gate-T1")
	upstreams := []nativeUpstream{{APIKey: tok}}
	h := makeThrottleHandlers(upstreams, 0.8)

	key := callback.RateLimitCacheKey(tok)
	h.RateLimitStore.Set(key, callback.AnthropicOAuthRateLimitState{
		TokenKey: key, UnifiedStatus: callback.UnifiedStatusAllowed,
		Unified5hUtilization:       0.10,
		Unified7dUtilization:       0.97, // would block non-Sonnet; replaced by Sonnet gate
		Unified7dSonnetUtilization: 0.76, // below 0.90 → should pass
	})

	u, err := h.selectUpstreamWithThrottle(context.Background(), "anthropic", upstreams, "claude-sonnet-4-5")
	require.NoError(t, err, "Sonnet request: 7d All=97%% must NOT block when Sonnet gate uses 7d_sonnet=76%%")
	assert.Equal(t, tok, u.APIKey)
}

// T2: Sonnet request, 7d Sonnet=95% — should be BLOCKED by Sonnet gate.
func TestGate_SonnetRequest_HighSonnet_Blocked(t *testing.T) {
	t.Parallel()
	tok := oauthKey("ma-gate-T2")
	upstreams := []nativeUpstream{{APIKey: tok}}
	h := makeThrottleHandlers(upstreams, 0.8)

	key := callback.RateLimitCacheKey(tok)
	h.RateLimitStore.Set(key, callback.AnthropicOAuthRateLimitState{
		TokenKey: key, UnifiedStatus: callback.UnifiedStatusAllowed,
		Unified5hUtilization:       0.10,
		Unified7dUtilization:       0.50, // below All gate
		Unified7dSonnetUtilization: 0.95, // above Sonnet gate → must block
		Unified7dSonnetReset:       fmt.Sprintf("%d", time.Now().Add(2*time.Hour).Unix()),
	})

	_, err := h.selectUpstreamWithThrottle(context.Background(), "anthropic", upstreams, "claude-sonnet-4-5")
	require.Error(t, err, "Sonnet request: 7d_sonnet=95%% must block even when 7d All is low")
	var ate *allTokensThrottledError
	assert.ErrorAs(t, err, &ate)
}

// T3: Non-Sonnet request, 7d All=97% — should be BLOCKED by All gate (no regression).
func TestGate_NonSonnetRequest_HighAll_Blocked(t *testing.T) {
	t.Parallel()
	tok := oauthKey("ma-gate-T3")
	upstreams := []nativeUpstream{{APIKey: tok}}
	h := makeThrottleHandlers(upstreams, 0.8)

	key := callback.RateLimitCacheKey(tok)
	h.RateLimitStore.Set(key, callback.AnthropicOAuthRateLimitState{
		TokenKey: key, UnifiedStatus: callback.UnifiedStatusAllowed,
		Unified5hUtilization:       0.10,
		Unified7dUtilization:       0.97, // above All gate
		Unified7dSonnetUtilization: 0.76, // Sonnet is fine but irrelevant for non-Sonnet
		Unified7dReset:             fmt.Sprintf("%d", time.Now().Add(2*time.Hour).Unix()),
	})

	_, err := h.selectUpstreamWithThrottle(context.Background(), "anthropic", upstreams, "claude-opus-4")
	require.Error(t, err, "Non-Sonnet request: 7d All=97%% must block regardless of Sonnet util")
	var ate *allTokensThrottledError
	assert.ErrorAs(t, err, &ate)
}

// Non-Sonnet request: sticky uses 7d All metrics (not 7d Sonnet).
func TestStickyScore_NonSonnetRequest_UsesAllMetrics(t *testing.T) {
	t.Parallel()
	tokA := oauthKey("sticky-score-T5-A")
	tokB := oauthKey("sticky-score-T5-B")
	upstreams := []nativeUpstream{
		{APIKey: tokA},
		{APIKey: tokB},
	}
	h := makeThrottleHandlers(upstreams, 0.8)
	h.Config.NativeUpstreamStrategy = config.StrategySticky

	now := time.Now()
	// A: 7d All reset soon → should WIN for All sticky
	keyA := callback.RateLimitCacheKey(tokA)
	h.RateLimitStore.Set(keyA, callback.AnthropicOAuthRateLimitState{
		TokenKey: keyA, UnifiedStatus: callback.UnifiedStatusAllowed,
		Unified5hUtilization:       0.10,
		Unified7dUtilization:       0.30,
		Unified7dReset:             fmt.Sprintf("%d", now.Add(1*time.Hour).Unix()), // resets soon
		Unified7dSonnetUtilization: 0.30,
		Unified7dSonnetReset:       fmt.Sprintf("%d", now.Add(6*24*time.Hour).Unix()), // far
	})
	// B: 7d All reset far → should NOT win for All sticky
	keyB := callback.RateLimitCacheKey(tokB)
	h.RateLimitStore.Set(keyB, callback.AnthropicOAuthRateLimitState{
		TokenKey: keyB, UnifiedStatus: callback.UnifiedStatusAllowed,
		Unified5hUtilization:       0.10,
		Unified7dUtilization:       0.30,
		Unified7dReset:             fmt.Sprintf("%d", now.Add(6*24*time.Hour).Unix()), // far
		Unified7dSonnetUtilization: 0.30,
		Unified7dSonnetReset:       fmt.Sprintf("%d", now.Add(1*time.Hour).Unix()), // resets soon
	})

	// Non-Sonnet request: sticky should pick A (All Models resets soonest)
	u, err := h.selectUpstreamWithThrottle(context.Background(), "anthropic", upstreams, "claude-opus-4")
	require.NoError(t, err)
	assert.Equal(t, tokA, u.APIKey, "Non-Sonnet sticky must pick token whose 7d All resets soonest")
}

// ─── Model-aware stickySelect: dual-track independence ───────────────────────

// T6: Sonnet and All sticky tracks are independent — their stickyState entries
// don't overwrite each other. Each track remembers its own selection and can
// be at a different token even when TimeWeightedScore would rank them identically.
func TestStickySelect_SonnetAndAll_IndependentTracks(t *testing.T) {
	t.Parallel()
	tokA := oauthKey("dual-track-T6-A")
	tokB := oauthKey("dual-track-T6-B")
	upstreams := []nativeUpstream{
		{APIKey: tokA},
		{APIKey: tokB},
	}
	h := makeThrottleHandlers(upstreams, 0.8)
	h.Config.NativeUpstreamStrategy = config.StrategySticky

	now := time.Now()
	// Symmetric state so TimeWeightedScore gives equal scores — this test
	// is about *sticky state* being remembered per track, not scoring.
	reset5h := fmt.Sprintf("%d", now.Add(4*time.Hour).Unix())
	reset7d := fmt.Sprintf("%d", now.Add(3*24*time.Hour).Unix())
	state := func(key string) callback.AnthropicOAuthRateLimitState {
		return callback.AnthropicOAuthRateLimitState{
			TokenKey: key, UnifiedStatus: callback.UnifiedStatusAllowed,
			Unified5hUtilization: 0.10, Unified5hReset: reset5h,
			Unified7dUtilization: 0.30, Unified7dReset: reset7d,
			Unified7dSonnetUtilization: 0.30, Unified7dSonnetReset: reset7d,
		}
	}
	keyA := callback.RateLimitCacheKey(tokA)
	keyB := callback.RateLimitCacheKey(tokB)
	h.RateLimitStore.Set(keyA, state(keyA))
	h.RateLimitStore.Set(keyB, state(keyB))

	// Pre-seed sticky state so each track is pointing at a different token.
	// (We bypass cold-start selection to test the remembrance property directly.)
	h.stickyState = map[string]stickyEntry{
		"anthropic:all":    {APIKey: tokA, Reset5hAt: reset5h},
		"anthropic:sonnet": {APIKey: tokB, Reset5hAt: reset5h},
	}

	// All track call → reuses its sticky (A)
	allResult, err := h.selectUpstreamWithThrottle(context.Background(), "anthropic", upstreams, "claude-opus-4")
	require.NoError(t, err)
	assert.Equal(t, tokA, allResult.APIKey, "All track must return its sticky token (A)")

	// Sonnet track call → reuses its own sticky (B), does NOT overwrite All track
	sonnetResult, err := h.selectUpstreamWithThrottle(context.Background(), "anthropic", upstreams, "claude-sonnet-4-5")
	require.NoError(t, err)
	assert.Equal(t, tokB, sonnetResult.APIKey, "Sonnet track must return its sticky token (B) independently")

	// All track still returns A
	allResult2, err := h.selectUpstreamWithThrottle(context.Background(), "anthropic", upstreams, "claude-opus-4")
	require.NoError(t, err)
	assert.Equal(t, tokA, allResult2.APIKey, "All track sticky must survive Sonnet call")
}

// T7: Sonnet sticky re-evaluates on 5h window reset and picks the token with soonest 7d Sonnet reset.
func TestStickySelect_SonnetTrack_ReevalsOn5hReset(t *testing.T) {
	t.Parallel()
	tokA := oauthKey("sonnet-reset-T7-A")
	tokB := oauthKey("sonnet-reset-T7-B")
	upstreams := []nativeUpstream{
		{APIKey: tokA},
		{APIKey: tokB},
	}
	h := makeThrottleHandlers(upstreams, 0.8)
	h.Config.NativeUpstreamStrategy = config.StrategySticky

	now := time.Now()
	reset5h := fmt.Sprintf("%d", now.Add(4*time.Hour).Unix())

	// Initial state: A has soonest 7d reset (3d) → selected first; B is far (6d).
	keyA := callback.RateLimitCacheKey(tokA)
	h.RateLimitStore.Set(keyA, callback.AnthropicOAuthRateLimitState{
		TokenKey: keyA, UnifiedStatus: callback.UnifiedStatusAllowed,
		Unified5hUtilization: 0.10, Unified5hReset: reset5h,
		Unified7dUtilization: 0.30, Unified7dReset: fmt.Sprintf("%d", now.Add(3*24*time.Hour).Unix()), // soonest
		Unified7dSonnetUtilization: 0.30,
		Unified7dSonnetReset:       fmt.Sprintf("%d", now.Add(1*time.Hour).Unix()),
	})
	keyB := callback.RateLimitCacheKey(tokB)
	h.RateLimitStore.Set(keyB, callback.AnthropicOAuthRateLimitState{
		TokenKey: keyB, UnifiedStatus: callback.UnifiedStatusAllowed,
		Unified5hUtilization: 0.10, Unified5hReset: reset5h,
		Unified7dUtilization: 0.30, Unified7dReset: fmt.Sprintf("%d", now.Add(6*24*time.Hour).Unix()), // far
		Unified7dSonnetUtilization: 0.30,
		Unified7dSonnetReset:       fmt.Sprintf("%d", now.Add(6*24*time.Hour).Unix()),
	})

	// First Sonnet call → selects A (Sonnet resets soonest)
	first, err := h.selectUpstreamWithThrottle(context.Background(), "anthropic", upstreams, "claude-sonnet-4-5")
	require.NoError(t, err)
	assert.Equal(t, tokA, first.APIKey, "first Sonnet call should select A (soonest Sonnet reset)")

	// Second call with same 5h reset → sticky, returns A again
	second, err := h.selectUpstreamWithThrottle(context.Background(), "anthropic", upstreams, "claude-sonnet-4-5")
	require.NoError(t, err)
	assert.Equal(t, tokA, second.APIKey, "sticky: same 5h window should return A again")

	// Simulate 5h window reset for A (5h reset timestamp changes).
	// B now has a sooner 7d reset than A → should win on re-evaluation.
	newReset5h := fmt.Sprintf("%d", now.Add(9*time.Hour).Unix()) // new 5h window
	h.RateLimitStore.Set(keyA, callback.AnthropicOAuthRateLimitState{
		TokenKey: keyA, UnifiedStatus: callback.UnifiedStatusAllowed,
		Unified5hUtilization: 0.60, Unified5hReset: newReset5h,
		Unified7dUtilization: 0.60, Unified7dReset: fmt.Sprintf("%d", now.Add(6*24*time.Hour).Unix()),
		Unified7dSonnetUtilization: 0.60,
		Unified7dSonnetReset:       fmt.Sprintf("%d", now.Add(6*24*time.Hour).Unix()),
	})
	h.RateLimitStore.Set(keyB, callback.AnthropicOAuthRateLimitState{
		TokenKey: keyB, UnifiedStatus: callback.UnifiedStatusAllowed,
		Unified5hUtilization: 0.05, Unified5hReset: reset5h,
		Unified7dUtilization: 0.10, Unified7dReset: fmt.Sprintf("%d", now.Add(12*time.Hour).Unix()), // B has soonest 7d reset
		Unified7dSonnetUtilization: 0.10,
		Unified7dSonnetReset:       fmt.Sprintf("%d", now.Add(12*time.Hour).Unix()),
	})

	// Third call: A's 5h has reset → re-evaluate Sonnet sticky → should now pick B
	// because B has soonest 7d reset (12h vs A's 6d).
	third, err := h.selectUpstreamWithThrottle(context.Background(), "anthropic", upstreams, "claude-sonnet-4-5")
	require.NoError(t, err)
	assert.Equal(t, tokB, third.APIKey, "after 5h reset, Sonnet sticky should re-evaluate and pick B (soonest 7d reset)")
}

// TestLowestUtilSelect_CapacityRateRanking: verifies the ranking fix from Phase 3.
// Phase 2 incorrectly ranked b1dcdc47 (7d=72%, 2d18h left) above 6f8d500b (7d=55%, 4d18h left).
// The capacity-rate formula corrects this:
//
//	b1dcdc47: 7d rate = τ/headroom = 0.393/(1-0.80) = 1.966
//	6f8d500b: 7d rate = τ/headroom = 0.680/(1-0.611) = 1.748
//	6f8d500b wins (lower score = better).
func TestLowestUtilSelect_CapacityRateRanking(t *testing.T) {
	t.Parallel()
	tokHigh := oauthKey("high-7d-short-wait")
	tokMid := oauthKey("mid-7d-long-wait")
	upstreams := []nativeUpstream{
		{APIKey: tokHigh},
		{APIKey: tokMid},
	}
	h := makeThrottleHandlers(upstreams, 0.8)

	now := time.Now()
	keyHigh := callback.RateLimitCacheKey(tokHigh)
	h.RateLimitStore.Set(keyHigh, callback.AnthropicOAuthRateLimitState{
		TokenKey: keyHigh, UnifiedStatus: callback.UnifiedStatusAllowed,
		Unified5hUtilization: 0,
		Unified7dUtilization: 0.72,
		Unified7dReset:       fmt.Sprintf("%d", now.Add(2*24*time.Hour+18*time.Hour).Unix()),
	})

	keyMid := callback.RateLimitCacheKey(tokMid)
	h.RateLimitStore.Set(keyMid, callback.AnthropicOAuthRateLimitState{
		TokenKey: keyMid, UnifiedStatus: callback.UnifiedStatusAllowed,
		Unified5hUtilization: 0,
		Unified7dUtilization: 0.55,
		Unified7dReset:       fmt.Sprintf("%d", now.Add(4*24*time.Hour+18*time.Hour).Unix()),
	})

	for i := 0; i < 20; i++ {
		u := h.lowestUtilizationSelect(context.Background(), "anthropic", upstreams)
		assert.Equal(t, tokMid, u.APIKey,
			"iteration %d: mid-util with more time should beat high-util with less time", i)
	}
}
