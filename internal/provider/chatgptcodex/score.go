package chatgptcodex

import (
	"math"
	"time"
)

const (
	DefaultPrimaryGate   = 0.9
	DefaultSecondaryGate = 0.9
	MaxSelectionScore    = 100.0
)

// SelectionScore computes the Codex capacity-rate score from the authoritative
// upstream rate_limit object. Lower scores are better.
func SelectionScore(snapshot UsageSnapshot, primaryGate float64) (score float64, available bool, ok bool) {
	return SelectionScoreAt(snapshot, primaryGate, time.Time{}, time.Time{})
}

func SelectionScoreAt(snapshot UsageSnapshot, primaryGate float64, fetchedAt, now time.Time) (score float64, available bool, ok bool) {
	return SelectionScoreAtWithGates(snapshot, primaryGate, DefaultSecondaryGate, fetchedAt, now)
}

func SelectionScoreAtWithGates(snapshot UsageSnapshot, primaryGate, secondaryGate float64, fetchedAt, now time.Time) (score float64, available bool, ok bool) {
	if primaryGate <= 0 {
		primaryGate = DefaultPrimaryGate
	}
	if secondaryGate <= 0 {
		secondaryGate = DefaultSecondaryGate
	}
	snapshot = ageAdjustedSnapshot(snapshot, fetchedAt, now)
	if snapshot.RateLimit == nil || snapshot.RateLimit.Allowed == nil || snapshot.RateLimit.LimitReached == nil {
		return 0, false, false
	}
	if !*snapshot.RateLimit.Allowed || *snapshot.RateLimit.LimitReached {
		return MaxSelectionScore, false, true
	}
	primaryScore, ok := windowScore(snapshot.RateLimit.PrimaryWindow, primaryGate)
	if !ok {
		return 0, false, false
	}
	secondaryScore, ok := windowScore(snapshot.RateLimit.SecondaryWindow, secondaryGate)
	if !ok {
		return 0, false, false
	}
	return math.Max(primaryScore, secondaryScore), true, true
}

func ageAdjustedSnapshot(snapshot UsageSnapshot, fetchedAt, now time.Time) UsageSnapshot {
	if snapshot.RateLimit == nil || fetchedAt.IsZero() || now.IsZero() || !now.After(fetchedAt) {
		return snapshot
	}
	elapsed := int64(now.Sub(fetchedAt).Seconds())
	if elapsed <= 0 {
		return snapshot
	}
	rateLimit := *snapshot.RateLimit
	rateLimit.PrimaryWindow = ageAdjustedWindow(rateLimit.PrimaryWindow, elapsed)
	rateLimit.SecondaryWindow = ageAdjustedWindow(rateLimit.SecondaryWindow, elapsed)
	snapshot.RateLimit = &rateLimit
	snapshot.PrimaryWindow = ageAdjustedWindow(snapshot.PrimaryWindow, elapsed)
	snapshot.WeeklyWindow = ageAdjustedWindow(snapshot.WeeklyWindow, elapsed)
	return snapshot
}

func ageAdjustedWindow(window UsageWindow, elapsed int64) UsageWindow {
	if window.ResetAfterSeconds == nil {
		return window
	}
	remaining := *window.ResetAfterSeconds - elapsed
	if remaining < 0 {
		remaining = 0
	}
	window.ResetAfterSeconds = &remaining
	return window
}

func windowScore(window UsageWindow, gate float64) (float64, bool) {
	if window.UsedPercent == nil || window.ResetAfterSeconds == nil || window.LimitWindowSeconds == nil {
		return 0, false
	}
	if gate <= 0 || *window.LimitWindowSeconds <= 0 || *window.ResetAfterSeconds < 0 {
		return 0, false
	}
	usage := clampUsagePercent(*window.UsedPercent)
	headroom := 1 - usage/gate
	if headroom <= 0 {
		return MaxSelectionScore, true
	}
	tau := float64(*window.ResetAfterSeconds) / float64(*window.LimitWindowSeconds)
	if tau < 0 {
		tau = 0
	}
	if tau > 1 {
		tau = 1
	}
	return tau / headroom, true
}
