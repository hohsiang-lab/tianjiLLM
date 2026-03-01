package pages_test

// HO-82: Failing tests for overageBadgeLabel() and 5h/7d utilization display.
// These tests document the EXPECTED behavior after the fix.
// They are intentionally FAILING on main to guide the implementation.

import (
	"strings"
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/ui/pages"
)

// TestOverageBadgeLabel_EmptyString verifies that when OverageDisabledReason is empty string,
// the rendered badge shows "— unknown" (not "⚠️ disabled").
//
// Current behavior (BUG): empty string hits default fallback -> "⚠️ disabled"
// Expected: "— unknown"
func TestOverageBadgeLabel_EmptyString(t *testing.T) {
	tok := pages.AnthropicRateLimitWidgetData{
		TokenKey:              "abc123def456",
		UnifiedStatus:        "allowed",
		Unified5hStatus:      "allowed",
		Unified5hUtilization: 0.3,
		Unified7dStatus:      "allowed",
		Unified7dUtilization: 0.5,
		OverageDisabledReason: "",
	}

	html := renderToString(t, pages.RateLimitWidget([]pages.AnthropicRateLimitWidgetData{tok}))

	if !strings.Contains(html, "\u2014 unknown") {
		t.Errorf("empty OverageDisabledReason: expected '\u2014 unknown' in badge, got:\n%s", html)
	}

	if strings.Contains(html, "\u26a0\ufe0f disabled") {
		t.Errorf("empty OverageDisabledReason: must not render '\u26a0\ufe0f disabled', got:\n%s", html)
	}
}

// TestOverageBadgeLabel_Disabled verifies that OverageDisabledReason="disabled"
// renders as "⚠️ disabled" (explicit case, not just default fallback).
func TestOverageBadgeLabel_Disabled(t *testing.T) {
	tok := pages.AnthropicRateLimitWidgetData{
		TokenKey:              "abc123def456",
		UnifiedStatus:        "overage",
		Unified5hStatus:      "overage",
		Unified5hUtilization: 0.9,
		Unified7dStatus:      "overage",
		Unified7dUtilization: 0.85,
		OverageDisabledReason: "disabled",
	}

	html := renderToString(t, pages.RateLimitWidget([]pages.AnthropicRateLimitWidgetData{tok}))

	if !strings.Contains(html, "\u26a0\ufe0f disabled") {
		t.Errorf("OverageDisabledReason='disabled': expected '\u26a0\ufe0f disabled' in badge, got:\n%s", html)
	}
}

// TestOverageBadgeLabel_Allowed verifies "allowed" renders as "✅ allowed".
func TestOverageBadgeLabel_Allowed(t *testing.T) {
	tok := pages.AnthropicRateLimitWidgetData{
		TokenKey:              "abc123def456",
		UnifiedStatus:        "allowed",
		Unified5hStatus:      "allowed",
		Unified5hUtilization: 0.2,
		Unified7dStatus:      "allowed",
		Unified7dUtilization: 0.15,
		OverageDisabledReason: "allowed",
	}

	html := renderToString(t, pages.RateLimitWidget([]pages.AnthropicRateLimitWidgetData{tok}))

	if !strings.Contains(html, "\u2705 allowed") {
		t.Errorf("OverageDisabledReason='allowed': expected '\u2705 allowed' in badge, got:\n%s", html)
	}
}

// TestOverageBadgeLabel_Rejected verifies "rejected" renders as "❌ rejected".
func TestOverageBadgeLabel_Rejected(t *testing.T) {
	tok := pages.AnthropicRateLimitWidgetData{
		TokenKey:              "abc123def456",
		UnifiedStatus:        "rate_limited",
		Unified5hStatus:      "rate_limited",
		Unified5hUtilization: 1.0,
		Unified7dStatus:      "rate_limited",
		Unified7dUtilization: 0.99,
		OverageDisabledReason: "rejected",
	}

	html := renderToString(t, pages.RateLimitWidget([]pages.AnthropicRateLimitWidgetData{tok}))

	if !strings.Contains(html, "\u274c rejected") {
		t.Errorf("OverageDisabledReason='rejected': expected '\u274c rejected' in badge, got:\n%s", html)
	}
}

// TestRateLimitWidget_5hUtilization_NegativeOne verifies that Unified5hUtilization=-1
// (sentinel for "header absent") renders as "—" dash, not a negative percentage.
//
// Root cause A: Anthropic does not return unified OAuth headers on every response.
// When absent, utilization is -1 and UI must show "—".
func TestRateLimitWidget_5hUtilization_NegativeOne(t *testing.T) {
	tok := pages.AnthropicRateLimitWidgetData{
		TokenKey:              "abc123def456",
		UnifiedStatus:        "allowed",
		Unified5hStatus:      "",
		Unified5hUtilization: -1,
		Unified7dStatus:      "",
		Unified7dUtilization: -1,
	}

	html := renderToString(t, pages.RateLimitWidget([]pages.AnthropicRateLimitWidgetData{tok}))

	if !strings.Contains(html, "\u2014") {
		t.Errorf("Unified5hUtilization=-1: expected '\u2014' (dash) in rendered HTML, got:\n%s", html)
	}

	if strings.Contains(html, "aria-valuenow=\"-100\"") {
		t.Errorf("Unified5hUtilization=-1: must not render '-100%%', got:\n%s", html)
	}
}

// TestRateLimitWidget_7dUtilization_NegativeOne verifies same for 7d utilization.
func TestRateLimitWidget_7dUtilization_NegativeOne(t *testing.T) {
	tok := pages.AnthropicRateLimitWidgetData{
		TokenKey:              "abc123def456",
		UnifiedStatus:        "allowed",
		Unified5hUtilization: 0.3,
		Unified7dUtilization: -1,
	}

	html := renderToString(t, pages.RateLimitWidget([]pages.AnthropicRateLimitWidgetData{tok}))

	if !strings.Contains(html, "\u2014") {
		t.Errorf("Unified7dUtilization=-1: expected '\u2014' (dash) in rendered HTML, got:\n%s", html)
	}

	if strings.Contains(html, "aria-valuenow=\"-100\"") {
		t.Errorf("Unified7dUtilization=-1: must not render '-100%%', got:\n%s", html)
	}
}
