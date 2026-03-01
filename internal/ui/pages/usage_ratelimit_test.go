package pages_test

// HO-79: Failing tests for rate limit widget UI improvements.
// These tests document the expected behavior AFTER the fix.
// They are intentionally FAILING on main to guide the implementation.

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/a-h/templ"
	"github.com/praxisllmlab/tianjiLLM/internal/ui/pages"
)

// renderToString renders a templ component to a plain HTML string for inspection.
func renderToString(t *testing.T, c templ.Component) string {
	t.Helper()
	var buf bytes.Buffer
	if err := c.Render(context.Background(), &buf); err != nil {
		t.Fatalf("failed to render component: %v", err)
	}
	return buf.String()
}

// TestRateLimitCard_SingleToken_ShowsKeyHash verifies that when there is only one
// Anthropic OAuth token, the widget still renders the token key hash (first 12 chars
// of the sha256 prefix stored in TokenKey) rather than the generic label "OAuth Token".
//
// Current behavior (BUG): single-token path hard-codes the string "OAuth Token".
// Expected: always display the 12-char key hash.
func TestRateLimitCard_SingleToken_ShowsKeyHash(t *testing.T) {
	const keyHash = "abc123def456" // first 12 chars of sha256

	tok := pages.AnthropicRateLimitWidgetData{
		TokenKey:             keyHash,
		UnifiedStatus:        "allowed",
		Unified5hStatus:      "allowed",
		Unified5hUtilization: 0.3,
		Unified7dStatus:      "allowed",
		Unified7dUtilization: 0.5,
	}

	html := renderToString(t, pages.RateLimitWidget([]pages.AnthropicRateLimitWidgetData{tok}))

	if strings.Contains(html, "OAuth Token") {
		t.Errorf("single-token card still shows generic label 'OAuth Token'; expected key hash %q.\nRendered HTML:\n%s", keyHash, html)
	}

	if !strings.Contains(html, keyHash) {
		t.Errorf("single-token card does not contain key hash %q.\nRendered HTML:\n%s", keyHash, html)
	}
}

// TestRateLimitCard_ProgressBar_RendersAccessibleBar verifies the utilization bars
// include role="progressbar" for accessibility.
//
// Current behavior (BUG): plain <div> with CSS width style, no ARIA role.
// Expected: at least one element with role="progressbar".
func TestRateLimitCard_ProgressBar_RendersAccessibleBar(t *testing.T) {
	tok := pages.AnthropicRateLimitWidgetData{
		TokenKey:             "xyz789abc012",
		UnifiedStatus:        "allowed",
		Unified5hStatus:      "allowed",
		Unified5hUtilization: 0.75,
		Unified7dStatus:      "allowed",
		Unified7dUtilization: 0.45,
	}

	html := renderToString(t, pages.RateLimitWidget([]pages.AnthropicRateLimitWidgetData{tok}))

	if !strings.Contains(html, `role="progressbar"`) {
		t.Errorf("no element with role=\"progressbar\" found; utilization bars should be accessible.\nRendered HTML:\n%s", html)
	}
}

// TestRateLimitCard_OverageDisabledReason_RendersBadge verifies that
// OverageDisabledReason="org_level_disabled" renders as a styled badge,
// not the raw machine string.
//
// Current behavior (BUG):
//
//	<p class="text-xs text-muted-foreground">Overage disabled: org_level_disabled</p>
//
// Expected: a badge element (span with badge class or emoji ⚠️).
func TestRateLimitCard_OverageDisabledReason_RendersBadge(t *testing.T) {
	tok := pages.AnthropicRateLimitWidgetData{
		TokenKey:              "def456ghi789",
		UnifiedStatus:         "overage",
		Unified5hStatus:       "overage",
		Unified5hUtilization:  1.0,
		Unified7dStatus:       "overage",
		Unified7dUtilization:  0.95,
		OverageDisabledReason: "org_level_disabled",
	}

	html := renderToString(t, pages.RateLimitWidget([]pages.AnthropicRateLimitWidgetData{tok}))

	if strings.Contains(html, "Overage disabled: org_level_disabled") {
		t.Errorf("raw string 'Overage disabled: org_level_disabled' rendered as plain text; expected a styled badge.\nRendered HTML:\n%s", html)
	}

	hasBadge := strings.Contains(html, `badge`) ||
		strings.Contains(html, "⚠") ||
		strings.Contains(html, `disabled-badge`) ||
		strings.Contains(html, `overage-badge`)
	if !hasBadge {
		t.Errorf("no badge element found for OverageDisabledReason='org_level_disabled'.\nRendered HTML:\n%s", html)
	}
}

// --- HO-82: Rate Limit utilization 顯示 "—" ---

// TestRateLimitTemplate_ShowsHelpTextWhenUtilizationUnavailable verifies that when
// Unified5hUtilization and Unified7dUtilization are -1 (missing headers = non-OAuth key),
// the rendered widget shows the "—" value AND provides an explanatory tooltip or
// ARIA attribute explaining WHY utilization is unavailable.
//
// Root cause of HO-82: non-OAuth API key responses lack utilization headers;
// parseFloat("") = -1 → fmtUtilPct(-1) = "—". Users see "—" with no explanation.
//
// FAILING TEST: The current template renders "—" but does NOT include a `title`
// attribute or aria-label near the utilization field explaining the unavailability.
// The fix must add: title="Not available for API keys (requires OAuth token)"
// or equivalent on the "—" span element.
func TestRateLimitTemplate_ShowsHelpTextWhenUtilizationUnavailable(t *testing.T) {
	tok := pages.AnthropicRateLimitWidgetData{
		TokenKey:             "apikey123abc",
		UnifiedStatus:        "",
		Unified5hStatus:      "",
		Unified5hUtilization: -1, // -1 = header absent (non-OAuth API key)
		Unified7dStatus:      "",
		Unified7dUtilization: -1,
	}

	html := renderToString(t, pages.RateLimitWidget([]pages.AnthropicRateLimitWidgetData{tok}))

	// Basic: "—" must be rendered.
	if !strings.Contains(html, "—") {
		t.Fatalf("rendered widget does not contain '—' for unavailable utilization.\nRendered HTML:\n%s", html)
	}

	// DESIRED (not yet implemented): when utilization == -1, the "—" span must carry
	// a title attribute with an explanation. This lets users hover to understand why.
	// Current implementation: <span class="text-xs font-medium">—</span> (no title).
	// Fixed implementation: <span class="text-xs font-medium" title="Not available for API keys">—</span>
	//
	// This assertion will FAIL until the template is updated to add the title attribute.
	hasTitleOnDash := strings.Contains(html, `title=`) && strings.Contains(html, "—")
	if !hasTitleOnDash {
		t.Errorf("Bug HO-82: utilization '—' is displayed without an explanatory title attribute.\n"+
			"Users with non-OAuth API keys see '—' with no hint about why utilization is unavailable.\n"+
			"Fix: add title=\"Not available for API keys (requires OAuth token)\" to the '—' span.\n"+
			"Current rendered HTML:\n%s", html)
	}
}
