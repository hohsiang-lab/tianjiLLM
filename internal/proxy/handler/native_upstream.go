package handler

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/praxisllmlab/tianjiLLM/internal/callback"
	"github.com/praxisllmlab/tianjiLLM/internal/provider/anthropic"
)

// nativeUpstream holds the resolved base URL and API key for a single upstream entry.
type nativeUpstream struct {
	BaseURL string
	APIKey  string
}

// roundRobinCounters holds per-provider atomic counters for round-robin upstream selection.
// FR-018: per-provider counters prevent cross-provider index drift.
var (
	roundRobinMu       sync.RWMutex
	roundRobinCounters = map[string]*atomic.Uint64{}
)

// resolveAllNativeUpstreams returns all upstream entries matching the given providerName.
// FR-017: unlike resolveNativeUpstream which returns only the first match, this returns all.
func (h *Handlers) resolveAllNativeUpstreams(providerName string) []nativeUpstream {
	var results []nativeUpstream
	for _, m := range h.Config.ModelList {
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

// parseUnixResetTime parses a unix timestamp string into time.Time.
// Returns zero time if the string is empty or unparseable.
func parseUnixResetTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	sec, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return time.Time{}
	}
	return time.Unix(sec, 0)
}

// roundRobinSelect implements a goroutine-safe per-provider round-robin selection.
// FR-018: each provider maintains its own atomic counter to avoid cross-provider drift.
func roundRobinSelect(provider string, upstreams []nativeUpstream) nativeUpstream {
	if len(upstreams) == 0 {
		return nativeUpstream{}
	}
	roundRobinMu.RLock()
	counter, ok := roundRobinCounters[provider]
	roundRobinMu.RUnlock()
	if !ok {
		roundRobinMu.Lock()
		// Double-check after acquiring write lock.
		if counter, ok = roundRobinCounters[provider]; !ok {
			counter = &atomic.Uint64{}
			roundRobinCounters[provider] = counter
		}
		roundRobinMu.Unlock()
	}
	idx := counter.Add(1) - 1
	return upstreams[idx%uint64(len(upstreams))]
}

// selectUpstreamWithThrottle filters out OAuth tokens that exceed the utilization
// threshold (5h or 7d) or are already rate-limited, then round-robins among the
// remaining healthy tokens. Returns allTokensThrottledError if none are available.
func (h *Handlers) selectUpstreamWithThrottle(
	providerName string, upstreams []nativeUpstream,
) (nativeUpstream, error) {
	if providerName != "anthropic" || h.RateLimitStore == nil {
		return roundRobinSelect(providerName, upstreams), nil
	}

	threshold := h.Config.RatelimitAlertThreshold
	if threshold == 0 {
		threshold = 0.8
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
		state, ok := h.RateLimitStore.Get(key)
		if !ok {
			available = append(available, u)
			continue
		}

		if state.UnifiedStatus == "rate_limited" || state.UnifiedStatus == "overage" {
			trackNearestReset(&nearestReset, state.UnifiedReset, state.Unified5hReset, state.Unified7dReset)
			continue
		}

		throttled := false
		if state.Unified5hUtilization >= 0 && state.Unified5hUtilization >= threshold {
			throttled = true
		}
		if state.Unified7dUtilization >= 0 && state.Unified7dUtilization >= threshold {
			throttled = true
		}

		if throttled {
			trackNearestReset(&nearestReset, state.Unified5hReset, state.Unified7dReset)
			continue
		}

		available = append(available, u)
	}

	if len(available) == 0 {
		return nativeUpstream{}, &allTokensThrottledError{resetAt: nearestReset}
	}
	return roundRobinSelect(providerName, available), nil
}

// trackNearestReset updates nearest with the earliest future reset time from the given timestamps.
func trackNearestReset(nearest *time.Time, timestamps ...string) {
	for _, ts := range timestamps {
		t := parseUnixResetTime(ts)
		if t.IsZero() {
			continue
		}
		if nearest.IsZero() || t.Before(*nearest) {
			*nearest = t
		}
	}
}
