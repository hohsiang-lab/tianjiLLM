package pages

// HO-80: Failing tests for Daily Spend chart not rendering (blank).
//
// Root cause: chart.min.js is loaded at the bottom of <body> (layout.templ line 20),
// but dailySpendChartScript emits an inline <script> that calls `new Chart(...)`
// immediately via an IIFE. When the IIFE executes, Chart.js is not yet loaded,
// so `Chart` is undefined and the chart silently fails.
//
// Fix: either move chart.min.js into <head>, or wrap every inline chart init in a
// DOMContentLoaded listener so they run after all scripts have loaded.
//
// These tests are intentionally FAILING on current code to guide the implementor.

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/a-h/templ"
)

func renderComponent(t *testing.T, c templ.Component) string {
	t.Helper()
	var buf bytes.Buffer
	if err := c.Render(context.Background(), &buf); err != nil {
		t.Fatalf("render failed: %v", err)
	}
	return buf.String()
}

// TestLayout_ChartJsLoadedBeforeInlineScripts — EXPECTED TO FAIL (HO-80)
//
// Verifies that chart.min.js is referenced inside <head> so that `Chart` is
// defined before any inline chart initialisation scripts execute.
//
// Current behaviour: chart.min.js is in <body> after content, causing
// `Chart is not defined` runtime errors for all chart IIFEs.
func TestLayout_ChartJsLoadedBeforeInlineScripts(t *testing.T) {
	html := renderComponent(t, document("Test"))

	headEnd := strings.Index(html, "</head>")
	if headEnd == -1 {
		t.Fatal("rendered HTML has no </head> tag")
	}
	head := html[:headEnd]

	if !strings.Contains(head, "chart.min.js") {
		t.Errorf(
			"BUG HO-80: chart.min.js is NOT inside <head>.\n"+
				"It will be undefined when the inline `new Chart(...)` IIFEs execute,\n"+
				"causing the Daily Spend chart (and all other charts) to be blank.\n\n"+
				"<head> rendered:\n%s", head,
		)
	}
}

// TestDailySpendChartScript_InitGuardedAgainstMissingChartJs — EXPECTED TO FAIL (HO-80)
//
// Verifies that the inline chart initialisation is deferred until Chart.js is
// guaranteed to be loaded (e.g. wrapped in DOMContentLoaded).
//
// Current behaviour: IIFE runs immediately — if chart.min.js is below in the DOM,
// `Chart` is not yet defined.
func TestDailySpendChartScript_InitGuardedAgainstMissingChartJs(t *testing.T) {
	data := []DailySpend{{Date: "2025-03-01", Spend: 1.00}}
	html := renderComponent(t, dailySpendChartScript(data))

	hasDeferredInit := strings.Contains(html, "DOMContentLoaded") ||
		strings.Contains(html, "window.onload") ||
		strings.Contains(html, "addEventListener('load'") ||
		strings.Contains(html, `addEventListener("load"`)

	if !hasDeferredInit {
		t.Errorf(
			"BUG HO-80: dailySpendChartScript IIFE has no load guard.\n"+
				"If chart.min.js is not yet in the DOM, `Chart` is undefined and the\n"+
				"Daily Spend bar chart renders as a blank white box.\n"+
				"Wrap the init in DOMContentLoaded or ensure chart.min.js loads before this script.\n\n"+
				"Rendered script:\n%s", html,
		)
	}
}

// TestDailySpendChartScript_JSONDataFieldNames — sanity / regression guard
//
// Verifies the JSON data embedded via templ.JSONScript uses the field names
// that the JavaScript reads: d.Date and d.Spend.
// If struct tags change these names the chart will receive empty series.
func TestDailySpendChartScript_JSONDataFieldNames(t *testing.T) {
	data := []DailySpend{{Date: "2025-03-01", Spend: 9.99}}
	html := renderComponent(t, dailySpendChartScript(data))

	if !strings.Contains(html, `"Date"`) {
		t.Errorf(`BUG HO-80: serialised JSON missing "Date" field; JS d.Date will be undefined. HTML: %s`, html)
	}
	if !strings.Contains(html, `"Spend"`) {
		t.Errorf(`BUG HO-80: serialised JSON missing "Spend" field; JS d.Spend will be undefined. HTML: %s`, html)
	}
	if !strings.Contains(html, "2025-03-01") {
		t.Errorf(`BUG HO-80: serialised JSON missing date value "2025-03-01". HTML: %s`, html)
	}
}

// TestUsageCostTab_DailySpendCanvas_RendersWhenDataPresent — regression guard
//
// Verifies that a non-empty DailySpend slice causes the canvas element to be
// rendered (not the "No spend data" placeholder).
func TestUsageCostTab_DailySpendCanvas_RendersWhenDataPresent(t *testing.T) {
	tab := CostTabData{
		DailySpend: []DailySpend{
			{Date: "2025-03-01", Spend: 5.00},
			{Date: "2025-03-02", Spend: 3.50},
		},
		TopKeyLimit: 5,
	}
	html := renderComponent(t, UsageCostTab(tab))

	if !strings.Contains(html, `id="dailySpendChart"`) {
		t.Errorf("BUG HO-80: #dailySpendChart canvas not rendered despite non-empty DailySpend.\nHTML (first 2000 chars):\n%.2000s", html)
	}
	if strings.Contains(html, "No spend data") {
		t.Errorf("BUG HO-80: placeholder 'No spend data' rendered even though DailySpend is non-empty.")
	}
}
