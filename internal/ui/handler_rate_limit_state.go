package ui

import (
	"encoding/json"
	"net/http"
	"sort"
	"time"

	"github.com/praxisllmlab/tianjiLLM/internal/ratelimitstate"
)

// dimensionJSON is the JSON shape for one rate limit dimension.
type dimensionJSON struct {
	Limit     int64  `json:"limit"`
	Remaining int64  `json:"remaining"`
	ResetsAt  string `json:"resets_at"`
}

// rateLimitStateJSON is the JSON response body for GET /ui/api/rate-limit-state.
// When has_data is false, all dimension fields are null.
type rateLimitStateJSON struct {
	HasData      bool           `json:"has_data"`
	CapturedAt   string         `json:"captured_at,omitempty"`
	InputTokens  *dimensionJSON `json:"input_tokens"`
	OutputTokens *dimensionJSON `json:"output_tokens"`
	Requests     *dimensionJSON `json:"requests"`
}

// multiTokenStateJSON is the JSON response for multi-token list.
type multiTokenStateJSON struct {
	KeyHash      string         `json:"key_hash"`
	HasData      bool           `json:"has_data"`
	CapturedAt   string         `json:"captured_at,omitempty"`
	InputTokens  *dimensionJSON `json:"input_tokens"`
	OutputTokens *dimensionJSON `json:"output_tokens"`
	Requests     *dimensionJSON `json:"requests"`
}

func toDimensionJSON(d *ratelimitstate.DimensionState) *dimensionJSON {
	if d == nil {
		return nil
	}
	reset := ""
	if !d.ResetsAt.IsZero() {
		reset = d.ResetsAt.Format(time.RFC3339)
	}
	return &dimensionJSON{
		Limit:     d.Limit,
		Remaining: d.Remaining,
		ResetsAt:  reset,
	}
}

// handleRateLimitState handles GET /ui/api/rate-limit-state.
// Requires authentication (returns 401 if no valid session).
// If h.RateLimitStore is set, returns a single-store JSON (backward compat).
// Otherwise, returns a list from the global multi-token registry.
func (h *UIHandler) handleRateLimitState(w http.ResponseWriter, r *http.Request) {
	// Auth check.
	if _, ok := getSessionFromRequest(r, h.sessionKey()); !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	// Single-store mode (used in tests and single-token deployments).
	if h.RateLimitStore != nil {
		snap, ok := h.RateLimitStore.Get()
		resp := rateLimitStateJSON{HasData: ok}
		if ok && snap != nil {
			resp.CapturedAt = snap.CapturedAt.Format(time.RFC3339)
			resp.InputTokens = toDimensionJSON(snap.InputTokens)
			resp.OutputTokens = toDimensionJSON(snap.OutputTokens)
			resp.Requests = toDimensionJSON(snap.Requests)
		}
		_ = json.NewEncoder(w).Encode(resp)
		return
	}

	// Multi-token mode: list all global stores.
	all := ratelimitstate.ListAll()
	entries := make([]multiTokenStateJSON, 0, len(all))
	for keyHash, store := range all {
		snap, ok := store.Get()
		entry := multiTokenStateJSON{KeyHash: keyHash, HasData: ok}
		if ok && snap != nil {
			entry.CapturedAt = snap.CapturedAt.Format(time.RFC3339)
			entry.InputTokens = toDimensionJSON(snap.InputTokens)
			entry.OutputTokens = toDimensionJSON(snap.OutputTokens)
			entry.Requests = toDimensionJSON(snap.Requests)
		}
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].KeyHash < entries[j].KeyHash
	})
	_ = json.NewEncoder(w).Encode(entries)
}
