package handler

import (
	"strings"
	"sync/atomic"
)

// nativeUpstream holds the resolved base URL and API key for a single upstream entry.
type nativeUpstream struct {
	BaseURL string
	APIKey  string
}

// globalRoundRobinCounter is a package-level atomic counter for round-robin upstream selection.
var globalRoundRobinCounter atomic.Uint64

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

// selectUpstream implements a goroutine-safe round-robin selection from the given upstreams slice.
// FR-018: uses atomic.Uint64 counter; no mutex required.
func selectUpstream(upstreams []nativeUpstream) nativeUpstream {
	if len(upstreams) == 0 {
		return nativeUpstream{}
	}
	idx := globalRoundRobinCounter.Add(1) - 1
	return upstreams[idx%uint64(len(upstreams))]
}

