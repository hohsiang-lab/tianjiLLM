package callback

import "time"

const (
	window5h int64   = 18000  // 5 hours in seconds
	window7d int64   = 604800 // 7 days in seconds
	gate7d   float64 = 0.9    // 7-day throttle gate threshold

	// Window7d is exported for callers that need the 7d window length with RemainingFraction.
	Window7d = window7d

	// Gate7d is the 7-day utilization gate threshold shared by the callback and handler packages.
	// Exported so handler.gate7d doesn't duplicate this value.
	Gate7d float64 = gate7d

	// DefaultGate5h is the default 5h utilization gate when the config field
	// RatelimitAlertThreshold is unset or ≤ 0. Exported so all call sites
	// reference the same constant instead of repeating the 0.9 literal.
	DefaultGate5h float64 = 0.9
)

// TimeWeightedScore returns the capacity-rate score for token selection.
// Lower score = better candidate.
//
// For each window: score_w = τ / (1 - r), where:
//   - r = util / gate (normalized utilization, 0..1)
//   - τ = remaining_time / window_length (0..1)
//
// Physical meaning: "how much time remains per unit of remaining capacity."
// Low value = plenty of capacity relative to time = good candidate.
//
// The final score is max across all three windows (bottleneck determines ranking).
// Edge cases:
//   - r → 1 (near gate): score → ∞ (worst — gate layer should filter before this)
//   - τ → 0 (near reset): score → 0 (best — window is about to refresh)
//   - r = 0: score = τ (time fraction alone)
//   - no reset info: falls back to 1/(1-r) (conservative, assumes full window remaining)
func TimeWeightedScore(state AnthropicOAuthRateLimitState, threshold5h float64, now time.Time) float64 {
	if threshold5h <= 0 {
		threshold5h = DefaultGate5h
	}
	return max(
		capacityRate(state.Unified5hUtilization, threshold5h, state.Unified5hReset, window5h, now),
		capacityRate(state.Unified7dUtilization, gate7d, state.Unified7dReset, window7d, now),
		capacityRate(state.Unified7dSonnetUtilization, gate7d, state.Unified7dSonnetReset, window7d, now),
	)
}

// capacityRate computes τ / (1 - r) for a single window.
// Returns 0 when the window has expired or utilization is zero.
// Returns τ/(1-r) with r clamped to avoid division by zero near the gate.
func capacityRate(util, gate float64, resetStr string, windowSec int64, now time.Time) float64 {
	r := max(0, util) / gate
	headroom := 1 - r
	if headroom <= 0 {
		return 100 // at or above gate — effectively worst score
	}
	tau := RemainingFraction(resetStr, windowSec, now)
	if tau < 0 {
		return 1 / headroom // no reset info → assume full window (τ=1), conservative
	}
	return tau / headroom
}

// RemainingFraction returns the fraction of a window remaining until reset.
// Returns -1 for unknown/missing resets, 0 for expired windows.
func RemainingFraction(resetStr string, windowSec int64, now time.Time) float64 {
	rt := ParseResetTime(resetStr)
	if rt.IsZero() {
		return -1 // unknown
	}
	remaining := rt.Sub(now).Seconds()
	if remaining <= 0 {
		return 0
	}
	return min(remaining/float64(windowSec), 1.0)
}
