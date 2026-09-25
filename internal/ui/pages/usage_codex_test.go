package pages_test

import (
	"strings"
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/ui/pages"
	"github.com/stretchr/testify/assert"
)

func TestUsagePageRendersCodexTabTriggerAndContent(t *testing.T) {
	v := 0.14
	weeklyV := 0.32
	resetCredits := 2
	html := renderToString(t, pages.UsagePage(
		pages.UsagePageData{ActiveTab: "codex", Preset: "7d"},
		pages.CodexUsageTabData{
			Strategy: "sticky",
			Cards: []pages.CodexUsageCredentialCard{{
				CredentialID:          "cred-123",
				Name:                  "OpenAI Subscription",
				Email:                 "operator@example.com",
				PlanType:              "pro",
				StatusLabel:           "Fresh",
				LifecycleAction:       "disable",
				LifecycleActionLabel:  "Disable",
				LifecycleActionPrompt: "Disable this OpenAI subscription credential?",
				SelectionScore:        &v,
				ResetCreditsAvailable: &resetCredits,
				PrimaryWindow: pages.CodexUsageWindowView{
					Name:        "Primary 5h",
					UsedPercent: &v,
					Reset:       "4102444800",
					Status:      "available",
				},
				WeeklyWindow: pages.CodexUsageWindowView{
					Name:        "Weekly",
					UsedPercent: &weeklyV,
					Reset:       "4102444800",
					Status:      "available",
				},
			}},
		},
	))

	assert.Contains(t, html, `data-tab="codex"`)
	assert.Contains(t, html, "Codex")
	assert.Contains(t, html, "Codex — Plan Usage Limits")
	assert.Contains(t, html, "Sticky")
	assert.Contains(t, html, "operator@example.com")
	assert.Contains(t, html, "Plan usage limits")
	assert.Contains(t, html, "Current session")
	assert.Contains(t, html, "Resets in")
	assert.Contains(t, html, "Reset credits: 2")
	assert.Contains(t, html, "Reset usage")
	assert.Contains(t, html, `/ui/credentials/cred-123/codex-usage/reset`)
	assert.Contains(t, html, `hx-confirm="Consume one Codex reset credit now?"`)
	assert.Contains(t, html, "Disable")
	assert.Contains(t, html, `/ui/credentials/cred-123/disable`)
	assert.Contains(t, html, `hx-confirm="Disable this OpenAI subscription credential?"`)
	assert.NotContains(t, html, `/ui/credentials/cred-123/codex-usage/refresh`)
	assert.NotContains(t, html, ">Refresh<")
	assert.Contains(t, html, "14% used")
	assert.Contains(t, html, "Weekly limits")
	assert.Contains(t, html, "Selection score")
	assert.NotContains(t, html, "unknown")
	assert.NotContains(t, html, "access-secret")
}

func TestCredentialDetailRendersCodexUsageWithoutSecrets(t *testing.T) {
	v := 0.06
	html := renderToString(t, pages.CredentialDetailContent(pages.CredentialDetailData{
		ID:                 "cred-123",
		Name:               "OpenAI Subscription",
		Email:              "operator@example.com",
		CredentialStatus:   "Active",
		CredentialVariant:  "default",
		QuotaStatus:        "Unknown",
		QuotaStatusVariant: "secondary",
		CodexUsage: pages.CodexUsageCredentialCard{
			CredentialID: "cred-123",
			Name:         "OpenAI Subscription",
			Email:        "operator@example.com",
			PlanType:     "pro",
			StatusLabel:  "Fresh",
			PrimaryWindow: pages.CodexUsageWindowView{
				Name:        "Primary 5h",
				UsedPercent: &v,
				Status:      "available",
			},
		},
	}))

	assert.Contains(t, html, "Codex usage")
	assert.Contains(t, html, "operator@example.com")
	assert.Contains(t, html, "6%")
	assert.Contains(t, html, "Plan usage limits")
	assert.NotContains(t, html, "access-secret")
	assert.NotContains(t, html, "refresh-secret")
}

func TestUsageCodexTabRendersWeeklyOnlyWindowWithoutCurrentSession(t *testing.T) {
	v := 0.76
	html := renderToString(t, pages.UsageCodexTab(pages.CodexUsageTabData{
		Cards: []pages.CodexUsageCredentialCard{{
			CredentialID:         "cred-123",
			Name:                 "OpenAI Subscription",
			Email:                "operator@example.com",
			StatusLabel:          "Fresh",
			LifecycleAction:      "disable",
			LifecycleActionLabel: "Disable",
			WeeklyWindow: pages.CodexUsageWindowView{
				Name:        "Weekly",
				UsedPercent: &v,
				Reset:       "4102444800",
			},
		}},
	}))

	assert.Contains(t, html, "Weekly limits")
	assert.Contains(t, html, "All models")
	assert.Contains(t, html, "76% used")
	assert.NotContains(t, html, "Current session")
}

func TestUsageCodexTabRendersGlobalAndAdditionalModelWindows(t *testing.T) {
	globalWeekly := 0.04
	sparkPrimary := 0.0
	sparkWeekly := 0.0
	html := renderToString(t, pages.UsageCodexTab(pages.CodexUsageTabData{
		Cards: []pages.CodexUsageCredentialCard{{
			CredentialID: "cred-123",
			Name:         "OpenAI Subscription",
			Email:        "operator@example.com",
			StatusLabel:  "Fresh",
			WeeklyWindow: pages.CodexUsageWindowView{
				UsedPercent: &globalWeekly,
			},
			AdditionalBuckets: []pages.CodexUsageBucketView{{
				Name: "GPT-5.3-Codex-Spark",
				PrimaryWindow: pages.CodexUsageWindowView{
					UsedPercent: &sparkPrimary,
				},
				WeeklyWindow: pages.CodexUsageWindowView{
					UsedPercent: &sparkWeekly,
				},
			}},
		}},
	}))

	assert.Contains(t, html, "All models")
	assert.Contains(t, html, "Additional model limits")
	assert.Contains(t, html, "GPT-5.3-Codex-Spark")
	assert.Contains(t, html, "5-hour limit")
	assert.Contains(t, html, "Weekly limit")
	assert.Contains(t, html, "4% used")
	assert.Equal(t, 1, strings.Count(html, "All models"))
	assert.Equal(t, 2, strings.Count(html, "0% used"))
}

func TestUsageCodexTabDisablesResetButtonWithoutCredits(t *testing.T) {
	zeroCredits := 0
	v := 0.2
	html := renderToString(t, pages.UsageCodexTab(pages.CodexUsageTabData{
		Cards: []pages.CodexUsageCredentialCard{{
			CredentialID:          "cred-123",
			Name:                  "OpenAI Subscription",
			Email:                 "operator@example.com",
			StatusLabel:           "Fresh",
			LifecycleAction:       "disable",
			LifecycleActionLabel:  "Disable",
			ResetCreditsAvailable: &zeroCredits,
			PrimaryWindow: pages.CodexUsageWindowView{
				UsedPercent: &v,
			},
		}},
	}))

	assert.Contains(t, html, "Reset usage")
	assert.Contains(t, html, "No reset credits available")
	assert.Contains(t, html, "disabled")
	assert.Contains(t, html, `/ui/credentials/cred-123/codex-usage/reset`)
}

func TestUsageCodexTabRendersEnableForDisabledCredential(t *testing.T) {
	v := 0.2
	html := renderToString(t, pages.UsageCodexTab(pages.CodexUsageTabData{
		Cards: []pages.CodexUsageCredentialCard{{
			CredentialID:         "cred-123",
			Name:                 "OpenAI Subscription",
			Email:                "operator@example.com",
			StatusLabel:          "Unavailable",
			LifecycleAction:      "enable",
			LifecycleActionLabel: "Enable",
			PrimaryWindow: pages.CodexUsageWindowView{
				UsedPercent: &v,
			},
		}},
	}))

	assert.Contains(t, html, "Enable")
	assert.Contains(t, html, `/ui/credentials/cred-123/enable`)
	assert.Contains(t, html, `hx-target="#codex-usage-tab-content"`)
	assert.NotContains(t, html, `/ui/credentials/cred-123/codex-usage/refresh`)
	assert.NotContains(t, html, ">Refresh<")
}
