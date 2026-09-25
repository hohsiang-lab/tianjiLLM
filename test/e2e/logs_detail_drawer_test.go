//go:build e2e

package e2e

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Smoke tests for log detail drawer overlay behavior.

func TestLogsDrawer_OpensOnRowClick(t *testing.T) {
	f := setup(t)
	f.SeedSpendLog(SeedSpendLogOpts{
		RequestID:       "req-drawer-open",
		Model:           "gpt-5.6-sol",
		ReasoningEffort: "xhigh",
		Spend:           0.05,
		Tokens:          150,
		Prompt:          100,
		Completion:      50,
	})
	f.NavigateToLogs()

	// Drawer should be off-screen initially
	panel := f.Page.Locator("#log-detail-panel")
	cls, err := panel.GetAttribute("class")
	require.NoError(t, err)
	assert.Contains(t, cls, "translate-x-full", "drawer should be hidden initially")

	// Click the log row
	require.NoError(t, f.Page.Locator("#logs-table table tbody tr").First().Click())
	time.Sleep(500 * time.Millisecond)
	f.WaitStable()

	// Drawer should slide in (translate-x-full removed)
	cls, err = panel.GetAttribute("class")
	require.NoError(t, err)
	assert.NotContains(t, cls, "translate-x-full", "drawer should be visible after click")

	// Backdrop should be visible
	backdrop := f.Page.Locator("#log-detail-backdrop")
	bCls, err := backdrop.GetAttribute("class")
	require.NoError(t, err)
	assert.NotContains(t, bCls, "hidden", "backdrop should be visible")

	// Drawer should contain log details
	content, err := panel.TextContent()
	require.NoError(t, err)
	assert.Contains(t, content, "gpt-5.6-sol(xhigh)")
	assert.Contains(t, content, "Request Details")
	assert.Contains(t, content, "req-drawer-open")
}

func TestLogsDrawer_CloseViaXButton(t *testing.T) {
	f := setup(t)
	f.SeedSpendLog(SeedSpendLogOpts{RequestID: "req-close-x", Model: "openai/gpt-4o"})
	f.NavigateToLogs()

	// Open drawer
	require.NoError(t, f.Page.Locator("#logs-table table tbody tr").First().Click())
	time.Sleep(500 * time.Millisecond)
	f.WaitStable()

	// Click the X close button inside the drawer (aria-label="Close")
	require.NoError(t, f.Page.Locator(`#log-detail-panel button[aria-label="Close"]`).Click())
	time.Sleep(400 * time.Millisecond)

	// Drawer should be off-screen again
	cls, err := f.Page.Locator("#log-detail-panel").GetAttribute("class")
	require.NoError(t, err)
	assert.Contains(t, cls, "translate-x-full", "drawer should be hidden after close")

	// Backdrop should be hidden
	bCls, err := f.Page.Locator("#log-detail-backdrop").GetAttribute("class")
	require.NoError(t, err)
	assert.Contains(t, bCls, "hidden", "backdrop should be hidden after close")
}

func TestLogsDrawer_CloseViaBackdrop(t *testing.T) {
	f := setup(t)
	f.SeedSpendLog(SeedSpendLogOpts{RequestID: "req-close-bd", Model: "openai/gpt-4o"})
	f.NavigateToLogs()

	// Open drawer
	require.NoError(t, f.Page.Locator("#logs-table table tbody tr").First().Click())
	time.Sleep(500 * time.Millisecond)
	f.WaitStable()

	// Click the backdrop to close (use JS to avoid Playwright visibility/z-index issues)
	_, err := f.Page.Evaluate("() => document.getElementById('log-detail-backdrop').click()")
	require.NoError(t, err)
	time.Sleep(400 * time.Millisecond)

	// Drawer should be hidden
	cls, err := f.Page.Locator("#log-detail-panel").GetAttribute("class")
	require.NoError(t, err)
	assert.Contains(t, cls, "translate-x-full", "drawer should close on backdrop click")
}

func TestLogsDrawer_TableRemainsFullWidth(t *testing.T) {
	f := setup(t)
	f.SeedSpendLog(SeedSpendLogOpts{Model: "openai/gpt-4o"})
	f.NavigateToLogs()

	// Get table width before opening drawer
	widthBefore, err := f.Page.Locator("#logs-split-container").Evaluate(
		"el => el.getBoundingClientRect().width", nil,
	)
	require.NoError(t, err)

	// Open drawer
	require.NoError(t, f.Page.Locator("#logs-table table tbody tr").First().Click())
	time.Sleep(500 * time.Millisecond)
	f.WaitStable()

	// Table width should not shrink (drawer overlays, doesn't push)
	widthAfter, err := f.Page.Locator("#logs-split-container").Evaluate(
		"el => el.getBoundingClientRect().width", nil,
	)
	require.NoError(t, err)

	assert.InDelta(t, widthBefore, widthAfter, 1.0,
		"table width should not change when drawer opens (overlay, not push)")
}

func TestLogsDrawer_ShowsFailedRequestDetails(t *testing.T) {
	f := setup(t)
	reqID := f.SeedSpendLog(SeedSpendLogOpts{RequestID: "req-fail-drawer", Model: "openai/gpt-4o"})
	f.SeedErrorLog(reqID, "openai/gpt-4o", "RateLimitError", 429)
	f.NavigateToLogs()

	// Click the failed log row
	require.NoError(t, f.Page.Locator("#logs-table table tbody tr").First().Click())
	time.Sleep(500 * time.Millisecond)
	f.WaitStable()

	content, err := f.Page.Locator("#log-detail-panel").TextContent()
	require.NoError(t, err)
	assert.Contains(t, content, "Failed")
	assert.Contains(t, content, "Request Failed")
	assert.Contains(t, content, "RateLimitError")
}
