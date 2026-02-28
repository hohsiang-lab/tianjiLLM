package handler

import (
	"net/http"
	"time"
)

// RateLimitStatus handles GET /internal/ratelimit.
func (h *Handlers) RateLimitStatus(w http.ResponseWriter, r *http.Request) {
	if h.RateLimitStore == nil {
		writeJSON(w, http.StatusOK, map[string]any{"providers": map[string]any{}})
		return
	}

	all := h.RateLimitStore.All()
	providers := make(map[string]any, len(all))
	for k, st := range all {
		providers[k] = map[string]any{
			"tokens_limit":       st.TokensLimit,
			"tokens_remaining":   st.TokensRemaining,
			"tokens_reset":       st.TokensReset.UTC().Format(time.RFC3339),
			"requests_limit":     st.RequestsLimit,
			"requests_remaining": st.RequestsRemaining,
			"updated_at":         st.UpdatedAt.UTC().Format(time.RFC3339),
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{"providers": providers})
}
