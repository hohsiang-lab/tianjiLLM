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
