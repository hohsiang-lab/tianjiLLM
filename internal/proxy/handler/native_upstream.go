package handler

import (
	"strings"
	"sync"
	"sync/atomic"
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
