package callback

import (
	"fmt"
	"testing"
	"time"
)

// futureReset returns a reset timestamp string N hours in the future.
func futureReset(hours int) string {
	return fmt.Sprintf("%d", time.Now().Add(time.Duration(hours)*time.Hour).Unix())
}

// ─── HO-494: Merge logic — non-Sonnet response must not wipe Sonnet data ────

func TestMergeRateLimitState_PreservesSonnetWhenIncomingHasNoData(t *testing.T) {
	existing := AnthropicOAuthRateLimitState{
		TokenKey:                   "tok1",
		Unified7dSonnetStatus:      UnifiedStatusAllowed,
		Unified7dSonnetUtilization: 0.95,
		Unified7dSonnetReset:       futureReset(24),
		Unified7dUtilization:       0.60,
		Unified7dStatus:            UnifiedStatusAllowed,
		Unified5hUtilization:       0.30,
		Unified5hStatus:            UnifiedStatusAllowed,
	}
	// Simulate an Opus response: no Sonnet headers → sentinels.
	incoming := AnthropicOAuthRateLimitState{
		TokenKey:                   "tok1",
		Unified7dSonnetStatus:      "", // no data
		Unified7dSonnetUtilization: -1, // sentinel
		Unified7dSonnetReset:       "", // no data
		Unified7dUtilization:       0.62,
		Unified7dStatus:            UnifiedStatusAllowed,
		Unified5hUtilization:       0.32,
		Unified5hStatus:            UnifiedStatusAllowed,
		ParsedAt:                   time.Now(),
	}

	merged := mergeRateLimitState(existing, incoming)

	// Sonnet fields must be preserved from existing.
	if merged.Unified7dSonnetStatus != UnifiedStatusAllowed {
		t.Errorf("Unified7dSonnetStatus = %q, want %q (preserved)", merged.Unified7dSonnetStatus, UnifiedStatusAllowed)
	}
	if merged.Unified7dSonnetUtilization != 0.95 {
		t.Errorf("Unified7dSonnetUtilization = %v, want 0.95 (preserved)", merged.Unified7dSonnetUtilization)
	}
	if merged.Unified7dSonnetReset == "" {
		t.Error("Unified7dSonnetReset should be preserved, got empty")
	}
	// Non-Sonnet fields must be updated from incoming.
	if merged.Unified7dUtilization != 0.62 {
		t.Errorf("Unified7dUtilization = %v, want 0.62 (updated)", merged.Unified7dUtilization)
	}
	if merged.Unified5hUtilization != 0.32 {
		t.Errorf("Unified5hUtilization = %v, want 0.32 (updated)", merged.Unified5hUtilization)
	}
}

func TestMergeRateLimitState_UpdatesSonnetWhenIncomingHasData(t *testing.T) {
	existing := AnthropicOAuthRateLimitState{
		TokenKey:                   "tok1",
		Unified7dSonnetUtilization: 0.80,
		Unified7dSonnetStatus:      UnifiedStatusAllowed,
	}
	// Sonnet response with fresh data.
	incoming := AnthropicOAuthRateLimitState{
		TokenKey:                   "tok1",
		Unified7dSonnetUtilization: 0.95,
		Unified7dSonnetStatus:      UnifiedStatusAllowedWarning,
		ParsedAt:                   time.Now(),
	}

	merged := mergeRateLimitState(existing, incoming)

	if merged.Unified7dSonnetUtilization != 0.95 {
		t.Errorf("Unified7dSonnetUtilization = %v, want 0.95 (updated)", merged.Unified7dSonnetUtilization)
	}
	if merged.Unified7dSonnetStatus != UnifiedStatusAllowedWarning {
		t.Errorf("Unified7dSonnetStatus = %q, want %q (updated)", merged.Unified7dSonnetStatus, UnifiedStatusAllowedWarning)
	}
}

func TestMergeRateLimitState_PreservesLegacyFieldsWhenIncomingSentinel(t *testing.T) {
	existing := AnthropicOAuthRateLimitState{
		RequestsLimit:     1000,
		RequestsRemaining: 900,
		TokensLimit:       100000,
		TokensRemaining:   90000,
		RequestsResetAt:   "1700000000",
		TokensResetAt:     "1700001000",
	}
	incoming := AnthropicOAuthRateLimitState{
		RequestsLimit:     -1,
		RequestsRemaining: -1,
		TokensLimit:       -1,
		TokensRemaining:   -1,
		RequestsResetAt:   "",
		TokensResetAt:     "",
	}

	merged := mergeRateLimitState(existing, incoming)

	if merged.RequestsLimit != 1000 {
		t.Errorf("RequestsLimit = %d, want 1000 (preserved)", merged.RequestsLimit)
	}
	if merged.TokensRemaining != 90000 {
		t.Errorf("TokensRemaining = %d, want 90000 (preserved)", merged.TokensRemaining)
	}
	if merged.RequestsResetAt != "1700000000" {
		t.Errorf("RequestsResetAt = %q, want preserved", merged.RequestsResetAt)
	}
}

func TestMergeRateLimitState_PreservesFallbackAndOverage(t *testing.T) {
	existing := AnthropicOAuthRateLimitState{
		FallbackPercentage:    0.5,
		OverageDisabledReason: "policy",
		OrganizationID:        "org-123",
		RepresentativeClaim:   "seven_day",
	}
	incoming := AnthropicOAuthRateLimitState{
		FallbackPercentage:    -1, // no data
		OverageDisabledReason: "", // no data
		OrganizationID:        "", // no data
		RepresentativeClaim:   "", // no data
	}

	merged := mergeRateLimitState(existing, incoming)

	if merged.FallbackPercentage != 0.5 {
		t.Errorf("FallbackPercentage = %v, want 0.5 (preserved)", merged.FallbackPercentage)
	}
	if merged.OverageDisabledReason != "policy" {
		t.Errorf("OverageDisabledReason = %q, want 'policy' (preserved)", merged.OverageDisabledReason)
	}
	if merged.OrganizationID != "org-123" {
		t.Errorf("OrganizationID = %q, want 'org-123' (preserved)", merged.OrganizationID)
	}
	if merged.RepresentativeClaim != "seven_day" {
		t.Errorf("RepresentativeClaim = %q, want 'seven_day' (preserved)", merged.RepresentativeClaim)
	}
}

// TestPutLocked_MergeIntegration verifies the full Set→Set flow merges correctly.
// Uses future reset timestamps to avoid NormalizeExpiredWindows clearing them on Get().
func TestPutLocked_MergeIntegration_SonnetThenOpus(t *testing.T) {
	store := NewInMemoryRateLimitStore()

	// First: Sonnet response sets Sonnet utilization.
	store.Set("tok1", AnthropicOAuthRateLimitState{
		TokenKey:                   "tok1",
		Unified7dSonnetUtilization: 0.95,
		Unified7dSonnetStatus:      UnifiedStatusAllowed,
		Unified7dSonnetReset:       futureReset(72),
		Unified7dUtilization:       0.60,
		Unified7dReset:             futureReset(120),
		Unified5hUtilization:       0.30,
		Unified5hReset:             futureReset(3),
		ParsedAt:                   time.Now(),
	})

	// Second: Opus response — no Sonnet headers.
	store.Set("tok1", AnthropicOAuthRateLimitState{
		TokenKey:                   "tok1",
		Unified7dSonnetUtilization: -1, // sentinel
		Unified7dSonnetStatus:      "",
		Unified7dSonnetReset:       "",
		Unified7dUtilization:       0.65,
		Unified7dReset:             futureReset(120),
		Unified5hUtilization:       0.35,
		Unified5hReset:             futureReset(3),
		ParsedAt:                   time.Now(),
	})

	got, ok := store.Get("tok1")
	if !ok {
		t.Fatal("Get returned ok=false")
	}

	// Sonnet data must survive the Opus overwrite.
	if got.Unified7dSonnetUtilization != 0.95 {
		t.Errorf("Unified7dSonnetUtilization = %v, want 0.95 (preserved after Opus response)", got.Unified7dSonnetUtilization)
	}
	if got.Unified7dSonnetStatus != UnifiedStatusAllowed {
		t.Errorf("Unified7dSonnetStatus = %q, want 'allowed' (preserved)", got.Unified7dSonnetStatus)
	}

	// Non-Sonnet data must be updated.
	if got.Unified7dUtilization != 0.65 {
		t.Errorf("Unified7dUtilization = %v, want 0.65 (updated by Opus)", got.Unified7dUtilization)
	}
	if got.Unified5hUtilization != 0.35 {
		t.Errorf("Unified5hUtilization = %v, want 0.35 (updated by Opus)", got.Unified5hUtilization)
	}
}

// TestPutLocked_FirstSetNoMerge verifies first Set (no existing entry) works fine.
func TestPutLocked_FirstSetNoMerge(t *testing.T) {
	store := NewInMemoryRateLimitStore()

	state := AnthropicOAuthRateLimitState{
		TokenKey:                   "tok-new",
		Unified7dSonnetUtilization: 0.50,
		Unified5hUtilization:       -1, // sentinel
		ParsedAt:                   time.Now(),
	}
	store.Set("tok-new", state)

	got, ok := store.Get("tok-new")
	if !ok {
		t.Fatal("Get returned ok=false for new entry")
	}
	if got.Unified7dSonnetUtilization != 0.50 {
		t.Errorf("Unified7dSonnetUtilization = %v, want 0.50", got.Unified7dSonnetUtilization)
	}
	if got.Unified5hUtilization != -1 {
		t.Errorf("Unified5hUtilization = %v, want -1", got.Unified5hUtilization)
	}
}
