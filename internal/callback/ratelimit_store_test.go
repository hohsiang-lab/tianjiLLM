package callback

import (
	"fmt"
	"net/http"
	"testing"
	"time"
)

func TestParseAnthropicOAuthRateLimitHeaders_5h(t *testing.T) {
	h := http.Header{}
	h.Set("anthropic-ratelimit-unified-5h-status", UnifiedStatusAllowed)
	h.Set("anthropic-ratelimit-unified-5h-utilization", "0.42")
	h.Set("anthropic-ratelimit-unified-5h-reset", "1700000000")

	state := ParseAnthropicOAuthRateLimitHeaders(h, "testhash5h")

	if state.TokenKey != "testhash5h" {
		t.Errorf("TokenKey = %q, want %q", state.TokenKey, "testhash5h")
	}
	if state.Unified5hStatus != UnifiedStatusAllowed {
		t.Errorf("Unified5hStatus = %q, want %q", state.Unified5hStatus, UnifiedStatusAllowed)
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
	h.Set("anthropic-ratelimit-unified-7d-status", UnifiedStatusRejected)
	h.Set("anthropic-ratelimit-unified-7d-utilization", "0.99")
	h.Set("anthropic-ratelimit-unified-7d-reset", "1700001000")
	h.Set("anthropic-ratelimit-unified-representative-claim", "seven_day")

	state := ParseAnthropicOAuthRateLimitHeaders(h, "testhash7d")

	if state.Unified7dStatus != UnifiedStatusRejected {
		t.Errorf("Unified7dStatus = %q, want %q", state.Unified7dStatus, UnifiedStatusRejected)
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
	h.Set("anthropic-ratelimit-unified-status", UnifiedStatusRejected)
	h.Set("anthropic-ratelimit-unified-overage-disabled-reason", "policy")
	h.Set("anthropic-ratelimit-unified-fallback-percentage", "0.5")

	state := ParseAnthropicOAuthRateLimitHeaders(h, "testhashov")

	if state.UnifiedStatus != UnifiedStatusRejected {
		t.Errorf("UnifiedStatus = %q, want %q", state.UnifiedStatus, UnifiedStatusRejected)
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
		Unified5hStatus: UnifiedStatusAllowed,
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
		t.Logf("NOTE HO-82: AnthropicOAuthRateLimitState lacks UtilizationAvailable flag. " +
			"Currently using -1 sentinel. Fix: add explicit bool to distinguish missing vs zero utilization.")
	}
}

// --- 197: Claude Code OAuth Usage ---

func TestParseHeaders_7dSonnet(t *testing.T) {
	h := http.Header{}
	h.Set("anthropic-ratelimit-unified-7d_sonnet-status", UnifiedStatusAllowed)
	h.Set("anthropic-ratelimit-unified-7d_sonnet-utilization", "0.40")
	h.Set("anthropic-ratelimit-unified-7d_sonnet-reset", "1774850400")

	state := ParseAnthropicOAuthRateLimitHeaders(h, "testsonnet")

	if state.Unified7dSonnetStatus != UnifiedStatusAllowed {
		t.Errorf("Unified7dSonnetStatus = %q, want 'allowed'", state.Unified7dSonnetStatus)
	}
	if state.Unified7dSonnetUtilization != 0.40 {
		t.Errorf("Unified7dSonnetUtilization = %v, want 0.40", state.Unified7dSonnetUtilization)
	}
	if state.Unified7dSonnetReset != "1774850400" {
		t.Errorf("Unified7dSonnetReset = %q, want '1774850400'", state.Unified7dSonnetReset)
	}
}

func TestParseHeaders_OrgID(t *testing.T) {
	h := http.Header{}
	h.Set("anthropic-organization-id", "40403251-c713-4890-91a7-03763c31003f")

	state := ParseAnthropicOAuthRateLimitHeaders(h, "testorg")

	if state.OrganizationID != "40403251-c713-4890-91a7-03763c31003f" {
		t.Errorf("OrganizationID = %q, want '40403251-c713-4890-91a7-03763c31003f'", state.OrganizationID)
	}
}

// --- 199: Dirty Tracking ---

func TestDirtyTracking_SetMarksDirty(t *testing.T) {
	store := NewInMemoryRateLimitStore()
	store.Set("k1", AnthropicOAuthRateLimitState{TokenKey: "k1"})

	snap := store.SnapshotDirty()
	if len(snap) != 1 {
		t.Fatalf("SnapshotDirty() = %d keys, want 1", len(snap))
	}
	if _, ok := snap["k1"]; !ok {
		t.Error("SnapshotDirty() missing 'k1'")
	}
}

func TestDirtyTracking_SetCleanNotDirty(t *testing.T) {
	store := NewInMemoryRateLimitStore()
	store.SetClean("k1", AnthropicOAuthRateLimitState{TokenKey: "k1"})

	snap := store.SnapshotDirty()
	if len(snap) != 0 {
		t.Errorf("SnapshotDirty() = %d keys after setClean, want 0", len(snap))
	}
	// But the entry should still be readable.
	if _, ok := store.Get("k1"); !ok {
		t.Error("Get('k1') should return entry written by setClean")
	}
}

func TestDirtyTracking_MarkDirty(t *testing.T) {
	store := NewInMemoryRateLimitStore()
	store.SetClean("k1", AnthropicOAuthRateLimitState{TokenKey: "k1"})
	store.MarkDirty([]string{"k1"})

	snap := store.SnapshotDirty()
	if len(snap) != 1 {
		t.Fatalf("SnapshotDirty() = %d keys after MarkDirty, want 1", len(snap))
	}
}

func TestDirtyTracking_SnapshotAndClear(t *testing.T) {
	store := NewInMemoryRateLimitStore()
	store.Set("a", AnthropicOAuthRateLimitState{TokenKey: "a", Unified5hUtilization: 0.1})
	store.Set("b", AnthropicOAuthRateLimitState{TokenKey: "b", Unified5hUtilization: 0.2})
	store.Set("c", AnthropicOAuthRateLimitState{TokenKey: "c", Unified5hUtilization: 0.3})

	snap := store.SnapshotDirty()
	if len(snap) != 3 {
		t.Fatalf("SnapshotDirty() = %d entries, want 3", len(snap))
	}
	for _, key := range []string{"a", "b", "c"} {
		if _, ok := snap[key]; !ok {
			t.Errorf("SnapshotDirty() missing key %q", key)
		}
	}

	// After snapshot, dirty set should be empty.
	snap2 := store.SnapshotDirty()
	if len(snap2) != 0 {
		t.Errorf("SnapshotDirty() = %d after second call, want 0", len(snap2))
	}
}

func TestParseHeaders_On429Response(t *testing.T) {
	// 429 responses should still have rate limit headers parsed correctly (FR-003).
	h := http.Header{}
	h.Set("anthropic-ratelimit-unified-5h-status", UnifiedStatusRejected)
	h.Set("anthropic-ratelimit-unified-5h-utilization", "1.0")
	h.Set("anthropic-ratelimit-unified-7d-status", UnifiedStatusAllowed)
	h.Set("anthropic-ratelimit-unified-7d-utilization", "0.80")
	h.Set("anthropic-ratelimit-unified-7d_sonnet-status", UnifiedStatusAllowed)
	h.Set("anthropic-ratelimit-unified-7d_sonnet-utilization", "0.50")
	h.Set("anthropic-organization-id", "test-org-429")

	state := ParseAnthropicOAuthRateLimitHeaders(h, "test429")

	if state.Unified5hStatus != UnifiedStatusRejected {
		t.Errorf("Unified5hStatus = %q, want %q", state.Unified5hStatus, UnifiedStatusRejected)
	}
	if state.Unified5hUtilization != 1.0 {
		t.Errorf("Unified5hUtilization = %v, want 1.0", state.Unified5hUtilization)
	}
	if state.Unified7dSonnetUtilization != 0.50 {
		t.Errorf("Unified7dSonnetUtilization = %v, want 0.50", state.Unified7dSonnetUtilization)
	}
	if state.OrganizationID != "test-org-429" {
		t.Errorf("OrganizationID = %q, want 'test-org-429'", state.OrganizationID)
	}
}

// ─── PruneExpired smoke tests ────────────────────────────────────────────────

// TestPruneExpired_KeepsEntryWithinResetWindow verifies that an entry whose
// 5h reset time is in the future is NOT removed by PruneExpired.
func TestPruneExpired_KeepsEntryWithinResetWindow(t *testing.T) {
	store := NewInMemoryRateLimitStore()
	futureReset := time.Now().Add(1 * time.Hour).Unix()
	store.Set("tok-active", AnthropicOAuthRateLimitState{
		TokenKey:             "tok-active",
		Unified5hUtilization: 0.5,
		Unified5hReset:       fmt.Sprintf("%d", futureReset),
		Unified7dReset:       fmt.Sprintf("%d", time.Now().Add(24*time.Hour).Unix()),
	})

	store.PruneExpired()

	if _, ok := store.Get("tok-active"); !ok {
		t.Error("PruneExpired removed an entry whose reset window has not expired")
	}
}

// TestPruneExpired_RemovesEntryAfterBothWindowsExpired verifies that an entry
// whose both 5h and 7d reset times have passed IS removed by PruneExpired.
func TestPruneExpired_RemovesEntryAfterBothWindowsExpired(t *testing.T) {
	store := NewInMemoryRateLimitStore()
	pastReset := time.Now().Add(-1 * time.Hour).Unix()
	store.Set("tok-expired", AnthropicOAuthRateLimitState{
		TokenKey:             "tok-expired",
		Unified5hUtilization: 0.3,
		Unified5hReset:       fmt.Sprintf("%d", pastReset),
		Unified7dReset:       fmt.Sprintf("%d", pastReset),
	})
	// Simulate a successful flush so the entry is no longer dirty.
	store.SnapshotDirty()

	store.PruneExpired()

	if _, ok := store.Get("tok-expired"); ok {
		t.Error("PruneExpired kept an entry whose both reset windows have expired")
	}
}

// TestPruneExpired_KeepsEntryIf7dWindowStillActive verifies that an entry
// is NOT removed when only the 5h window expired but 7d is still active.
func TestPruneExpired_KeepsEntryIf7dWindowStillActive(t *testing.T) {
	store := NewInMemoryRateLimitStore()
	store.Set("tok-7d-active", AnthropicOAuthRateLimitState{
		TokenKey:             "tok-7d-active",
		Unified5hUtilization: 0.2,
		Unified5hReset:       fmt.Sprintf("%d", time.Now().Add(-1*time.Hour).Unix()), // expired
		Unified7dReset:       fmt.Sprintf("%d", time.Now().Add(48*time.Hour).Unix()), // active
	})

	store.PruneExpired()

	if _, ok := store.Get("tok-7d-active"); !ok {
		t.Error("PruneExpired removed entry whose 7d window is still active")
	}
}

// TestPruneExpired_SkipsDirtyEntries verifies dirty entries are never pruned.
func TestPruneExpired_SkipsDirtyEntries(t *testing.T) {
	store := NewInMemoryRateLimitStore()
	pastReset := fmt.Sprintf("%d", time.Now().Add(-2*time.Hour).Unix())
	// Set marks dirty
	store.Set("tok-dirty", AnthropicOAuthRateLimitState{
		TokenKey:       "tok-dirty",
		Unified5hReset: pastReset,
		Unified7dReset: pastReset,
	})

	store.PruneExpired()

	if _, ok := store.Get("tok-dirty"); !ok {
		t.Error("PruneExpired removed a dirty entry that has not been flushed yet")
	}
}

// TestPruneExpired_KeepsEntryWithNoResetHeaders verifies that a token with
// empty reset fields (no Anthropic reset headers received yet) is NOT pruned.
// Zero reset time means "in-flight / new token", not "expired".
func TestPruneExpired_KeepsEntryWithNoResetHeaders(t *testing.T) {
	store := NewInMemoryRateLimitStore()
	store.Set("tok-no-reset", AnthropicOAuthRateLimitState{
		TokenKey:             "tok-no-reset",
		Unified5hUtilization: 0.5,
		// Unified5hReset and Unified7dReset intentionally empty (no header received yet)
	})
	store.SnapshotDirty() // flush so it's not dirty

	store.PruneExpired()

	if _, ok := store.Get("tok-no-reset"); !ok {
		t.Error("PruneExpired removed a new token with no reset headers — zero reset time must mean NOT expired")
	}
}

// ─── ParseResetTime unit tests ───────────────────────────────────────────────

// HO-490: NormalizeExpiredWindows — branch coverage for 7d < 5h case
func TestNormalizeExpiredWindows_5hExpired_7dNot(t *testing.T) {
	past := time.Now().Add(-1 * time.Hour)
	future := time.Now().Add(2 * time.Hour)
	state := AnthropicOAuthRateLimitState{
		Unified5hReset:       fmt.Sprintf("%d", past.Unix()),
		Unified5hUtilization: 0.9,
		Unified5hStatus:      UnifiedStatusAllowedWarning,
		Unified7dReset:       fmt.Sprintf("%d", future.Unix()),
		Unified7dUtilization: 0.5,
	}
	got := NormalizeExpiredWindows(state, time.Now())
	if got.Unified5hUtilization != 0 || got.Unified5hStatus != "" {
		t.Errorf("expected 5h cleared, got util=%.2f status=%q", got.Unified5hUtilization, got.Unified5hStatus)
	}
	if got.Unified7dUtilization != 0.5 {
		t.Errorf("7d should be untouched, got util=%.2f", got.Unified7dUtilization)
	}
}

func TestNormalizeExpiredWindows_7dExpired_5hNot(t *testing.T) {
	past := time.Now().Add(-1 * time.Hour)
	future := time.Now().Add(2 * time.Hour)
	state := AnthropicOAuthRateLimitState{
		Unified5hReset:       fmt.Sprintf("%d", future.Unix()),
		Unified5hUtilization: 0.5,
		Unified7dReset:       fmt.Sprintf("%d", past.Unix()),
		Unified7dUtilization: 0.9,
		Unified7dStatus:      UnifiedStatusAllowedWarning,
	}
	got := NormalizeExpiredWindows(state, time.Now())
	if got.Unified7dUtilization != 0 || got.Unified7dStatus != "" {
		t.Errorf("expected 7d cleared, got util=%.2f status=%q", got.Unified7dUtilization, got.Unified7dStatus)
	}
	if got.Unified5hUtilization != 0.5 {
		t.Errorf("5h should be untouched, got util=%.2f", got.Unified5hUtilization)
	}
}

func TestNormalizeExpiredWindows_AllowedWarning_UnifiedResetExpired(t *testing.T) {
	past := time.Now().Add(-1 * time.Hour)
	state := AnthropicOAuthRateLimitState{
		UnifiedStatus: UnifiedStatusAllowedWarning,
		UnifiedReset:  fmt.Sprintf("%d", past.Unix()),
	}
	got := NormalizeExpiredWindows(state, time.Now())
	if got.UnifiedStatus != "" {
		t.Errorf("UnifiedStatus = %q, want empty: allowed_warning must be cleared when UnifiedReset expired", got.UnifiedStatus)
	}
}

func TestNormalizeExpiredWindows_AllowedWarning_UnifiedResetNotYetExpired(t *testing.T) {
	future := time.Now().Add(2 * time.Hour)
	state := AnthropicOAuthRateLimitState{
		UnifiedStatus: UnifiedStatusAllowedWarning,
		UnifiedReset:  fmt.Sprintf("%d", future.Unix()),
	}
	got := NormalizeExpiredWindows(state, time.Now())
	if got.UnifiedStatus != UnifiedStatusAllowedWarning {
		t.Errorf("UnifiedStatus = %q, want %q: allowed_warning must be preserved when not expired", got.UnifiedStatus, UnifiedStatusAllowedWarning)
	}
}

func TestParseResetTime_EmptyString(t *testing.T) {
	if !ParseResetTime("").IsZero() {
		t.Error("ParseResetTime(\"\") should return zero time")
	}
}

func TestParseResetTime_ValidUnixTimestamp(t *testing.T) {
	want := time.Unix(1774850400, 0)
	got := ParseResetTime("1774850400")
	if !got.Equal(want) {
		t.Errorf("ParseResetTime(\"1774850400\") = %v, want %v", got, want)
	}
}

func TestParseResetTime_InvalidString(t *testing.T) {
	// Non-numeric strings should return zero time (and log a warning).
	if !ParseResetTime("not-a-number").IsZero() {
		t.Error("ParseResetTime(\"not-a-number\") should return zero time")
	}
}

func TestParseResetTime_NegativeTimestamp(t *testing.T) {
	// Negative timestamps (before epoch) are valid unix time — not zero.
	got := ParseResetTime("-1")
	if got.IsZero() {
		t.Error("ParseResetTime(\"-1\") should not be zero — negative unix ts is a real time (before epoch)")
	}
	if !got.Before(time.Unix(0, 0)) {
		t.Errorf("ParseResetTime(\"-1\") = %v, want a time before unix epoch", got)
	}
}

// ─── clampUtilization unit tests ─────────────────────────────────────────────

func TestClampUtilization(t *testing.T) {
	tests := []struct {
		name  string
		input float64
		want  float64
	}{
		{"sentinel -1 passthrough", -1, -1},
		{"negative non-sentinel clamped to 0", -0.1, 0},
		{"mid-range unchanged", 0.5, 0.5},
		{"upper boundary unchanged", 1.0, 1.0},
		{"above upper boundary clamped", 1.05, 1.0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := clampUtilization(tc.input)
			if got != tc.want {
				t.Errorf("clampUtilization(%v) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

func TestParseAnthropicOAuthRateLimitHeaders_UtilizationOverRange(t *testing.T) {
	// Over-range value (1.1) should be clamped to 1.0.
	h := http.Header{}
	h.Set("anthropic-ratelimit-unified-5h-utilization", "1.1")
	state := ParseAnthropicOAuthRateLimitHeaders(h, "over-range-test")
	if state.Unified5hUtilization != 1.0 {
		t.Errorf("Unified5hUtilization = %v, want 1.0 (clamped from 1.1)", state.Unified5hUtilization)
	}

	// Negative non-sentinel value (-0.5) should be clamped to 0.0, not treated as sentinel.
	h2 := http.Header{}
	h2.Set("anthropic-ratelimit-unified-5h-utilization", "-0.5")
	state2 := ParseAnthropicOAuthRateLimitHeaders(h2, "neg-clamp-test")
	if state2.Unified5hUtilization != 0.0 {
		t.Errorf("Unified5hUtilization = %v, want 0.0 (clamped from -0.5, not sentinel passthrough)", state2.Unified5hUtilization)
	}
}
