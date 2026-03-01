package ui

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/praxisllmlab/tianjiLLM/internal/ui/pages"
)

// handleRateLimitState serves GET /ui/api/rate-limit-state.
// FR-008: returns JSON array of per-token rate limit state. Empty array when no data.
func (h *UIHandler) handleRateLimitState(w http.ResponseWriter, r *http.Request) {
	type tokenState struct {
		TokenKey              string    `json:"token_key"`
		UpdatedAt             time.Time `json:"updated_at"`
		RequestsLimit         int       `json:"requests_limit"`
		RequestsRemaining     int       `json:"requests_remaining"`
		TokensLimit           int       `json:"tokens_limit"`
		TokensRemaining       int       `json:"tokens_remaining"`
		RequestsResetAt       string    `json:"requests_reset_at"`
		TokensResetAt         string    `json:"tokens_reset_at"`
		// Unified OAuth fields
		UnifiedStatus         string  `json:"unified_status"`
		Unified5hStatus       string  `json:"unified_5h_status"`
		Unified5hUtilization  float64 `json:"unified_5h_utilization"`
		Unified5hReset        string  `json:"unified_5h_reset"`
		Unified7dStatus       string  `json:"unified_7d_status"`
		Unified7dUtilization  float64 `json:"unified_7d_utilization"`
		Unified7dReset        string  `json:"unified_7d_reset"`
		RepresentativeClaim   string  `json:"representative_claim"`
		OverageDisabledReason string  `json:"overage_disabled_reason"`
	}

	var items []tokenState
	if h.RateLimitStore != nil {
		all := h.RateLimitStore.GetAll()
		for _, s := range all {
			items = append(items, tokenState{
				TokenKey:              s.TokenKey,
				UpdatedAt:             s.ParsedAt,
				RequestsLimit:         s.RequestsLimit,
				RequestsRemaining:     s.RequestsRemaining,
				TokensLimit:           s.TokensLimit,
				TokensRemaining:       s.TokensRemaining,
				RequestsResetAt:       s.RequestsResetAt,
				TokensResetAt:         s.TokensResetAt,
				UnifiedStatus:         s.UnifiedStatus,
				Unified5hStatus:       s.Unified5hStatus,
				Unified5hUtilization:  s.Unified5hUtilization,
				Unified5hReset:        s.Unified5hReset,
				Unified7dStatus:       s.Unified7dStatus,
				Unified7dUtilization:  s.Unified7dUtilization,
				Unified7dReset:        s.Unified7dReset,
				RepresentativeClaim:   s.RepresentativeClaim,
				OverageDisabledReason: s.OverageDisabledReason,
			})
		}
	}

	if items == nil {
		items = []tokenState{}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(items)
}

// buildRateLimitWidgetData converts store data into templ widget format.
// FR-012: called from handleUsage for server-side initial render.
func (h *UIHandler) buildRateLimitWidgetData() []pages.AnthropicRateLimitWidgetData {
	if h.RateLimitStore == nil {
		return nil
	}
	all := h.RateLimitStore.GetAll()
	var out []pages.AnthropicRateLimitWidgetData
	for _, s := range all {
		out = append(out, pages.AnthropicRateLimitWidgetData{
			TokenKey:              s.TokenKey,
			UnifiedStatus:         s.UnifiedStatus,
			Unified5hStatus:       s.Unified5hStatus,
			Unified5hUtilization:  s.Unified5hUtilization,
			Unified7dStatus:       s.Unified7dStatus,
			Unified7dUtilization:  s.Unified7dUtilization,
			RepresentativeClaim:   s.RepresentativeClaim,
			OverageDisabledReason: s.OverageDisabledReason,
		})
	}
	return out
}
