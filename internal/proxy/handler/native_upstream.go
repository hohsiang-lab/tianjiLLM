package handler

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/praxisllmlab/tianjiLLM/internal/callback"
	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/praxisllmlab/tianjiLLM/internal/provider/anthropic"
)

// sentinelMaxComposite is a sentinel value larger than any real capacity-rate score.
// Real scores range from 0 (best: expired/empty window) to ~100 (at gate threshold).
// Used to initialize bestUtil in lowestUtilizationSelect so any real score wins.
const sentinelMaxComposite = 200.0

// gate7d is the throttle threshold for the 7-day utilization window. Intentionally
// higher than the configurable 5h threshold (RatelimitAlertThreshold): the 7d window
// accumulates usage over a much longer period than the 5h window, so tokens are
// expected to reach higher utilization before load needs to be shed. Keeping it
// separate from the 5h threshold avoids over-throttling tokens that are simply
// building up their rolling weekly baseline.
const gate7d = callback.Gate7d

// nativeUpstream holds the resolved base URL and API key for a single upstream entry.
type nativeUpstream struct {
	BaseURL string
	APIKey  string
}

// stickyEntry tracks the current sticky selection and the 5h reset timestamp
// at the time of selection. When the stored reset differs from the live value,
// the 5h window has cycled and sticky re-evaluates instead of blindly reusing
// the same token across multiple 5h periods.
type stickyEntry struct {
	APIKey    string
	Reset5hAt string // Unified5hReset at selection time
}

// resolveAllNativeUpstreams returns all upstream entries matching the given providerName.
// FR-017: unlike resolveNativeUpstream which returns only the first match, this returns all.
func (h *Handlers) resolveAllNativeUpstreams(ctx context.Context, providerName string) []nativeUpstream {
	var results []nativeUpstream
	for _, m := range h.runtimeModelList(ctx) {
		parts := strings.SplitN(m.TianjiParams.Model, "/", 2)
		if len(parts) >= 1 && parts[0] == providerName {
			apiKey := ""
			if m.TianjiParams.APIKey != nil {
				apiKey = *m.TianjiParams.APIKey
			}
			base := ""
			if m.TianjiParams.APIBase != nil {
				base = *m.TianjiParams.APIBase
			}
			if base == "" {
				base = defaultBaseURL(providerName)
			}
			results = append(results, nativeUpstream{BaseURL: base, APIKey: apiKey})
		}
	}
	return results
}

// allTokensThrottledError is returned when all OAuth tokens exceed the utilization threshold.
type allTokensThrottledError struct {
	resetAt time.Time
}

func (e *allTokensThrottledError) Error() string {
	return fmt.Sprintf("all OAuth tokens throttled, nearest reset at %s", e.resetAt.Format(time.RFC3339))
}

// roundRobinSelect implements a goroutine-safe per-provider round-robin selection.
// FR-018: each provider maintains its own atomic counter to avoid cross-provider drift.
func (h *Handlers) roundRobinSelect(provider string, upstreams []nativeUpstream) nativeUpstream {
	if len(upstreams) == 0 {
		return nativeUpstream{}
	}
	h.roundRobinMu.RLock()
	counter, ok := h.roundRobinCounters[provider]
	h.roundRobinMu.RUnlock()
	if !ok {
		h.roundRobinMu.Lock()
		// Double-check after acquiring write lock.
		if h.roundRobinCounters == nil {
			h.roundRobinCounters = make(map[string]*atomic.Uint64)
		}
		if counter, ok = h.roundRobinCounters[provider]; !ok {
			counter = &atomic.Uint64{}
			h.roundRobinCounters[provider] = counter
		}
		h.roundRobinMu.Unlock()
	}
	idx := counter.Add(1) - 1
	return upstreams[idx%uint64(len(upstreams))]
}

func isSonnetModel(model string) bool {
	return strings.Contains(strings.ToLower(model), "sonnet")
}

// selectUpstreamWithThrottle filters out OAuth tokens that exceed utilization
// thresholds (5h, 7d, 7d_sonnet) or are rate-limited, then delegates to
// the configured strategy. Returns allTokensThrottledError if none pass.
func (h *Handlers) selectUpstreamWithThrottle(
	ctx context.Context, providerName string, upstreams []nativeUpstream, modelName string,
) (nativeUpstream, error) {
	if len(upstreams) == 0 {
		// Return zero-value with nil error so nativeProxy falls through to the
		// upstream.BaseURL == "" guard and returns 501 "not configured".
		log.Printf("warn: native-proxy: no upstreams configured for provider=%s", providerName)
		return nativeUpstream{}, nil
	}

	if providerName != "anthropic" || h.RateLimitStore == nil {
		return h.roundRobinSelect(providerName, upstreams), nil
	}

	threshold := h.Config.RatelimitAlertThreshold
	if threshold <= 0 {
		threshold = callback.DefaultGate5h
	}

	var available []nativeUpstream
	var nearestReset time.Time
	seen := make(map[string]bool, len(upstreams))

	for _, u := range upstreams {
		if seen[u.APIKey] {
			continue
		}
		seen[u.APIKey] = true

		if !anthropic.IsOAuthToken(u.APIKey) {
			available = append(available, u)
			continue
		}

		key := callback.RateLimitCacheKey(u.APIKey)

		if h.DisabledTokens != nil && h.DisabledTokens.IsDisabled(key) {
			continue
		}

		state, ok := h.RateLimitStore.Get(key)
		if !ok {
			available = append(available, u)
			continue
		}

		// rejected: quota exhausted, no extra usage — skip.
		if state.UnifiedStatus == callback.UnifiedStatusRejected {
			trackNearestReset(&nearestReset, state.UnifiedReset, state.Unified5hReset, state.Unified7dReset)
			continue
		}

		// allowed_warning: extra usage (overage credit) active — bypass utilization gates.
		if state.UnifiedStatus == callback.UnifiedStatusAllowedWarning {
			available = append(available, u)
			continue
		}

		// Gate thresholds are positive (0.8, 0.9), so sentinel -1 naturally passes.
		// Sonnet requests use 7d_sonnet gate instead of 7d_all gate (replacement, not additive).
		throttled := false
		if state.Unified5hUtilization >= threshold {
			throttled = true
			log.Printf("warn: gate: skipping token=%s reason=5h_gate (util=%.2f)", key, state.Unified5hUtilization)
		}
		if isSonnetModel(modelName) {
			if state.Unified7dSonnetUtilization >= gate7d {
				throttled = true
				log.Printf("warn: gate: skipping token=%s reason=7d_sonnet_gate (util=%.2f, model=%s)", key, state.Unified7dSonnetUtilization, modelName)
			}
		} else if state.Unified7dUtilization >= gate7d {
			throttled = true
			log.Printf("warn: gate: skipping token=%s reason=7d_gate (util=%.2f)", key, state.Unified7dUtilization)
		}

		if throttled {
			trackNearestReset(&nearestReset, state.Unified5hReset, state.Unified7dReset, state.Unified7dSonnetReset)
			continue
		}

		available = append(available, u)
	}

	if len(available) == 0 {
		return nativeUpstream{}, &allTokensThrottledError{resetAt: nearestReset}
	}
	switch h.Config.NativeUpstreamStrategy {
	case config.StrategyLowestUtilization:
		return h.lowestUtilizationSelect(ctx, providerName, available), nil
	case config.StrategySticky:
		return h.stickySelect(ctx, providerName, modelName, available), nil
	default:
		return h.roundRobinSelect(providerName, available), nil
	}
}

// stickyTrackKey returns the stickyState map key for a given provider+model combination.
// Sonnet and non-Sonnet requests maintain independent sticky tracks so that
// each model class reuses its own best token without interfering with the other.
func stickyTrackKey(providerName, modelName string) string {
	if isSonnetModel(modelName) {
		return providerName + ":sonnet"
	}
	return providerName + ":all"
}

// stickySelect reuses the current upstream until it leaves the available set or
// its 5h window has reset. On re-evaluation, picks the token with the soonest
// Unified7dReset timestamp — this maximizes quota burn-rate by always routing
// to the token whose 7-day window is nearest expiry. Utilization levels are
// handled entirely at the gate layer; scoring only uses reset time.
// Sonnet and non-Sonnet requests use independent tracks (":sonnet" vs ":all")
// so prompt-cache affinity per class survives cross-class traffic.
//
// Unknown-token semantics: a token absent from both memory and DB has no known
// reset timestamp, so it is treated as if its reset is maximally far away
// (math.MaxInt64). It loses to any token with a known 7d reset. When every
// token is unknown, all tie at MaxInt64 and the first upstream wins.
func (h *Handlers) stickySelect(ctx context.Context, providerName, modelName string, upstreams []nativeUpstream) nativeUpstream {
	if len(upstreams) == 1 {
		return upstreams[0]
	}
	if h.RateLimitStore == nil {
		return h.roundRobinSelect(providerName, upstreams)
	}

	trackKey := stickyTrackKey(providerName, modelName)
	h.stickyMu.Lock()
	if legacyEntry, ok := h.stickyState[trackKey]; ok {
		h.stickyCore.SeedIfAbsent(trackKey, stickyStrategyEntry{
			SelectedID: callback.RateLimitCacheKey(legacyEntry.APIKey),
			Metadata:   legacyEntry,
		})
	}
	h.stickyMu.Unlock()

	now := time.Now()
	candidates := make([]stickyStrategyCandidate, 0, len(upstreams))
	for _, u := range upstreams {
		key := callback.RateLimitCacheKey(u.APIKey)
		state, ok := h.RateLimitStore.Get(key)

		// Memory miss — try to recover from DB and back-fill the store.
		// Use a short timeout: this is the request hot-path, a slow DB must not block routing.
		if !ok && h.RateLimitDB != nil {
			dbCtx, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
			row, dbErr := h.RateLimitDB.GetOAuthTokenRateLimitState(dbCtx, key)
			cancel()
			switch {
			case dbErr == nil:
				state = callback.NormalizeExpiredWindows(callback.DBRowToState(row), now)
				h.RateLimitStore.SetClean(key, state)
				ok = true
			case errors.Is(dbErr, callback.ErrRateLimitStateNotFound):
				// Token not in DB yet — normal for new tokens, no log needed.
			case errors.Is(dbErr, context.Canceled):
				log.Printf("warn: sticky: DB lookup cancelled for token=%s, treating as farthest reset", key)
			case errors.Is(dbErr, context.DeadlineExceeded):
				log.Printf("warn: sticky: DB lookup timed out for token=%s, treating as farthest reset", key)
			default:
				log.Printf("ERROR: sticky: DB lookup failed for token=%s: %v", key, dbErr)
			}
		}

		// Parse Unified7dReset unix timestamp. Tokens without a known reset are
		// treated as maximally far away (MaxInt64) and lose to any known token.
		reset := int64(math.MaxInt64)
		if ok && state.Unified7dReset != "" {
			if ts, err := strconv.ParseInt(state.Unified7dReset, 10, 64); err == nil {
				reset = ts
			}
		}
		candidates = append(candidates, stickyStrategyCandidate{
			ID:       key,
			Payload:  u,
			Metadata: stickyEntry{APIKey: u.APIKey, Reset5hAt: state.Unified5hReset},
			// Claude treats unknown 7d reset as farthest, so every candidate has
			// a comparable score and strict tie handling preserves input order.
			MetadataScore: reset,
		})
	}

	selected, entry, ok := h.stickyCore.Select(trackKey, candidates, stickyStrategyPolicy{
		CanReuse: func(entry stickyStrategyEntry, candidate stickyStrategyCandidate) bool {
			previous, _ := entry.Metadata.(stickyEntry)
			current, _ := candidate.Metadata.(stickyEntry)
			if previous.Reset5hAt == "" {
				return true
			}
			reuse := previous.Reset5hAt == current.Reset5hAt
			if !reuse {
				log.Printf("info: sticky: provider=%s track=%s 5h window reset for token=%s, re-evaluating",
					providerName, trackKey, candidate.ID)
			}
			return reuse
		},
		Select: func(candidates []stickyStrategyCandidate) stickyStrategyCandidate {
			return selectLowestStickyScore(candidates, func(candidate stickyStrategyCandidate) (int64, bool) {
				return candidate.MetadataScore, true
			})
		},
	})
	if !ok {
		return upstreams[0]
	}
	best, ok := selected.Payload.(nativeUpstream)
	if !ok {
		return upstreams[0]
	}
	bestEntry, _ := entry.Metadata.(stickyEntry)
	bestReset := selected.MetadataScore

	if bestReset == math.MaxInt64 {
		log.Printf("info: sticky: provider=%s track=%s switched to token=%s (7d_reset=unknown)",
			providerName, trackKey, callback.RateLimitCacheKey(best.APIKey))
	} else {
		resetIn := time.Until(time.Unix(bestReset, 0)).Truncate(time.Minute)
		log.Printf("info: sticky: provider=%s track=%s switched to token=%s (7d_reset_in=%s)",
			providerName, trackKey, callback.RateLimitCacheKey(best.APIKey), resetIn)
	}

	h.stickyMu.Lock()
	if h.stickyState == nil {
		h.stickyState = make(map[string]stickyEntry)
	}
	h.stickyState[trackKey] = bestEntry
	h.stickyMu.Unlock()
	return best
}

// sticky5hReset returns true if the token's current 5h reset timestamp differs
// from what was recorded at selection time, indicating the 5h window has cycled.
func (h *Handlers) sticky5hReset(entry stickyEntry) bool {
	if h.RateLimitStore == nil || entry.Reset5hAt == "" {
		return false
	}
	state, ok := h.RateLimitStore.Get(callback.RateLimitCacheKey(entry.APIKey))
	if !ok {
		return false
	}
	return state.Unified5hReset != entry.Reset5hAt
}

// lowestUtilizationSelect picks the upstream with the lowest capacity-rate score:
// max(τ_w / (1 - u_w/θ_w)) across all three windows, where τ = remaining/window_length.
// Lower score = more remaining capacity per unit time = better candidate.
// On a memory miss (e.g. after PruneExpired), it loads state from DB and back-fills
// the memory store. If DB data exists but all reset windows have passed, the token
// is treated as idle (composite=0). Falls back to round-robin only when no token
// reached the composite evaluation step (i.e., the context was cancelled before any token was evaluated).
// If the context is cancelled mid-loop and at least one token has been scored, the
// partial best result is returned rather than round-robin.
func (h *Handlers) lowestUtilizationSelect(ctx context.Context, providerName string, upstreams []nativeUpstream) nativeUpstream {
	if len(upstreams) <= 1 || h.RateLimitStore == nil {
		return h.roundRobinSelect(providerName, upstreams)
	}

	// TimeWeightedScore applies its own default when threshold5h <= 0.
	threshold := h.Config.RatelimitAlertThreshold

	var best nativeUpstream
	bestUtil := sentinelMaxComposite
	total := len(upstreams)
	evaluated := 0

	for _, u := range upstreams {
		// Stop issuing DB queries if the request was cancelled.
		if ctx.Err() != nil {
			if evaluated > 0 {
				log.Printf("warn: lowest-util: context cancelled after evaluating %d/%d upstreams, using partial result", evaluated, total)
			}
			break
		}
		evaluated++

		key := callback.RateLimitCacheKey(u.APIKey)
		state, ok := h.RateLimitStore.Get(key)

		// Memory miss — try to recover from DB and back-fill the store.
		// Use a short timeout: this is the request hot-path, a slow DB must not block routing.
		if !ok && h.RateLimitDB != nil {
			dbCtx, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
			row, dbErr := h.RateLimitDB.GetOAuthTokenRateLimitState(dbCtx, key)
			cancel()
			switch {
			case dbErr == nil:
				state = callback.NormalizeExpiredWindows(callback.DBRowToState(row), time.Now())
				// SetClean avoids marking dirty — no need to flush data just read from DB.
				h.RateLimitStore.SetClean(key, state)
				ok = true
			case errors.Is(dbErr, callback.ErrRateLimitStateNotFound):
				// Token not in DB yet — normal for new tokens, no log needed.
			case errors.Is(dbErr, context.Canceled):
				// Request was cancelled before DB responded — not a DB problem.
				log.Printf("warn: lowest-util: DB lookup cancelled for token=%s, assigning composite=0 (fail-open)", key)
			case errors.Is(dbErr, context.DeadlineExceeded):
				log.Printf("warn: lowest-util: DB lookup timed out for token=%s, assigning composite=0 (fail-open)", key)
			default:
				log.Printf("ERROR: lowest-util: DB lookup failed for token=%s: %v", key, dbErr)
			}
		}

		// Both Get() and the DB backfill path normalize expired windows.
		// TimeWeightedScore handles sentinels internally and discounts by remaining time.
		var composite float64
		if !ok {
			composite = 0 // no data anywhere = assume idle
		} else {
			composite = callback.TimeWeightedScore(state, threshold, time.Now())
		}
		if composite < bestUtil {
			bestUtil = composite
			best = u
		}
	}

	// Fall back to round-robin only when no token reached the composite evaluation
	// step — i.e., bestUtil was never updated from the sentinel, meaning every token
	// was missing from both memory and DB.
	if bestUtil == sentinelMaxComposite {
		log.Printf("warn: lowest-util: no composite scores available for %d upstreams, falling back to round-robin", total)
		return h.roundRobinSelect(providerName, upstreams)
	}
	return best
}

// trackNearestReset updates nearest with the earliest reset time from the given timestamps.
// Does not filter past times — the caller handles stale resets (e.g. Retry-After floors to 60s).
func trackNearestReset(nearest *time.Time, timestamps ...string) {
	for _, ts := range timestamps {
		t := callback.ParseResetTime(ts)
		if t.IsZero() {
			continue
		}
		if nearest.IsZero() || t.Before(*nearest) {
			*nearest = t
		}
	}
}
