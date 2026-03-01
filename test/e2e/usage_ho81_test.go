//go:build e2e

package e2e

// HO-81: bug: Usage 頁 Top Public Model Names 區塊空白且無 empty state
//
// Root cause hypothesis: The chart section renders a <canvas> when
// TopModels is non-empty, but the model names are either:
//   (a) stored as empty string in SpendLogs.model, so the chart has
//       empty-string labels (visually blank bars with no labels), OR
//   (b) the topModelsData JSON script is missing / malformed, causing
//       Chart.js to silently fail and leave the canvas blank.
//
// These tests are written to FAIL until the bug is fixed.

import (
	"encoding/json"
	"testing"

	"github.com/playwright-community/playwright-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// filterByText is a helper to create a playwright LocatorFilterOptions.
func filterByText(text string) playwright.LocatorFilterOptions {
	return playwright.LocatorFilterOptions{HasText: text}
}

// TestHO81_TopModels_ModelNameInEmbeddedJSON verifies that when spend logs
// exist with a known model name, that name is embedded in the topModelsData
// JSON script on the page.
//
// Failing symptom: the model name is absent from the JSON data (empty string
// stored in SpendLogs.model or GetTopModelsBySpend returns no rows).
func TestHO81_TopModels_ModelNameInEmbeddedJSON(t *testing.T) {
	f := setup(t)
	const wantModel = "openai/gpt-4o"
	f.SeedSpendLog(SeedSpendLogOpts{Model: wantModel, Spend: 5.0, Tokens: 1000})
	f.NavigateToUsage()

	// The topModelsData <script type="application/json"> element must exist.
	require.True(t, f.Has("#topModelsData"),
		"topModelsData JSON script element must be present on the page")

	// Read the embedded JSON.
	raw, err := f.Page.Locator("#topModelsData").TextContent()
	require.NoError(t, err, "failed to read topModelsData element content")
	require.NotEmpty(t, raw, "topModelsData must not be empty")

	// Decode and verify at least one entry has a non-empty Model field
	// matching the seeded model name.
	type topModelEntry struct {
		Model string
	}
	var entries []topModelEntry
	require.NoError(t, json.Unmarshal([]byte(raw), &entries),
		"topModelsData JSON must be valid")

	require.NotEmpty(t, entries, "topModelsData must contain at least one entry")

	found := false
	for _, e := range entries {
		if e.Model == wantModel {
			found = true
			break
		}
	}
	assert.True(t, found,
		"expected model %q to appear in topModelsData, got entries: %+v", wantModel, entries)
}

// TestHO81_TopModels_NoEmptyLabelWhenDataExists verifies that the chart data
// does NOT contain empty-string model names when spend logs are seeded.
//
// Failing symptom: SpendLogs.model is stored as "" so the chart renders
// blank (empty-label) bars.
func TestHO81_TopModels_NoEmptyLabelWhenDataExists(t *testing.T) {
	f := setup(t)
	f.SeedSpendLog(SeedSpendLogOpts{Model: "anthropic/claude-sonnet-4-5-20250929", Spend: 3.0, Tokens: 600})
	f.NavigateToUsage()

	require.True(t, f.Has("#topModelsData"),
		"topModelsData JSON script element must be present on the page")

	raw, err := f.Page.Locator("#topModelsData").TextContent()
	require.NoError(t, err)
	require.NotEmpty(t, raw)

	type topModelEntry struct {
		Model string
	}
	var entries []topModelEntry
	require.NoError(t, json.Unmarshal([]byte(raw), &entries))

	for _, e := range entries {
		assert.NotEmpty(t, e.Model,
			"topModelsData must not contain entries with empty model name; got: %+v", entries)
	}
}

// TestHO81_TopModels_EmptyStateNotShownWhenDataExists verifies that when
// spend logs exist, the "No model data" empty state placeholder is hidden
// and the chart canvas is shown instead.
//
// Failing symptom: neither the chart nor the empty-state text is visible
// (the whole section appears blank).
func TestHO81_TopModels_EmptyStateNotShownWhenDataExists(t *testing.T) {
	f := setup(t)
	f.SeedSpendLog(SeedSpendLogOpts{Model: "openai/gpt-4o", Spend: 5.0, Tokens: 1000})
	f.NavigateToUsage()

	// Canvas must be present.
	assert.True(t, f.Has("#topModelsChart"),
		"#topModelsChart canvas must be present when data exists")

	// "No model data" empty state must NOT appear.
	body := f.Text("#usage-content")
	assert.NotContains(t, body, "No model data",
		`"No model data" empty state must not appear when there is spend data`)
}

// TestHO81_TopModels_MultipleModels verifies that multiple distinct model
// names all appear in topModelsData when spend logs from different models
// are seeded.
func TestHO81_TopModels_MultipleModels(t *testing.T) {
	f := setup(t)
	models := []string{
		"openai/gpt-4o",
		"anthropic/claude-sonnet-4-5-20250929",
		"openai/gpt-4o-mini",
	}
	for i, m := range models {
		f.SeedSpendLog(SeedSpendLogOpts{
			Model:  m,
			Spend:  float64(i+1) * 1.5,
			Tokens: int32((i + 1) * 300),
		})
	}
	f.NavigateToUsage()

	require.True(t, f.Has("#topModelsData"),
		"topModelsData JSON script element must be present on the page")

	raw, err := f.Page.Locator("#topModelsData").TextContent()
	require.NoError(t, err)
	require.NotEmpty(t, raw)

	// All three model names must appear in the embedded JSON.
	for _, m := range models {
		assert.Contains(t, raw, m,
			"expected model %q to be present in topModelsData JSON", m)
	}
}

// TestHO81_TopModels_TabSwitch_PreservesData verifies that after switching
// away from and back to the Cost tab via HTMX, the Top Models chart data
// is still present and correct.
//
// Failing symptom: tab switching clears or fails to re-populate
// the Top Models section.
func TestHO81_TopModels_TabSwitch_PreservesData(t *testing.T) {
	f := setup(t)
	const wantModel = "openai/gpt-4o"
	f.SeedSpendLog(SeedSpendLogOpts{Model: wantModel, Spend: 5.0, Tokens: 1000})
	f.NavigateToUsage()

	// Switch away to Model Activity tab.
	err := f.Page.Locator("button").Filter(
		playwright.LocatorFilterOptions{HasText: "Model Activity"},
	).Click()
	require.NoError(t, err)
	f.WaitStable()

	// Switch back to Cost tab (first button with text "Cost").
	err = f.Page.Locator("button").Filter(
		playwright.LocatorFilterOptions{HasText: "Cost"},
	).First().Click()
	require.NoError(t, err)
	f.WaitStable()

	// topModelsData must still be present and contain the model name.
	require.True(t, f.Has("#topModelsData"),
		"topModelsData must be present after switching back to Cost tab")

	raw, err := f.Page.Locator("#topModelsData").TextContent()
	require.NoError(t, err)
	assert.Contains(t, raw, wantModel,
		"model name must still appear in topModelsData after tab switch")
}
