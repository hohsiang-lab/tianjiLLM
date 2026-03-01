package pages_test

// HO-80: Failing tests for Daily Spend chart ReferenceError bug.
//
// Root Cause: chart.min.js is loaded AFTER { children... } in layout.templ,
// so the dailySpendChartScript IIFE executes before Chart is defined → ReferenceError.
//
// These tests document the expected behavior AFTER the fix.
// They are intentionally FAILING on the current (unfixed) codebase.

import (
	"context"
	"strings"
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/ui/pages"
)

// costTabPage renders a full UsagePage with CostTabData and returns the HTML.
// We render the whole page so that layout.templ's <body> is included and we can
// inspect the relative order of <script src="chart.min.js"> vs the IIFE.
func costTabPage(t *testing.T, dailySpend []pages.DailySpend) string {
	t.Helper()

	data := pages.UsagePageData{
		ActiveTab: "cost",
		StartDate: "2026-02-17",
		EndDate:   "2026-02-24",
		Preset:    "7d",
	}
	tab := pages.CostTabData{
		UsagePageData: data,
		Metrics:       pages.UsageMetrics{},
		DailySpend:    dailySpend,
		TopKeyLimit:   5,
	}

	comp := pages.UsagePage(data, tab, nil)

	var buf strings.Builder
	if err := comp.Render(context.Background(), &buf); err != nil {
		t.Fatalf("render failed: %v", err)
	}
	return buf.String()
}

// ─── AC1 / AC2: chart.min.js must load before the dailySpendChartScript IIFE ──

// TestDailySpendChart_ScriptOrder_ChartJsBeforeIIFE verifies that chart.min.js
// appears in HTML BEFORE the inline IIFE that calls `new Chart(...)`.
//
// Current behavior (BUG): chart.min.js is appended after { children... }, so its
// position in the HTML is AFTER the IIFE → Chart undefined → ReferenceError.
func TestDailySpendChart_ScriptOrder_ChartJsBeforeIIFE(t *testing.T) {
	spend := []pages.DailySpend{
		{Date: "2026-02-17", Spend: 1.23},
		{Date: "2026-02-18", Spend: 4.56},
	}
	html := costTabPage(t, spend)

	chartJsIdx := strings.Index(html, "chart.min.js")
	iifeIdx := strings.Index(html, "new Chart(")

	if chartJsIdx == -1 {
		t.Fatal("chart.min.js script tag not found in rendered HTML")
	}
	if iifeIdx == -1 {
		t.Fatal("'new Chart(' IIFE not found in rendered HTML")
	}

	if chartJsIdx >= iifeIdx {
		t.Errorf(
			"chart.min.js (offset %d) appears AFTER 'new Chart(' (offset %d);\n"+
				"chart.min.js must be loaded BEFORE any script that references Chart.\n"+
				"This is the root cause of the ReferenceError in HO-80.",
			chartJsIdx, iifeIdx,
		)
	}
}

// TestDailySpendChart_ChartJsBeforeFirstCanvas verifies that chart.min.js appears
// before the first <canvas in the document. This ensures the library is available
// when the IIFE that initializes the chart runs (AC3: tab-switch stability).
func TestDailySpendChart_ChartJsBeforeFirstCanvas(t *testing.T) {
	spend := []pages.DailySpend{{Date: "2026-02-17", Spend: 2.0}}
	html := costTabPage(t, spend)

	chartJsIdx := strings.Index(html, "chart.min.js")
	canvasIdx := strings.Index(html, "<canvas")

	if chartJsIdx == -1 {
		t.Fatal("chart.min.js not found in rendered HTML")
	}
	if canvasIdx == -1 {
		t.Fatal("<canvas element not found in rendered HTML")
	}

	if chartJsIdx >= canvasIdx {
		t.Errorf(
			"chart.min.js (offset %d) appears AFTER first <canvas (offset %d);\n"+
				"Fix: move <script src=chart.min.js> to <head> or before { children... }.",
			chartJsIdx, canvasIdx,
		)
	}
}

// ─── AC4: DailySpend with data (including all-zero) renders chart canvas ────

// TestDailySpendChart_WithData_RendersCanvas verifies that when DailySpend has
// entries, the canvas element is rendered (not the "No spend data" empty state).
func TestDailySpendChart_WithData_RendersCanvas(t *testing.T) {
	spend := []pages.DailySpend{
		{Date: "2026-02-17", Spend: 1.23},
		{Date: "2026-02-18", Spend: 0.00},
	}
	html := costTabPage(t, spend)

	if strings.Contains(html, "No spend data for selected period") {
		t.Error("DailySpend has entries but rendered 'No spend data for selected period'; expected chart canvas")
	}
	if !strings.Contains(html, `id="dailySpendChart"`) {
		t.Error("canvas#dailySpendChart not found in HTML; chart should be rendered when DailySpend is non-empty")
	}
}

// TestDailySpendChart_AllZeroSpend_RendersCanvas verifies AC4: when all DailySpend
// entries have Spend==0, the chart should still render (a $0 bar chart), NOT show
// the "No spend data" empty state.
func TestDailySpendChart_AllZeroSpend_RendersCanvas(t *testing.T) {
	spend := []pages.DailySpend{
		{Date: "2026-02-17", Spend: 0.0},
		{Date: "2026-02-18", Spend: 0.0},
		{Date: "2026-02-19", Spend: 0.0},
	}
	html := costTabPage(t, spend)

	if strings.Contains(html, "No spend data for selected period") {
		t.Error("all-zero DailySpend shows 'No spend data for selected period'; AC4 says it should render a $0 bar chart")
	}
	if !strings.Contains(html, `id="dailySpendChart"`) {
		t.Error("canvas#dailySpendChart not found for all-zero DailySpend; should render chart per AC4")
	}
}

// ─── AC5: Empty DailySpend renders "No spend data" empty state ──────────────

// TestDailySpendChart_EmptyData_ShowsEmptyState verifies that when DailySpend
// is nil (no SpendLog records at all), the empty-state message is shown.
func TestDailySpendChart_EmptyData_ShowsEmptyState(t *testing.T) {
	html := costTabPage(t, nil)

	if !strings.Contains(html, "No spend data for selected period") {
		t.Error("empty DailySpend should render 'No spend data for selected period' but it was not found")
	}
	if strings.Contains(html, `id="dailySpendChart"`) {
		t.Error("canvas#dailySpendChart should NOT be rendered when DailySpend is empty")
	}
}
