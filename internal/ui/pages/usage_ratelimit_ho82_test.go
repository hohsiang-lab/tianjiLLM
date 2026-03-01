package pages_test

// HO-82: Tests for overage badge display bug fixes.

import (
	"strings"
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/ui/pages"
)

// TestRateLimitWidget_EmptyOverageDisabledReason_ShowsUnknown verifies that when
// OverageDisabledReason is empty string, the widget renders "— unknown" (not "⚠️ disabled").
func TestRateLimitWidget_EmptyOverageDisabledReason_ShowsUnknown(t *testing.T) {
	tok := pages.AnthropicRateLimitWidgetData{
		TokenKey:              "abc123def456",
		UnifiedStatus:         "",
		Unified5hUtilization:  -1,
		Unified7dUtilization:  -1,
		OverageDisabledReason: "",
	}

	html := renderToString(t, pages.RateLimitWidget([]pages.AnthropicRateLimitWidgetData{tok}))

	if strings.Contains(html, "\u26a0\ufe0f disabled") {
		t.Errorf("empty OverageDisabledReason should not render '⚠️ disabled'; got:\n%s", html)
	}

	if !strings.Contains(html, "\u2014 unknown") {
		t.Errorf("empty OverageDisabledReason should render '— unknown'; got:\n%s", html)
	}
}

// TestRateLimitWidget_DisabledOverageReason_ShowsDisabled verifies that when
// OverageDisabledReason is "disabled", the widget renders "⚠️ disabled".
func TestRateLimitWidget_DisabledOverageReason_ShowsDisabled(t *testing.T) {
	tok := pages.AnthropicRateLimitWidgetData{
		TokenKey:              "abc123def456",
		UnifiedStatus:         "overage",
		Unified5hUtilization:  0.9,
		Unified7dUtilization:  0.85,
		OverageDisabledReason: "disabled",
	}

	html := renderToString(t, pages.RateLimitWidget([]pages.AnthropicRateLimitWidgetData{tok}))

	if !strings.Contains(html, "\u26a0\ufe0f disabled") {
		t.Errorf("OverageDisabledReason='disabled' should render '⚠️ disabled'; got:\n%s", html)
	}
}

// TestRateLimitWidget_AllowedOverageReason_ShowsAllowed verifies allowed renders correctly.
func TestRateLimitWidget_AllowedOverageReason_ShowsAllowed(t *testing.T) {
	tok := pages.AnthropicRateLimitWidgetData{
		TokenKey:              "abc123def456",
		OverageDisabledReason: "allowed",
	}
	html := renderToString(t, pages.RateLimitWidget([]pages.AnthropicRateLimitWidgetData{tok}))

	if !strings.Contains(html, "\u2705 allowed") {
		t.Errorf("OverageDisabledReason='allowed' should render '✅ allowed'; got:\n%s", html)
	}
}
