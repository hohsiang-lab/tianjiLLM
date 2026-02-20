//go:build e2e

package e2e

import (
	"testing"
	"time"

	"github.com/playwright-community/playwright-go"
	"github.com/stretchr/testify/assert"
)

// US1 — Request Logs list with paginated table.

func TestLogsList_EmptyState(t *testing.T) {
	f := setup(t)
	f.NavigateToLogs()

	assert.Contains(t, f.Text("#logs-table"), "No request logs yet")
}

func TestLogsList_ShowsLogs(t *testing.T) {
	f := setup(t)
	f.SeedSpendLog(SeedSpendLogOpts{Model: "openai/gpt-4o", Spend: 0.05, Tokens: 150, Prompt: 100, Completion: 50})
	f.SeedSpendLog(SeedSpendLogOpts{Model: "anthropic/claude-sonnet-4-5-20250929", Spend: 0.12, Tokens: 300, Prompt: 200, Completion: 100})
	f.NavigateToLogs()

	// 2 data rows
	rows := f.Count("#logs-table table tbody tr")
	assert.Equal(t, 2, rows)

	body := f.Text("#logs-table")
	assert.Contains(t, body, "openai/gpt-4o")
	assert.Contains(t, body, "anthropic/claude-sonnet-4-5-20250929")
	assert.Contains(t, body, "Success")
}

func TestLogsList_FailedStatus(t *testing.T) {
	f := setup(t)

	// Seed a successful request
	f.SeedSpendLog(SeedSpendLogOpts{RequestID: "req-ok-1", Model: "openai/gpt-4o"})

	// Seed a failed request (SpendLog + ErrorLog)
	reqID := f.SeedSpendLog(SeedSpendLogOpts{RequestID: "req-fail-1", Model: "openai/gpt-4o"})
	f.SeedErrorLog(reqID, "openai/gpt-4o", "AuthenticationError", 401)

	f.NavigateToLogs()

	body := f.Text("#logs-table")
	assert.Contains(t, body, "Success")
	assert.Contains(t, body, "Failed")
}

func TestLogsList_NullValuesShowDash(t *testing.T) {
	f := setup(t)
	// Log with zero spend, no team, no tokens
	f.SeedSpendLog(SeedSpendLogOpts{Model: "openai/gpt-4o", Spend: 0, Tokens: 0})
	f.NavigateToLogs()

	// Table should render dashes for zero values
	body := f.Text("#logs-table")
	assert.Contains(t, body, "–")
}

// US2 — Filters

func TestLogsList_FilterByStatus(t *testing.T) {
	f := setup(t)

	f.SeedSpendLog(SeedSpendLogOpts{RequestID: "req-s1", Model: "openai/gpt-4o"})
	reqFail := f.SeedSpendLog(SeedSpendLogOpts{RequestID: "req-f1", Model: "openai/gpt-4o"})
	f.SeedErrorLog(reqFail, "openai/gpt-4o", "RateLimitError", 429)

	f.NavigateToLogs()

	// Open filter panel
	f.ClickButton("Filters")
	time.Sleep(200 * time.Millisecond)

	// Filter by "failed"
	f.SelectLogFilter("status", "failed")
	time.Sleep(500 * time.Millisecond)
	f.WaitStable()

	rows := f.Count("#logs-table table tbody tr")
	assert.Equal(t, 1, rows)
	assert.Contains(t, f.Text("#logs-table"), "Failed")
	assert.NotContains(t, f.Text("#logs-table"), "req-s1")
}

func TestLogsList_FilterByModel(t *testing.T) {
	f := setup(t)

	f.SeedSpendLog(SeedSpendLogOpts{Model: "openai/gpt-4o"})
	f.SeedSpendLog(SeedSpendLogOpts{Model: "anthropic/claude-sonnet-4-5-20250929"})
	f.SeedSpendLog(SeedSpendLogOpts{Model: "openai/gpt-4o"})

	f.NavigateToLogs()

	// Open filter panel and filter by model
	f.ClickButton("Filters")
	time.Sleep(200 * time.Millisecond)
	f.FilterLogs("model", "anthropic/claude-sonnet-4-5-20250929")
	time.Sleep(500 * time.Millisecond)
	f.WaitStable()

	rows := f.Count("#logs-table table tbody tr")
	assert.Equal(t, 1, rows)
	assert.Contains(t, f.Text("#logs-table"), "anthropic/claude-sonnet-4-5-20250929")
}

func TestLogsList_SearchByRequestID(t *testing.T) {
	f := setup(t)

	f.SeedSpendLog(SeedSpendLogOpts{RequestID: "req-abc-123", Model: "openai/gpt-4o"})
	f.SeedSpendLog(SeedSpendLogOpts{RequestID: "req-def-456", Model: "openai/gpt-4o"})

	f.NavigateToLogs()

	// Search by request ID
	f.FilterLogs("request_id", "req-abc-123")
	time.Sleep(500 * time.Millisecond)
	f.WaitStable()

	rows := f.Count("#logs-table table tbody tr")
	assert.Equal(t, 1, rows)
	assert.Contains(t, f.Text("#logs-table"), "req-abc-123")
}

func TestLogsList_FilterNoMatch(t *testing.T) {
	f := setup(t)
	f.SeedSpendLog(SeedSpendLogOpts{Model: "openai/gpt-4o"})
	f.NavigateToLogs()

	f.FilterLogs("request_id", "nonexistent-request-id")
	time.Sleep(500 * time.Millisecond)
	f.WaitStable()

	assert.Contains(t, f.Text("#logs-table"), "No results matching your filters")
}

func TestLogsList_TimeRangeFilter(t *testing.T) {
	f := setup(t)

	// Recent log (within 1h)
	f.SeedSpendLog(SeedSpendLogOpts{RequestID: "req-recent", Model: "openai/gpt-4o", StartAge: 30 * time.Minute})
	// Old log (25 hours ago, outside 24h)
	f.SeedSpendLog(SeedSpendLogOpts{RequestID: "req-old", Model: "openai/gpt-4o", StartAge: 25 * time.Hour})

	f.NavigateToLogs()

	// Default is 24h — should see only recent
	rows := f.Count("#logs-table table tbody tr")
	assert.Equal(t, 1, rows)
	assert.Contains(t, f.Text("#logs-table"), "req-recent")

	// Switch to 7d — should see both
	f.SelectLogFilter("time_range", "7d")
	time.Sleep(500 * time.Millisecond)
	f.WaitStable()

	rows = f.Count("#logs-table table tbody tr")
	assert.Equal(t, 2, rows)
}

// US3 — Live Tail

func TestLogsList_LiveTailToggle(t *testing.T) {
	f := setup(t)
	f.SeedSpendLog(SeedSpendLogOpts{Model: "openai/gpt-4o"})
	f.NavigateToLogs()

	// Live Tail should be off initially — no green indicator
	assert.NotContains(t, f.Text("#logs-table"), "Auto-refreshing every 15 seconds")

	// Click Live button to enable
	f.ClickButton("Live")
	f.WaitStable()

	// Should now see the Live Tail indicator
	assert.Contains(t, f.Text("#logs-table"), "Auto-refreshing every 15 seconds")

	// URL should contain live_tail=true
	assert.Contains(t, f.URL(), "live_tail=true")
}

func TestLogsList_SidebarNavigation(t *testing.T) {
	f := setup(t)

	// Navigate to Logs via sidebar link
	_, err := f.Page.Goto(testServer.URL + "/ui/")
	assert.NoError(t, err)
	assert.NoError(t, f.Page.WaitForLoadState())

	// Use GetByRole with exact name match to find the visible sidebar link
	assert.NoError(t, f.Page.GetByRole("link", playwright.PageGetByRoleOptions{
		Name:  "Logs",
		Exact: playwright.Bool(true),
	}).First().Click())
	assert.NoError(t, f.Page.WaitForLoadState())

	assert.Contains(t, f.URL(), "/ui/logs")
	assert.Contains(t, f.Text("h1"), "Request Logs")
}
