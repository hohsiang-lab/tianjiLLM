package handler

import (
	"log/slog"
	"math/rand/v2"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/praxisllmlab/tianjiLLM/internal/callback"
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

// selectUpstream implements a goroutine-safe per-provider round-robin selection.
// FR-018: each provider maintains its own atomic counter to avoid cross-provider drift.
func selectUpstream(provider string, upstreams []nativeUpstream) nativeUpstream {
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

// nativeUtilizationState tracks active token state for native proxy lowest-utilization selection.
var (
	nativeUtilMu         sync.RWMutex
	nativeActiveKey      string
	nativeLastUsedAt     = map[string]time.Time{}
)

// selectUpstreamByUtilization picks the upstream with the lowest 5h utilization.
// Shared logic with router strategy — mirrors pickDeployment behavior.
// Falls back to round-robin when store is nil or no utilization data exists.
func (h *Handlers) selectUpstreamByUtilization(provider string, upstreams []nativeUpstream) nativeUpstream {
	if len(upstreams) == 0 {
		return nativeUpstream{}
	}
	if h.RateLimitStore == nil {
		return selectUpstream(provider, upstreams)
	}

	// Build key → upstream mapping.
	keyToUpstream := make(map[string]nativeUpstream, len(upstreams))
	var allKeys []string
	for _, u := range upstreams {
		key := callback.RateLimitCacheKey(u.APIKey)
		keyToUpstream[key] = u
		allKeys = append(allKeys, key)
	}

	threshold := float64(80) // default
	if h.Config != nil && h.Config.RouterSettings != nil {
		if t, ok := h.Config.RouterSettings.RoutingStrategyArgs["utilization_threshold"]; ok {
			if v, ok := t.(float64); ok {
				threshold = v
			} else if v, ok := t.(int); ok {
				threshold = float64(v)
			}
		}
	}

	nativeUtilMu.Lock()
	defer nativeUtilMu.Unlock()

	picked := pickNativeTokenKey(h.RateLimitStore, allKeys, threshold)

	nativeLastUsedAt[picked] = time.Now()

	if u, ok := keyToUpstream[picked]; ok {
		return u
	}
	return selectUpstream(provider, upstreams)
}

func pickNativeTokenKey(store callback.RateLimitStore, allKeys []string, threshold float64) string {
	// Cold start
	if nativeActiveKey == "" {
		picked := pickBestNativeKey(store, allKeys)
		if picked != "" {
			nativeActiveKey = picked
			return picked
		}
		picked = allKeys[rand.IntN(len(allKeys))]
		nativeActiveKey = picked
		return picked
	}

	// Check if active key is still available
	activeAvailable := false
	for _, k := range allKeys {
		if k == nativeActiveKey {
			activeAvailable = true
			break
		}
	}
	if !activeAvailable {
		return switchNativeKey(store, allKeys)
	}

	// Check utilization
	util, ok := store.GetUtilization(nativeActiveKey)
	if ok {
		state, hasState := store.Get(nativeActiveKey)
		isRateLimited := hasState && state.Unified5hStatus == "rate_limited"
		if isRateLimited || util >= threshold {
			return switchNativeKey(store, allKeys)
		}
	}

	return nativeActiveKey
}

func switchNativeKey(store callback.RateLimitStore, allKeys []string) string {
	oldKey := nativeActiveKey
	picked := pickBestNativeKey(store, allKeys)
	if picked == "" {
		picked = allKeys[rand.IntN(len(allKeys))]
	}
	nativeActiveKey = picked
	if oldKey != "" && picked != oldKey {
		slog.Warn("native proxy OAuth token switched",
			"old_token", maskNativeKey(oldKey),
			"new_token", maskNativeKey(picked))
	}
	return picked
}

func pickBestNativeKey(store callback.RateLimitStore, keys []string) string {
	bestKey := ""
	bestUtil := float64(-1)
	var bestLastUsed time.Time

	for _, key := range keys {
		state, ok := store.Get(key)
		if !ok || state.Unified5hStatus == "rate_limited" || state.Unified5hUtilization < 0 {
			continue
		}
		util := state.Unified5hUtilization * 100
		lastUsed := nativeLastUsedAt[key]
		if bestKey == "" || util < bestUtil || (util == bestUtil && lastUsed.Before(bestLastUsed)) {
			bestKey = key
			bestUtil = util
			bestLastUsed = lastUsed
		}
	}
	return bestKey
}

func maskNativeKey(key string) string {
	if len(key) <= 4 {
		return key
	}
	return key[len(key)-4:]
}
