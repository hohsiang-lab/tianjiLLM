package callback

// HO-82: Tests for ParseAnthropicOAuthRateLimitHeaders when unified headers are absent.
// Root cause A: Anthropic does not return unified OAuth rate limit headers on every response.
// When absent, utilization sentinel must be -1 (not 0 or any other value).

import (
	"net/http"
	"testing"
)

// TestParseAnthropicOAuthRateLimitHeaders_NoUnifiedHeaders verifies that when no unified
// OAuth headers are present, all utilization fields are -1 (sentinel = "unknown").
func TestParseAnthropicOAuthRateLimitHeaders_NoUnifiedHeaders(t *testing.T) {
	// Only legacy headers present, no unified OAuth headers.
	h := http.Header{}
	h.Set("anthropic-ratelimit-requests-limit", "1000")
	h.Set("anthropic-ratelimit-requests-remaining", "900")

	state := ParseAnthropicOAuthRateLimitHeaders(h, "keyonly")

	if state.Unified5hUtilization != -1 {
		t.Errorf("Unified5hUtilization = %v, want -1 when header absent", state.Unified5hUtilization)
	}
	if state.Unified7dUtilization != -1 {
		t.Errorf("Unified7dUtilization = %v, want -1 when header absent", state.Unified7dUtilization)
	}
	if state.FallbackPercentage != -1 {
		t.Errorf("FallbackPercentage = %v, want -1 when header absent", state.FallbackPercentage)
	}
	if state.UnifiedStatus != "" {
		t.Errorf("UnifiedStatus = %q, want empty string when header absent", state.UnifiedStatus)
	}
	if state.Unified5hStatus != "" {
		t.Errorf("Unified5hStatus = %q, want empty string when header absent", state.Unified5hStatus)
	}
	if state.Unified7dStatus != "" {
		t.Errorf("Unified7dStatus = %q, want empty string when header absent", state.Unified7dStatus)
	}
	if state.OverageDisabledReason != "" {
		t.Errorf("OverageDisabledReason = %q, want empty string when header absent", state.OverageDisabledReason)
	}
}

// TestParseAnthropicOAuthRateLimitHeaders_EmptyHeaders verifies sentinel values
// when the response has no rate limit headers at all (e.g., non-Anthropic proxy response).
func TestParseAnthropicOAuthRateLimitHeaders_EmptyHeaders(t *testing.T) {
	h := http.Header{}

	state := ParseAnthropicOAuthRateLimitHeaders(h, "emptykey")

	if state.Unified5hUtilization != -1 {
		t.Errorf("Unified5hUtilization = %v, want -1 for empty headers", state.Unified5hUtilization)
	}
	if state.Unified7dUtilization != -1 {
		t.Errorf("Unified7dUtilization = %v, want -1 for empty headers", state.Unified7dUtilization)
	}
	if state.RequestsLimit != -1 {
		t.Errorf("RequestsLimit = %v, want -1 for empty headers", state.RequestsLimit)
	}
	if state.TokensLimit != -1 {
		t.Errorf("TokensLimit = %v, want -1 for empty headers", state.TokensLimit)
	}
	// TokenKey must be preserved
	if state.TokenKey != "emptykey" {
		t.Errorf("TokenKey = %q, want %q", state.TokenKey, "emptykey")
	}
}

// TestParseAnthropicOAuthRateLimitHeaders_OverageDisabledReason_Empty verifies that
// when overage-disabled-reason header is absent, OverageDisabledReason is "" (empty string).
// This is critical: the UI must render "— unknown" for empty string, not "⚠️ disabled".
func TestParseAnthropicOAuthRateLimitHeaders_OverageDisabledReason_Empty(t *testing.T) {
	h := http.Header{}
	h.Set("anthropic-ratelimit-unified-status", "allowed")
	// No overage-disabled-reason header

	state := ParseAnthropicOAuthRateLimitHeaders(h, "testkey")

	if state.OverageDisabledReason != "" {
		t.Errorf("OverageDisabledReason = %q, want empty string when header absent", state.OverageDisabledReason)
	}
}

// TestParseAnthropicOAuthRateLimitHeaders_OverageDisabledReason_Disabled verifies
// that "disabled" value in header is preserved exactly.
func TestParseAnthropicOAuthRateLimitHeaders_OverageDisabledReason_Disabled(t *testing.T) {
	h := http.Header{}
	h.Set("anthropic-ratelimit-unified-overage-disabled-reason", "disabled")

	state := ParseAnthropicOAuthRateLimitHeaders(h, "testkey")

	if state.OverageDisabledReason != "disabled" {
		t.Errorf("OverageDisabledReason = %q, want %q", state.OverageDisabledReason, "disabled")
	}
}
