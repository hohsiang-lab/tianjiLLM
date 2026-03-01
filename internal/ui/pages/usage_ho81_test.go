package pages_test

// HO-81: Failing tests for Top Public Model Names blank in usage page.
//
// Root cause: proxy handler streaming response does not capture model name;
// SpendLogs.model field is stored as empty string; GetTopModelsBySpend groups
// by model and returns rows with Model="", which renders as blank in the chart.
//
// Expected fixes:
//  1. Template: when TopModels is empty (or all entries have empty model name),
//     render a visible "No model data" empty state.
//  2. Template/handler: entries with Model=="" should not appear as a blank
//     label; they should be rendered with fallback text "(unknown model)".
//  3. Chart.js <script> must appear before the canvas IIFE (same as HO-80).
//
// These tests are intentionally FAILING on main to guide the implementation.

import (
	"strings"
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/ui/pages"
)

// buildCostTabData is a helper that returns a minimal CostTabData with the
// provided TopModels slice.
func buildCostTabData(models []pages.TopModel) pages.CostTabData {
	return pages.CostTabData{
		UsagePageData: pages.UsagePageData{
			ActiveTab: "cost",
			StartDate: "2025-01-01",
			EndDate:   "2025-01-07",
			Preset:    "7d",
		},
		TopKeyLimit: 5,
		TopModels:   models,
	}
}

// TestTopModels_EmptyState_ShowsNoModelData verifies that when TopModels is
// empty the cost tab renders a human-readable "No model data" empty state.
//
// Current behavior (BUG): the section is blank — no canvas, no message.
// Expected: visible text containing "No model data" is present in the HTML.
func TestTopModels_EmptyState_ShowsNoModelData(t *testing.T) {
	html := renderToString(t, pages.UsageCostTab(buildCostTabData(nil)))

	if !strings.Contains(html, "No model data") {
		t.Errorf(
			"UsageCostTab with empty TopModels does not contain \"No model data\".\n"+
				"Expected a visible empty-state message when there are no model entries.\n"+
				"HTML:\n%s", html,
		)
	}
}

// TestTopModels_EmptyModelName_RendersAsFallback verifies that when a TopModel
// entry has an empty Model string the rendered HTML shows a meaningful fallback
// label (e.g. "(unknown model)") instead of a blank label.
//
// Current behavior (BUG): the blank model name propagates through to the chart
// JSON, resulting in an invisible or blank axis label.
// Expected: empty Model is replaced with "(unknown model)" in the rendered HTML.
func TestTopModels_EmptyModelName_RendersAsFallback(t *testing.T) {
	models := []pages.TopModel{
		{Model: "", TotalSpend: 2.00, TotalTokens: 500, RequestCount: 5},
	}
	html := renderToString(t, pages.UsageCostTab(buildCostTabData(models)))

	if !strings.Contains(html, "(unknown model)") {
		t.Errorf(
			"UsageCostTab with empty Model name does not render \"(unknown model)\" fallback.\n"+
				"Expected blank model names to be replaced with a meaningful label.\n"+
				"HTML:\n%s", html,
		)
	}
}

// TestTopModels_WithData_RendersModelNamesAsText verifies that when TopModels
// contains valid entries the model names appear as readable text in the HTML
// so they are accessible even if Chart.js fails to initialize (HO-80).
//
// Current behavior (BUG): model names are only embedded inside a <script> JSON
// blob and a <canvas> element — invisible when Chart.js is absent.
// Expected: model names appear as visible text outside of <script> tags.
func TestTopModels_WithData_RendersModelNamesAsText(t *testing.T) {
	const modelName = "anthropic/claude-3-5-sonnet"

	models := []pages.TopModel{
		{Model: modelName, TotalSpend: 3.14, TotalTokens: 800, RequestCount: 4},
	}
	html := renderToString(t, pages.UsageCostTab(buildCostTabData(models)))

	// Strip <script> blocks and check that the model name still appears.
	htmlWithoutScripts := stripScriptTagsHO81(html)
	if !strings.Contains(htmlWithoutScripts, modelName) {
		t.Errorf(
			"UsageCostTab with model %q does not render the name as visible text "+
				"(outside <script> tags).\n"+
				"Model names must be accessible when Chart.js fails to load.\n"+
				"HTML (scripts stripped):\n%s", modelName, htmlWithoutScripts,
		)
	}
}

// TestTopModels_ChartJsBeforeTopModelsCanvas verifies that chart.min.js is
// loaded before the topModelsChart canvas / IIFE so that Chart is defined when
// the inline script executes (same class of bug as HO-80 for dailySpendChart).
//
// Current behavior (BUG): chart.min.js is loaded after the inline script,
// causing a ReferenceError: Chart is not defined.
// Expected: the <script src="chart.min.js"> tag appears before the first
// reference to topModelsChart in the HTML.
func TestTopModels_ChartJsBeforeTopModelsCanvas(t *testing.T) {
	models := []pages.TopModel{
		{Model: "openai/gpt-4o", TotalSpend: 5.00, TotalTokens: 1000, RequestCount: 10},
	}

	// Render the full page so layout.templ <body> is included.
	data := pages.UsagePageData{
		ActiveTab: "cost",
		StartDate: "2025-01-01",
		EndDate:   "2025-01-07",
		Preset:    "7d",
	}
	tab := buildCostTabData(models)
	html := renderToString(t, pages.UsagePage(data, tab, nil))

	chartJsIdx := strings.Index(html, "chart.min.js")
	topModelsIdx := strings.Index(html, "topModelsChart")

	if chartJsIdx == -1 {
		t.Fatal("chart.min.js not found in rendered HTML")
	}
	if topModelsIdx == -1 {
		t.Fatal("topModelsChart reference not found in rendered HTML")
	}
	if chartJsIdx > topModelsIdx {
		t.Errorf(
			"chart.min.js (pos %d) appears AFTER topModelsChart canvas/script (pos %d).\n"+
				"Chart.js must be loaded before the inline IIFE calls new Chart(...).\n"+
				"This causes: ReferenceError: Chart is not defined.",
			chartJsIdx, topModelsIdx,
		)
	}
}

// TestTopVirtualKeys_NotAffectedByTopModelsChange verifies that Top Virtual
// Keys table continues to render correctly regardless of TopModels content.
//
// Regression guard: the HO-81 fix must not break the keys table.
func TestTopVirtualKeys_NotAffectedByTopModelsChange(t *testing.T) {
	models := []pages.TopModel{
		{Model: "", TotalSpend: 1.00, TotalTokens: 100, RequestCount: 1},
	}
	html := renderToString(t, pages.UsageCostTab(buildCostTabData(models)))

	if !strings.Contains(html, "Top Virtual Keys") {
		t.Errorf(
			"UsageCostTab does not render \"Top Virtual Keys\" section.\n"+
				"The HO-81 fix must not remove or break the top keys table.\n"+
				"HTML:\n%s", html,
		)
	}
}

// stripScriptTagsHO81 removes all content between <script ...> and </script> tags.
func stripScriptTagsHO81(html string) string {
	var result strings.Builder
	remaining := html
	for {
		start := strings.Index(remaining, "<script")
		if start == -1 {
			result.WriteString(remaining)
			break
		}
		result.WriteString(remaining[:start])
		end := strings.Index(remaining[start:], "</script>")
		if end == -1 {
			break
		}
		remaining = remaining[start+end+len("</script>"):]
	}
	return result.String()
}
