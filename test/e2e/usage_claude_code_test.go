//go:build e2e

package e2e

// Tests for loadClaudeCodeData DB fallback path.
//
// loadClaudeCodeData reads rate-limit state from InMemoryRateLimitStore first,
// then falls back to DB.GetOAuthTokenRateLimitState when the store has no entry
// for a token (e.g. after server restart or TTL eviction).
// In the E2E setup RateLimitStore is not wired on uiHandler, so DB fallback is
// always exercised here.
//
// NormalizeExpiredWindows is applied on the DB row before rendering, matching
// the behavior of InMemoryRateLimitStore.Get() (HO-568).

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/mxschmitt/playwright-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/praxisllmlab/tianjiLLM/internal/callback"
	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/praxisllmlab/tianjiLLM/internal/db"
)

// TestClaudeCode_ShowsUtilization_AfterCacheEviction verifies that when
// OAuthTokenRateLimitState data exists in the DB and the in-memory store has
// no entry (simulating restart / TTL eviction), the Claude Code tab displays
// the persisted utilization percentages via the DB fallback path.
func TestClaudeCode_ShowsUtilization_AfterCacheEviction(t *testing.T) {
	ctx := context.Background()

	// A minimal OAuth token — IsOAuthToken() only checks the prefix.
	const oauthToken = "sk-ant-oat-e2e-cache-eviction-test-token"
	tokenKey := callback.RateLimitCacheKey(oauthToken)

	// --- setup: add the token to cfg.ModelList temporarily ---
	apiKey := oauthToken
	originalModelList := cfg.ModelList
	cfg.ModelList = append(cfg.ModelList, config.ModelConfig{
		ModelName: "e2e-claude-code-test",
		TianjiParams: config.TianjiParams{
			Model:  "anthropic/claude-sonnet-4-5-20250929",
			APIKey: &apiKey,
		},
	})
	t.Cleanup(func() { cfg.ModelList = originalModelList })

	// setup(t) calls cleanDB which wipes OAuthTokenRateLimitState — seed AFTER.
	f := setup(t)

	require.NoError(t, testDB.UpsertOAuthTokenRateLimitState(ctx, db.UpsertOAuthTokenRateLimitStateParams{
		TokenKey:                   tokenKey,
		UnifiedStatus:              "allowed",
		Unified5hStatus:            "allowed",
		Unified5hUtilization:       0.42, // 42 % of 5h window used
		Unified5hReset:             "1999999999",
		Unified7dStatus:            "allowed",
		Unified7dUtilization:       0.30, // 30 % of 7d window used
		Unified7dReset:             "1999999999",
		Unified7dSonnetStatus:      "",
		Unified7dSonnetUtilization: -1,
		Unified7dSonnetReset:       "",
		OrgID:                      "org-e2e-test",
	}))
	f.NavigateToUsage()

	require.NoError(t, f.Page.Locator(`button[data-tab="claude-code"]`).Click())
	f.WaitStable()

	// Wait for the Claude Code tab content to render.
	require.NoError(t, f.Page.Locator("#claude-code-tab-content").WaitFor(playwright.LocatorWaitForOptions{
		Timeout: playwright.Float(defaultWaitTimeout),
	}))

	body := f.Text("#claude-code-tab-content")

	// The card for our token must be present (token key appears in the header).
	// This is a precondition for the percentage assertions below.
	require.Contains(t, body, tokenKey,
		"token card must appear in the Claude Code tab")

	assert.NotContains(t, body, "No data yet",
		"Claude Code tab must not show 'No data yet' when DB has utilization data")

	assert.Contains(t, body, "42% used",
		"5h utilization (42%) must be visible when data exists in DB")
	assert.Contains(t, body, "30% used",
		"7d utilization (30%) must be visible when data exists in DB")
}

// TestClaudeCode_ExpiredWindow_ShowsZero_NotStaleValue verifies HO-568:
// when the DB fallback path is used and the 5h rate-limit window has already
// expired, the progress bar must display 0% (reset) rather than the stale
// pre-reset utilization value stored in the DB.
//
// This exercises the NormalizeExpiredWindows call added to handler_claude_code.go.
// RateLimitStore is not set on uiHandler in the E2E setup, so DB fallback is
// always used — no extra emptying required.
func TestClaudeCode_ExpiredWindow_ShowsZero_NotStaleValue(t *testing.T) {
	ctx := context.Background()

	const oauthToken = "sk-ant-oat-e2e-ho568-expired-window-token"
	tokenKey := callback.RateLimitCacheKey(oauthToken)

	// Add token to cfg.ModelList so enumerateOAuthTokens picks it up.
	apiKey := oauthToken
	originalModelList := cfg.ModelList
	cfg.ModelList = append(cfg.ModelList, config.ModelConfig{
		ModelName: "e2e-ho568-test",
		TianjiParams: config.TianjiParams{
			Model:  "anthropic/claude-sonnet-4-5-20250929",
			APIKey: &apiKey,
		},
	})
	t.Cleanup(func() { cfg.ModelList = originalModelList })

	// setup(t) calls cleanDB which wipes OAuthTokenRateLimitState — seed AFTER.
	f := setup(t)

	// 5h window expired 1 hour ago; 7d window still active (6 days out).
	expiredReset := strconv.FormatInt(time.Now().Add(-1*time.Hour).Unix(), 10)
	futureReset := strconv.FormatInt(time.Now().Add(6*24*time.Hour).Unix(), 10)

	require.NoError(t, testDB.UpsertOAuthTokenRateLimitState(ctx, db.UpsertOAuthTokenRateLimitStateParams{
		TokenKey:                   tokenKey,
		UnifiedStatus:              "allowed",
		Unified5hStatus:            "allowed",
		Unified5hUtilization:       0.92, // stale — window has already reset
		Unified5hReset:             expiredReset,
		Unified7dStatus:            "allowed",
		Unified7dUtilization:       0.30, // still active
		Unified7dReset:             futureReset,
		Unified7dSonnetStatus:      "",
		Unified7dSonnetUtilization: -1,
		Unified7dSonnetReset:       "",
		OrgID:                      "org-e2e-ho568",
	}))
	f.NavigateToUsage()

	require.NoError(t, f.Page.Locator(`button[data-tab="claude-code"]`).Click())
	f.WaitStable()

	require.NoError(t, f.Page.Locator("#claude-code-tab-content").WaitFor(playwright.LocatorWaitForOptions{
		Timeout: playwright.Float(defaultWaitTimeout),
	}))

	body := f.Text("#claude-code-tab-content")

	// The card for our token must appear — precondition for percentage assertions.
	require.Contains(t, body, tokenKey,
		"token card must be present in the Claude Code tab")

	// Stale pre-reset value must NOT appear (HO-568 regression guard).
	// Use "92% used" (not bare "92%") to match the rendered label precisely.
	assert.NotContains(t, body, "92% used",
		"stale 5h utilization (92%) must not appear after window expiry")

	// 5h window expired → NormalizeExpiredWindows zeroes it; renders as "0% used".
	// Use "0% used" (not bare "0%") to avoid false positive — "30%" contains "0%".
	assert.Contains(t, body, "0% used",
		"expired 5h window must render as 0%% after normalization")

	// 7d window still active → value preserved.
	assert.Contains(t, body, "30%",
		"active 7d utilization (30%) must be preserved and visible")
}
