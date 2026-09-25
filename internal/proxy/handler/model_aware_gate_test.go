package handler

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/praxisllmlab/tianjiLLM/internal/callback"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sonnetGateState builds a single-token throttle test fixture with configurable
// 7d and 7d_sonnet utilization. Fills in sensible defaults for other fields.
func sonnetGateState(tokenID string, u7d, u7dSonnet float64) ([]nativeUpstream, *Handlers) {
	raw := oauthKey(tokenID)
	upstreams := []nativeUpstream{{BaseURL: "https://api.anthropic.com", APIKey: raw}}
	h := makeThrottleHandlers(upstreams, 0.8)

	key := callback.RateLimitCacheKey(raw)
	h.RateLimitStore.Set(key, callback.AnthropicOAuthRateLimitState{
		TokenKey:                   key,
		UnifiedStatus:              callback.UnifiedStatusAllowed,
		Unified5hUtilization:       0.3,
		Unified7dUtilization:       u7d,
		Unified7dSonnetUtilization: u7dSonnet,
		Unified5hReset:             fmt.Sprintf("%d", time.Now().Add(time.Hour).Unix()),
		Unified7dReset:             fmt.Sprintf("%d", time.Now().Add(5*24*time.Hour).Unix()),
		Unified7dSonnetReset:       fmt.Sprintf("%d", time.Now().Add(2*24*time.Hour).Unix()),
	})
	return upstreams, h
}

func TestSelectUpstreamThrottle_OpusRequest_SonnetGateIgnored(t *testing.T) {
	t.Parallel()
	upstreams, h := sonnetGateState("OPUS-SGI", 0.5, 0.95)

	u, err := h.selectUpstreamWithThrottle(context.Background(), "anthropic", upstreams, "claude-opus-4-6")
	require.NoError(t, err, "Opus request must not be blocked by 7d_sonnet gate")
	assert.Equal(t, oauthKey("OPUS-SGI"), u.APIKey)
}

func TestSelectUpstreamThrottle_SonnetRequest_SonnetGateApplied(t *testing.T) {
	t.Parallel()
	upstreams, h := sonnetGateState("SON-SGA", 0.5, 0.95)

	_, err := h.selectUpstreamWithThrottle(context.Background(), "anthropic", upstreams, "claude-sonnet-4-6")
	require.Error(t, err, "Sonnet request must be blocked when 7d_sonnet exceeds gate")
	var ate *allTokensThrottledError
	assert.ErrorAs(t, err, &ate)
}

// Sonnet request: 7d All=95% does NOT block because Sonnet gate replaces All gate.
// Only 7d Sonnet=0.4 is checked, which is below 0.9 → token passes.
func TestSelectUpstreamThrottle_SonnetRequest_7dAllHighButSonnetLow_Passes(t *testing.T) {
	t.Parallel()
	upstreams, h := sonnetGateState("SON-7DA", 0.95, 0.4)

	u, err := h.selectUpstreamWithThrottle(context.Background(), "anthropic", upstreams, "claude-sonnet-4-6")
	require.NoError(t, err, "Sonnet request: 7d All=95%% must NOT block when Sonnet gate uses 7d_sonnet=40%%")
	assert.Equal(t, oauthKey("SON-7DA"), u.APIKey)
}

// Empty model name uses All gate (non-Sonnet path): 7d All=50% < 0.9 → passes.
// 7d Sonnet=95% is irrelevant for non-Sonnet requests.
func TestSelectUpstreamThrottle_EmptyModel_UsesAllGate(t *testing.T) {
	t.Parallel()
	upstreams, h := sonnetGateState("EMPTY-M", 0.5, 0.95)

	u, err := h.selectUpstreamWithThrottle(context.Background(), "anthropic", upstreams, "")
	require.NoError(t, err, "empty model name uses All gate only; 7d_sonnet=95%% must not block")
	assert.Equal(t, oauthKey("EMPTY-M"), u.APIKey)
}

// Two tokens: A has 7d_sonnet over gate, B has 7d over gate.
// For Opus, the sonnet-gated token should still be available.
func TestSelectUpstreamThrottle_OpusRequest_PrefersSonnetGatedToken(t *testing.T) {
	t.Parallel()
	rawA, rawB := oauthKey("PREF-A"), oauthKey("PREF-B")
	upstreams := []nativeUpstream{
		{BaseURL: "https://api.anthropic.com", APIKey: rawA},
		{BaseURL: "https://api.anthropic.com", APIKey: rawB},
	}
	h := makeThrottleHandlers(upstreams, 0.8)

	keyA := callback.RateLimitCacheKey(rawA)
	keyB := callback.RateLimitCacheKey(rawB)

	h.RateLimitStore.Set(keyA, callback.AnthropicOAuthRateLimitState{
		TokenKey:                   keyA,
		UnifiedStatus:              callback.UnifiedStatusAllowed,
		Unified5hUtilization:       0.3,
		Unified7dUtilization:       0.5,
		Unified7dSonnetUtilization: 0.95,
		Unified5hReset:             fmt.Sprintf("%d", time.Now().Add(time.Hour).Unix()),
		Unified7dReset:             fmt.Sprintf("%d", time.Now().Add(5*24*time.Hour).Unix()),
	})
	h.RateLimitStore.Set(keyB, callback.AnthropicOAuthRateLimitState{
		TokenKey:                   keyB,
		UnifiedStatus:              callback.UnifiedStatusAllowed,
		Unified5hUtilization:       0.3,
		Unified7dUtilization:       0.92,
		Unified7dSonnetUtilization: 0.3,
		Unified5hReset:             fmt.Sprintf("%d", time.Now().Add(time.Hour).Unix()),
		Unified7dReset:             fmt.Sprintf("%d", time.Now().Add(5*24*time.Hour).Unix()),
	})

	u, err := h.selectUpstreamWithThrottle(context.Background(), "anthropic", upstreams, "claude-opus-4-6")
	require.NoError(t, err, "Opus: A should be available (sonnet gate ignored), B blocked by 7d gate")
	assert.Equal(t, rawA, u.APIKey, "A is the only available token for Opus requests")
}

func TestIsSonnetModel(t *testing.T) {
	t.Parallel()
	assert.True(t, isSonnetModel("claude-sonnet-4-6"))
	assert.True(t, isSonnetModel("claude-sonnet-4-5-20250929"))
	assert.True(t, isSonnetModel("claude-3-5-sonnet-20241022"))
	assert.True(t, isSonnetModel("claude-sonnet-4-6-1m"))
	assert.False(t, isSonnetModel("claude-opus-4-6"))
	assert.False(t, isSonnetModel("claude-haiku-4-5-20251001"))
	assert.False(t, isSonnetModel(""))
}
