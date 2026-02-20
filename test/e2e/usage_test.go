//go:build e2e

package e2e

import (
	"testing"
	"time"

	"github.com/playwright-community/playwright-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Navigation helper ---

func (f *Fixture) NavigateToUsage() {
	f.T.Helper()
	_, err := f.Page.Goto(testServer.URL + "/ui/usage")
	require.NoError(f.T, err)
	require.NoError(f.T, f.Page.WaitForLoadState())
}

// --- US1: Cost Tab ---

func TestUsage_CostTab_EmptyState(t *testing.T) {
	f := setup(t)
	f.NavigateToUsage()

	body := f.Text("#usage-content")
	assert.Contains(t, body, "$0.00")
	assert.Contains(t, body, "Total Requests")
	assert.Contains(t, body, "No key data for selected period")
	assert.Contains(t, body, "No model data")
}

func TestUsage_CostTab_ShowsSpendData(t *testing.T) {
	f := setup(t)
	f.SeedSpendLog(SeedSpendLogOpts{Model: "openai/gpt-4o", Spend: 1.50, Tokens: 500, Prompt: 300, Completion: 200})
	f.SeedSpendLog(SeedSpendLogOpts{Model: "anthropic/claude-sonnet-4-5-20250929", Spend: 2.30, Tokens: 800, Prompt: 500, Completion: 300})
	f.NavigateToUsage()

	body := f.Text("#usage-content")
	// Daily spend chart should render (canvas exists)
	assert.True(t, f.Has("#dailySpendChart"))
	// Top keys table should have rows
	assert.Contains(t, body, "Top Virtual Keys")
}

func TestUsage_CostTab_TopKeysLimitSelector(t *testing.T) {
	f := setup(t)
	// Seed multiple keys with different spends
	for i := range 8 {
		key := f.SeedKey(SeedOpts{Alias: "key-" + string(rune('a'+i))})
		f.SeedSpendLog(SeedSpendLogOpts{ApiKey: key, Spend: float64(i+1) * 0.10, Tokens: 100})
	}
	f.NavigateToUsage()

	// Default: 5 keys
	rows := f.Count("#top-keys table tbody tr")
	assert.Equal(t, 5, rows)

	// Click "10" limit button
	require.NoError(t, f.Page.Locator("#usage-content button").Filter(playwright.LocatorFilterOptions{
		HasText: "10",
	}).First().Click())
	f.WaitStable()

	rows = f.Count("#top-keys table tbody tr")
	assert.Equal(t, 8, rows) // only 8 keys exist
}

func TestUsage_CostTab_TopModelsChart(t *testing.T) {
	f := setup(t)
	f.SeedSpendLog(SeedSpendLogOpts{Model: "openai/gpt-4o", Spend: 5.0, Tokens: 1000})
	f.SeedSpendLog(SeedSpendLogOpts{Model: "anthropic/claude-sonnet-4-5-20250929", Spend: 3.0, Tokens: 600})
	f.NavigateToUsage()

	assert.True(t, f.Has("#topModelsChart"))
}

// --- US2: Metric Cards ---

func TestUsage_MetricCards_Values(t *testing.T) {
	f := setup(t)

	// 3 successful + 1 failed
	f.SeedSpendLog(SeedSpendLogOpts{RequestID: "req-s1", Model: "openai/gpt-4o", Spend: 1.00, Tokens: 100})
	f.SeedSpendLog(SeedSpendLogOpts{RequestID: "req-s2", Model: "openai/gpt-4o", Spend: 2.00, Tokens: 200})
	f.SeedSpendLog(SeedSpendLogOpts{RequestID: "req-s3", Model: "openai/gpt-4o", Spend: 3.00, Tokens: 300})
	reqFail := f.SeedSpendLog(SeedSpendLogOpts{RequestID: "req-f1", Model: "openai/gpt-4o", Spend: 0.50, Tokens: 50})
	f.SeedErrorLog(reqFail, "openai/gpt-4o", "RateLimitError", 429)

	f.NavigateToUsage()

	body := f.Text("#usage-content")
	assert.Contains(t, body, "Total Requests")
	assert.Contains(t, body, "4")          // total requests
	assert.Contains(t, body, "3")          // successful
	assert.Contains(t, body, "1")          // failed
	assert.Contains(t, body, "650")        // total tokens
	assert.Contains(t, body, "Avg Cost")   // avg cost card present
}

// --- US3: Tab Navigation ---

func TestUsage_TabSwitch_ModelActivity(t *testing.T) {
	f := setup(t)
	f.SeedSpendLog(SeedSpendLogOpts{Model: "openai/gpt-4o", Spend: 1.0, Tokens: 100})
	f.NavigateToUsage()

	// Click Model Activity tab
	require.NoError(t, f.Page.Locator("button").Filter(playwright.LocatorFilterOptions{
		HasText: "Model Activity",
	}).Click())
	f.WaitStable()

	body := f.Text("#usage-content")
	assert.Contains(t, body, "Requests by Model")
}

func TestUsage_TabSwitch_KeyActivity(t *testing.T) {
	f := setup(t)
	f.SeedSpendLog(SeedSpendLogOpts{Model: "openai/gpt-4o", Spend: 1.0, Tokens: 100})
	f.NavigateToUsage()

	require.NoError(t, f.Page.Locator("button").Filter(playwright.LocatorFilterOptions{
		HasText: "Key Activity",
	}).Click())
	f.WaitStable()

	body := f.Text("#usage-content")
	assert.Contains(t, body, "Requests by Key")
}

func TestUsage_TabSwitch_EndpointActivity(t *testing.T) {
	f := setup(t)
	f.SeedSpendLog(SeedSpendLogOpts{Model: "openai/gpt-4o", Spend: 1.0, Tokens: 100})
	f.NavigateToUsage()

	require.NoError(t, f.Page.Locator("button").Filter(playwright.LocatorFilterOptions{
		HasText: "Endpoint Activity",
	}).Click())
	f.WaitStable()

	body := f.Text("#usage-content")
	assert.Contains(t, body, "Requests by Endpoint")
}

// --- US4: Date Range ---

func TestUsage_DateRange_PresetChange(t *testing.T) {
	f := setup(t)
	// Seed a log 15 days ago (outside 7d window)
	f.SeedSpendLog(SeedSpendLogOpts{Model: "openai/gpt-4o", Spend: 10.0, Tokens: 1000, StartAge: 15 * 24 * time.Hour})
	// Seed a log 2 days ago (inside 7d window)
	f.SeedSpendLog(SeedSpendLogOpts{Model: "openai/gpt-4o", Spend: 5.0, Tokens: 500, StartAge: 2 * 24 * time.Hour})
	f.NavigateToUsage()

	// Default 7d — should show 1 recent log
	body := f.Text("#usage-content")
	assert.Contains(t, body, "1") // 1 request in 7d window

	// Switch to 30d — should show both
	_, err := f.Page.Locator("select[name='preset']").SelectOption(playwright.SelectOptionValues{
		Values: &[]string{"30d"},
	})
	require.NoError(t, err)
	f.WaitStable()
	time.Sleep(500 * time.Millisecond)

	body = f.Text("#usage-content")
	assert.Contains(t, body, "2") // both requests
}

// --- US6: Export ---

func TestUsage_ExportButton_Exists(t *testing.T) {
	f := setup(t)
	f.NavigateToUsage()
	assert.True(t, f.Has("a[href*='/ui/usage/export']"))
}
