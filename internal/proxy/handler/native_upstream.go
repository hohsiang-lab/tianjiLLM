package handler

import (
	"strings"
	"sync"
	"sync/atomic"

	"github.com/praxisllmlab/tianjiLLM/internal/callback"
	"github.com/praxisllmlab/tianjiLLM/internal/router/strategy"
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

// nativeUtilInstances holds per-provider LowestUtilization instances,
// ensuring each provider tracks its own active token independently.
var (
	nativeUtilMu        sync.Mutex
	nativeUtilInstances = map[string]*strategy.LowestUtilization{}
)

// selectUpstreamByUtilization picks the upstream with the lowest 5h utilization.
// Delegates to strategy.LowestUtilization.PickKey (shared with router path).
// Falls back to round-robin when store is nil.
func (h *Handlers) selectUpstreamByUtilization(provider string, upstreams []nativeUpstream) nativeUpstream {
	if len(upstreams) == 0 {
		return nativeUpstream{}
	}
	if h.RateLimitStore == nil {
		return selectUpstream(provider, upstreams)
	}

	keyToUpstream := make(map[string]nativeUpstream, len(upstreams))
	var allKeys []string
	for _, u := range upstreams {
		key := callback.RateLimitCacheKey(u.APIKey)
		keyToUpstream[key] = u
		allKeys = append(allKeys, key)
	}

	lu := h.getNativeUtilInstance(provider)
	picked := lu.PickKey(allKeys)

	if u, ok := keyToUpstream[picked]; ok {
		return u
	}
	return selectUpstream(provider, upstreams)
}

// getNativeUtilInstance returns (or lazily creates) a per-provider LowestUtilization instance.
func (h *Handlers) getNativeUtilInstance(provider string) *strategy.LowestUtilization {
	nativeUtilMu.Lock()
	defer nativeUtilMu.Unlock()

	if lu, ok := nativeUtilInstances[provider]; ok {
		return lu
	}

	threshold := float64(80)
	if h.Config != nil && h.Config.RouterSettings != nil {
		if t, ok := h.Config.RouterSettings.RoutingStrategyArgs["utilization_threshold"]; ok {
			if v, ok := t.(float64); ok {
				threshold = v
			} else if v, ok := t.(int); ok {
				threshold = float64(v)
			}
		}
	}

	lu := strategy.NewLowestUtilization(h.RateLimitStore, threshold, nil)
	nativeUtilInstances[provider] = lu
	return lu
}
