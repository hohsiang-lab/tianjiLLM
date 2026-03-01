package ui

// HO-81: Unit tests for the Cost Tab / Top Public Model Names handler behaviour.
// These tests run without a real DB by verifying:
//   1. loadCostTabData with DB=nil returns empty TopModels (and template renders empty state)
//   2. The rendered HTML of UsageCostTab contains expected elements
//
// NOTE: The full integration test (with real DB) is covered by the E2E tests
// in test/e2e/usage_ho81_test.go. These unit tests specifically document and
// pin the expected handler behaviour so regressions are caught early.

import (
	"bytes"
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/praxisllmlab/tianjiLLM/internal/ui/pages"
)

// TestLoadCostTabData_NilDB_TopModelsIsEmpty verifies that when DB is nil
// (no database connected), loadCostTabData returns an empty TopModels slice.
// The empty state "No model data" should be rendered instead of a blank section.
func TestLoadCostTabData_NilDB_TopModelsIsEmpty(t *testing.T) {
	h := &UIHandler{DB: nil}
	r := httptest.NewRequest("GET", "/ui/usage", nil)

	ts := pgtype.Timestamptz{}
	base := pages.UsagePageData{ActiveTab: "cost"}

	costData := h.loadCostTabData(r, base, ts, ts)

	// When DB is nil, TopModels must be nil/empty — no phantom data.
	assert.Empty(t, costData.TopModels,
		"TopModels must be empty when DB is nil; got: %+v", costData.TopModels)
}

// TestUsageCostTab_Renders_EmptyState_WhenNoModels verifies that the
// UsageCostTab templ component renders the "No model data" empty state
// when TopModels is empty.
//
// This pins the rendering expectation so we know the template is correct.
func TestUsageCostTab_Renders_EmptyState_WhenNoModels(t *testing.T) {
	data := pages.CostTabData{
		TopModels: nil, // empty
		TopKeys:   nil,
	}
	var buf bytes.Buffer
	err := pages.UsageCostTab(data).Render(context.Background(), &buf)
	require.NoError(t, err, "UsageCostTab render must not error")

	html := buf.String()
	assert.Contains(t, html, "No model data",
		`rendered HTML must contain "No model data" when TopModels is empty`)
	assert.NotContains(t, html, "topModelsChart",
		`#topModelsChart canvas must NOT appear when TopModels is empty`)
}

// TestUsageCostTab_Renders_Chart_WhenModelsExist verifies that the
// UsageCostTab templ component renders the chart canvas AND embeds the
// model names in topModelsData when TopModels is populated.
//
// Failing symptom for HO-81: even with data, the section appears blank.
// This test pins that the template itself is correct; if it fails, the
// template has a regression. If it passes but the browser still shows
// blank, the bug is upstream (DB query or handler).
func TestUsageCostTab_Renders_Chart_WhenModelsExist(t *testing.T) {
	data := pages.CostTabData{
		TopModels: []pages.TopModel{
			{Model: "openai/gpt-4o", TotalSpend: 5.0, TotalTokens: 1000, RequestCount: 3},
			{Model: "anthropic/claude-sonnet-4-5-20250929", TotalSpend: 3.0, TotalTokens: 600, RequestCount: 2},
		},
	}
	var buf bytes.Buffer
	err := pages.UsageCostTab(data).Render(context.Background(), &buf)
	require.NoError(t, err)

	html := buf.String()

	// Canvas must be rendered.
	assert.Contains(t, html, `id="topModelsChart"`,
		`#topModelsChart canvas must be present when TopModels is non-empty`)

	// JSON script must be rendered with model names.
	assert.Contains(t, html, `id="topModelsData"`,
		`topModelsData JSON script must be embedded in the page`)

	// Model names must appear in the embedded JSON data.
	assert.Contains(t, html, "openai/gpt-4o",
		`model name "openai/gpt-4o" must appear in topModelsData`)
	assert.Contains(t, html, "anthropic/claude-sonnet-4-5-20250929",
		`model name "anthropic/claude-sonnet-4-5-20250929" must appear in topModelsData`)

	// Empty state must NOT appear.
	assert.NotContains(t, html, "No model data",
		`"No model data" must not appear when TopModels is non-empty`)
}

// TestUsageCostTab_TopModels_ModelNamesNotEmpty verifies that TopModel entries
// with empty Model string are not acceptable in the rendered chart data.
//
// Failing symptom: SpendLogs.model stored as "" leads to chart with blank labels.
func TestUsageCostTab_TopModels_ModelNamesNotEmpty(t *testing.T) {
	// This simulates what happens if the DB returns rows with empty model names.
	data := pages.CostTabData{
		TopModels: []pages.TopModel{
			{Model: "", TotalSpend: 5.0, TotalTokens: 1000, RequestCount: 3},
		},
	}
	var buf bytes.Buffer
	err := pages.UsageCostTab(data).Render(context.Background(), &buf)
	require.NoError(t, err)

	html := buf.String()

	// If model name is empty, the chart would render blank bars.
	// The handler should filter out or reject empty model names.
	// This test documents the EXPECTED behaviour (should not happen),
	// and acts as a canary: if the handler allows empty model names,
	// the bug is in the data pipeline upstream.
	//
	// For now we check: does the rendered HTML contain an empty "Model" key
	// in the JSON data? It should NOT be present when the fix is applied.
	assert.NotContains(t, strings.ReplaceAll(html, `"Model":""`, "EMPTY_MODEL"),
		"EMPTY_MODEL",
		`TopModel with empty Model string must not appear in chart data (SpendLogs.model stored as empty string — data pipeline bug)`)
}
