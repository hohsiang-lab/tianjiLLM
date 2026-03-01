package pages_test

// HO-79: Failing tests for rate limit widget UI improvements.
// These tests document the expected behaviour AFTER the fix.
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
// Current behaviour (BUG): single-token path hard-codes the string "OAuth Token".
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

// TestRateLimitCard_ProgressBar_RendersAccessibleBar verifies the utilisation bars
// include role="progressbar" for accessibility.
//
// Current behaviour (BUG): plain <div> with CSS width style, no ARIA role.
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
		t.Errorf("no element with role=\"progressbar\" found; utilisation bars should be accessible.\nRendered HTML:\n%s", html)
	}
}

// TestRateLimitCard_OverageDisabledReason_RendersBadge verifies that
// OverageDisabledReason="org_level_disabled" renders as a styled badge,
// not the raw machine string.
//
// Current behaviour (BUG):
//   <p class="text-xs text-muted-foreground">Overage disabled: org_level_disabled</p>
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

// TestRateLimitPollingScript_ProgressBar_HasARIA verifies that the JS polling
// script (rendered inside RateLimitWidget) includes ARIA attributes on the
// dynamically-generated inner progress bar divs for both 5h and 7d windows.
func TestRateLimitPollingScript_ProgressBar_HasARIA(t *testing.T) {
	// Render with no tokens so we still get the script block
	html := renderToString(t, pages.RateLimitWidget(nil))

	if !strings.Contains(html, `role="progressbar"`) {
		t.Errorf("polling script does not contain role=\"progressbar\"; JS-generated inner bar is missing ARIA.\nRendered HTML:\n%s", html)
	}
	if !strings.Contains(html, "aria-valuenow") {
		t.Errorf("polling script does not contain aria-valuenow.\nRendered HTML:\n%s", html)
	}
	if !strings.Contains(html, "aria-valuemin") {
		t.Errorf("polling script does not contain aria-valuemin.\nRendered HTML:\n%s", html)
	}
	if !strings.Contains(html, "aria-valuemax") {
		t.Errorf("polling script does not contain aria-valuemax.\nRendered HTML:\n%s", html)
	}
}
