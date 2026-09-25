package ui

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/praxisllmlab/tianjiLLM/internal/callback"
	"github.com/praxisllmlab/tianjiLLM/internal/db"
	"github.com/praxisllmlab/tianjiLLM/internal/provider/anthropic"
	"github.com/praxisllmlab/tianjiLLM/internal/ui/pages"
)

// rateLimitStateReader is a minimal interface for DB reads in loadClaudeCodeData.
// Tests inject a stub; production callers pass h.dbReader().
type rateLimitStateReader interface {
	GetOAuthTokenRateLimitState(ctx context.Context, tokenKey string) (db.OAuthTokenRateLimitState, error)
}

// dbReader returns the production rateLimitStateReader backed by h.DB,
// or nil when no database is configured. The adapter (callback.NewRateLimitDB)
// translates pgx.ErrNoRows into callback.ErrRateLimitStateNotFound, keeping
// pgx out of this layer.
func (h *UIHandler) dbReader() rateLimitStateReader {
	if h.DB != nil {
		return callback.NewRateLimitDB(h.DB)
	}
	return nil
}

// loadClaudeCodeData builds the data for the Claude Code tab.
// Data comes from RateLimitStore (populated by response headers) with DB fallback
// so tokens with expired cache still show utilization data.
func (h *UIHandler) loadClaudeCodeData(r *http.Request, dbr rateLimitStateReader) pages.ClaudeCodeTabData {
	data := pages.ClaudeCodeTabData{
		Strategy: string(h.Config.NativeUpstreamStrategy),
	}

	tokens := h.enumerateOAuthTokens()
	if len(tokens) == 0 {
		return data
	}

	// Load org IDs from DB (survives restarts).
	orgMap := map[string]string{}
	if h.DB != nil {
		rows, err := h.DB.GetAllOAuthTokenMetadata(r.Context())
		if err != nil {
			log.Printf("ERROR loading oauth token metadata: %v", err)
		} else {
			for _, row := range rows {
				orgMap[row.TokenKey] = row.OrgID
			}
		}
	}

	for _, tok := range tokens {
		key := callback.RateLimitCacheKey(tok)

		// Try in-memory RateLimitStore first, then fall back to DB.
		var rlState callback.AnthropicOAuthRateLimitState
		found := false
		if h.RateLimitStore != nil {
			rlState, found = h.RateLimitStore.Get(key)
		}
		if !found {
			if dbr != nil {
				row, err := dbr.GetOAuthTokenRateLimitState(r.Context(), key)
				switch {
				case err == nil:
					rlState = callback.NormalizeExpiredWindows(callback.DBRowToState(row), time.Now())
					found = true
				case errors.Is(err, callback.ErrRateLimitStateNotFound):
					// Token not in DB yet — normal for new tokens.
				default:
					log.Printf("warn: claude-code: DB lookup for token=%s failed: %v", key, err)
				}
			}
		}

		var sessionUsage, weeklyUsage, sonnetUsage *float64
		if found {
			if rlState.Unified5hUtilization >= 0 {
				v := rlState.Unified5hUtilization
				sessionUsage = &v
			}
			if rlState.Unified7dUtilization >= 0 {
				v := rlState.Unified7dUtilization
				weeklyUsage = &v
			}
			if rlState.Unified7dSonnetUtilization >= 0 {
				v := rlState.Unified7dSonnetUtilization
				sonnetUsage = &v
			}
		}

		card := pages.ClaudeCodeTokenCard{
			TokenKey:              key,
			OrgID:                 orgMap[key],
			FetchedAt:             rlState.ParsedAt,
			SessionUsage:          sessionUsage,
			SessionResetAt:        rlState.Unified5hReset,
			WeeklyUsage:           weeklyUsage,
			WeeklyResetAt:         rlState.Unified7dReset,
			SonnetUsage:           sonnetUsage,
			SonnetResetAt:         rlState.Unified7dSonnetReset,
			OverageStatus:         rlState.UnifiedStatus,
			OverageDisabledReason: rlState.OverageDisabledReason,
			Disabled:              h.DisabledTokens != nil && h.DisabledTokens.IsDisabled(key),
		}
		// TimeWeightedScore discounts utilization by remaining window time.
		// Only compute when at least one window has data.
		if sessionUsage != nil || weeklyUsage != nil {
			s := callback.TimeWeightedScore(rlState, h.Config.RatelimitAlertThreshold, time.Now())
			card.CompositeScore = &s
		}
		data.Cards = append(data.Cards, card)
	}

	return data
}

// enumerateOAuthTokens returns deduplicated OAuth tokens from config.
func (h *UIHandler) enumerateOAuthTokens() []string {
	if h.Config == nil {
		return nil
	}
	seen := map[string]bool{}
	var tokens []string
	for _, m := range h.Config.ModelList {
		provider, _, _ := strings.Cut(m.TianjiParams.Model, "/")
		if provider != "anthropic" || m.TianjiParams.APIKey == nil {
			continue
		}
		tok := *m.TianjiParams.APIKey
		if !anthropic.IsOAuthToken(tok) {
			continue
		}
		key := callback.RateLimitCacheKey(tok)
		if seen[key] {
			continue
		}
		seen[key] = true
		tokens = append(tokens, tok)
	}
	return tokens
}

// handleOAuthTokenDisable disables an OAuth token from upstream selection.
func (h *UIHandler) handleOAuthTokenDisable(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	tokenKey := r.FormValue("token_key")
	if tokenKey == "" {
		http.Error(w, "token_key required", http.StatusBadRequest)
		return
	}
	if h.DB != nil {
		if err := h.DB.DisableOAuthToken(r.Context(), tokenKey); err != nil {
			log.Printf("ERROR disabling oauth token %s: %v", tokenKey, err)
			http.Error(w, "failed to persist token state", http.StatusInternalServerError)
			return
		}
	}
	if h.DisabledTokens != nil {
		h.DisabledTokens.Disable(tokenKey)
	}
	data := h.loadClaudeCodeData(r, h.dbReader())
	render(r.Context(), w, pages.UsageClaudeCodeTab(data))
}

// handleOAuthTokenEnable re-enables a previously disabled OAuth token.
func (h *UIHandler) handleOAuthTokenEnable(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	tokenKey := r.FormValue("token_key")
	if tokenKey == "" {
		http.Error(w, "token_key required", http.StatusBadRequest)
		return
	}
	if h.DB != nil {
		if err := h.DB.EnableOAuthToken(r.Context(), tokenKey); err != nil {
			log.Printf("ERROR enabling oauth token %s: %v", tokenKey, err)
			http.Error(w, "failed to persist token state", http.StatusInternalServerError)
			return
		}
	}
	if h.DisabledTokens != nil {
		h.DisabledTokens.Enable(tokenKey)
	}
	data := h.loadClaudeCodeData(r, h.dbReader())
	render(r.Context(), w, pages.UsageClaudeCodeTab(data))
}

// handleClaudeCodeAPI serves GET /ui/api/claude-code-usage (JSON for manual refresh).
func (h *UIHandler) handleClaudeCodeAPI(w http.ResponseWriter, r *http.Request) {
	data := h.loadClaudeCodeData(r, h.dbReader())
	buf, err := json.Marshal(data.Cards)
	if err != nil {
		log.Printf("ERROR marshaling claude code cards: %v", err)
		http.Error(w, "encode error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(buf)
}
