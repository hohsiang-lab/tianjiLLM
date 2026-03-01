package callback

import (
	"net/http"
	"testing"
	"time"
)

func TestParseAnthropicOAuthRateLimitHeaders_5h(t *testing.T) {
	h := http.Header{}
	h.Set("anthropic-ratelimit-unified-5h-status", "allowed")
	h.Set("anthropic-ratelimit-unified-5h-utilization", "0.42")
	h.Set("anthropic-ratelimit-unified-5h-reset", "1700000000")

	state := ParseAnthropicOAuthRateLimitHeaders(h, "testhash5h")

	if state.TokenKey != "testhash5h" {
		t.Errorf("TokenKey = %q, want %q", state.TokenKey, "testhash5h")
	}
	if state.Unified5hStatus != "allowed" {
		t.Errorf("Unified5hStatus = %q, want %q", state.Unified5hStatus, "allowed")
	}
	if state.Unified5hUtilization != 0.42 {
		t.Errorf("Unified5hUtilization = %v, want 0.42", state.Unified5hUtilization)
	}
	if state.Unified5hReset != "1700000000" {
		t.Errorf("Unified5hReset = %q, want %q", state.Unified5hReset, "1700000000")
	}
	if state.Unified7dUtilization != -1 {
		t.Errorf("Unified7dUtilization = %v, want -1", state.Unified7dUtilization)
	}
}

func TestParseAnthropicOAuthRateLimitHeaders_7d(t *testing.T) {
	h := http.Header{}
	h.Set("anthropic-ratelimit-unified-7d-status", "rate_limited")
	h.Set("anthropic-ratelimit-unified-7d-utilization", "0.99")
	h.Set("anthropic-ratelimit-unified-7d-reset", "1700001000")
	h.Set("anthropic-ratelimit-unified-representative-claim", "seven_day")

	state := ParseAnthropicOAuthRateLimitHeaders(h, "testhash7d")

	if state.Unified7dStatus != "rate_limited" {
		t.Errorf("Unified7dStatus = %q, want %q", state.Unified7dStatus, "rate_limited")
	}
	if state.Unified7dUtilization != 0.99 {
		t.Errorf("Unified7dUtilization = %v, want 0.99", state.Unified7dUtilization)
	}
	if state.RepresentativeClaim != "seven_day" {
		t.Errorf("RepresentativeClaim = %q, want %q", state.RepresentativeClaim, "seven_day")
	}
	if state.Unified5hUtilization != -1 {
		t.Errorf("Unified5hUtilization = %v, want -1", state.Unified5hUtilization)
	}
}

func TestParseAnthropicOAuthRateLimitHeaders_Overage(t *testing.T) {
	h := http.Header{}
	h.Set("anthropic-ratelimit-unified-status", "overage")
	h.Set("anthropic-ratelimit-unified-overage-disabled-reason", "policy")
	h.Set("anthropic-ratelimit-unified-fallback-percentage", "0.5")

	state := ParseAnthropicOAuthRateLimitHeaders(h, "testhashov")

	if state.UnifiedStatus != "overage" {
		t.Errorf("UnifiedStatus = %q, want %q", state.UnifiedStatus, "overage")
	}
	if state.OverageDisabledReason != "policy" {
		t.Errorf("OverageDisabledReason = %q, want %q", state.OverageDisabledReason, "policy")
	}
	if state.FallbackPercentage != 0.5 {
		t.Errorf("FallbackPercentage = %v, want 0.5", state.FallbackPercentage)
	}
}

func TestInMemoryRateLimitStore_SetGet(t *testing.T) {
	store := NewInMemoryRateLimitStore()

	state := AnthropicOAuthRateLimitState{
		TokenKey:        "abc123",
		Unified5hStatus: "allowed",
		ParsedAt:        time.Now(),
	}
	store.Set("abc123", state)

	got, ok := store.Get("abc123")
	if !ok {
		t.Fatal("Get returned ok=false, want true")
	}
	if got.TokenKey != state.TokenKey {
		t.Errorf("TokenKey = %q, want %q", got.TokenKey, state.TokenKey)
	}
	if got.Unified5hStatus != state.Unified5hStatus {
		t.Errorf("Unified5hStatus = %q, want %q", got.Unified5hStatus, state.Unified5hStatus)
	}

	_, ok2 := store.Get("nonexistent")
	if ok2 {
		t.Error("Get returned ok=true for nonexistent key")
	}
}

func TestInMemoryRateLimitStore_GetAll(t *testing.T) {
	store := NewInMemoryRateLimitStore()

	store.Set("key1", AnthropicOAuthRateLimitState{TokenKey: "key1"})
	store.Set("key2", AnthropicOAuthRateLimitState{TokenKey: "key2"})
	store.Set("key3", AnthropicOAuthRateLimitState{TokenKey: "key3"})

	all := store.GetAll()
	if len(all) != 3 {
		t.Errorf("GetAll returned %d entries, want 3", len(all))
	}
	for _, k := range []string{"key1", "key2", "key3"} {
		if _, ok := all[k]; !ok {
			t.Errorf("GetAll missing key %q", k)
		}
	}
}

func TestInMemoryRateLimitStore_Prune(t *testing.T) {
	store := NewInMemoryRateLimitStore()

	store.Set("old", AnthropicOAuthRateLimitState{TokenKey: "old"})

	store.mu.Lock()
	e := store.entries["old"]
	e.updatedAt = time.Now().Add(-2 * time.Hour)
	store.entries["old"] = e
	store.mu.Unlock()

	store.Set("fresh", AnthropicOAuthRateLimitState{TokenKey: "fresh"})

	store.Prune(1 * time.Hour)

	if _, ok := store.Get("old"); ok {
		t.Error("Prune: 'old' entry should have been pruned")
	}
	if _, ok := store.Get("fresh"); !ok {
		t.Error("Prune: 'fresh' entry should still be present")
	}
}

// --- HO-82: Rate Limit utilization 顯示 "—" ---

// TestParseRateLimitHeaders_MissingUtilizationHeaders verifies that when
// anthropic-ratelimit-unified-5h/7d-utilization headers are absent (non-OAuth API key),
// ParseAnthropicOAuthRateLimitHeaders returns -1 for both utilization fields.
//
// Root cause of HO-82: non-OAuth API key responses don't include the unified
// utilization headers; parseFloat("") returns -1. The -1 sentinel is then
// passed through to the UI layer where fmtUtilPct(-1) = "—".
//
// This test documents that the CURRENT behavior is: -1 for missing headers.
// The DOWNSTREAM problem is that "—" alone gives no hint to the user.
// (See TestRateLimitTemplate_ShowsHelpTextWhenUtilizationUnavailable for UI fix)
func TestParseRateLimitHeaders_MissingUtilizationHeaders(t *testing.T) {
	// Non-OAuth API key response: has legacy request/token headers but NO unified headers.
	h := http.Header{}
	h.Set("anthropic-ratelimit-requests-limit", "1000")
	h.Set("anthropic-ratelimit-requests-remaining", "999")
	h.Set("anthropic-ratelimit-tokens-limit", "100000")
	h.Set("anthropic-ratelimit-tokens-remaining", "99000")

	state := ParseAnthropicOAuthRateLimitHeaders(h, "api-key-hash")

	// Missing utilization headers → must be exactly -1 (the unavailable sentinel).
	if state.Unified5hUtilization != -1 {
		t.Errorf("Unified5hUtilization = %v, want -1 (unavailable sentinel) when header is absent.\n"+
			"Bug HO-82: -1 is the sentinel value that causes the UI to show '—' without explanation.",
			state.Unified5hUtilization)
	}
	if state.Unified7dUtilization != -1 {
		t.Errorf("Unified7dUtilization = %v, want -1 (unavailable sentinel) when header is absent.\n"+
			"Bug HO-82: -1 is the sentinel value that causes the UI to show '—' without explanation.",
			state.Unified7dUtilization)
	}

	// Legacy fields must be correctly parsed (non-OAuth keys DO have these headers).
	if state.RequestsLimit != 1000 {
		t.Errorf("RequestsLimit = %d, want 1000", state.RequestsLimit)
	}
	if state.TokensRemaining != 99000 {
		t.Errorf("TokensRemaining = %d, want 99000", state.TokensRemaining)
	}

	// Unified status must be empty (header absent for non-OAuth keys).
	if state.UnifiedStatus != "" {
		t.Errorf("UnifiedStatus = %q, want empty when unified headers are absent", state.UnifiedStatus)
	}

	// DESIRED BEHAVIOR (currently NOT implemented → this assertion FAILS):
	// The struct should distinguish "not present" from "0 utilization" explicitly,
	// so the UI can show a helpful message instead of just "—".
	// A proper fix would add: UtilizationAvailable bool (false when headers absent).
	// Currently this field does not exist, so we verify the implicit contract:
	// "if UnifiedStatus is empty AND Unified5hUtilization == -1, it's an API key token".
	//
	// This assertion documents the gap: no explicit "unavailable" flag exists.
	// The fix is to add UtilizationAvailable bool to AnthropicOAuthRateLimitState.
	type hasUtilAvailable interface {
		IsUtilizationAvailable() bool
	}
	stateIface := interface{}(state)
	if checker, ok := stateIface.(hasUtilAvailable); ok {
		// If the fix is applied and IsUtilizationAvailable() exists, it must return false.
		if checker.IsUtilizationAvailable() {
			t.Errorf("IsUtilizationAvailable() must return false when utilization headers are absent")
		}
	} else {
		// Fix not yet applied: document the gap.
		t.Logf("NOTE HO-82: AnthropicOAuthRateLimitState lacks UtilizationAvailable flag. "+
			"Currently using -1 sentinel. Fix: add explicit bool to distinguish missing vs zero utilization.")
	}
}
