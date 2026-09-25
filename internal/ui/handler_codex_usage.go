package ui

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/praxisllmlab/tianjiLLM/internal/provider/chatgptcodex"
	proxyhandler "github.com/praxisllmlab/tianjiLLM/internal/proxy/handler"
	"github.com/praxisllmlab/tianjiLLM/internal/security/redact"
	"github.com/praxisllmlab/tianjiLLM/internal/ui/components/toast"
	"github.com/praxisllmlab/tianjiLLM/internal/ui/pages"
)

func (h *UIHandler) loadCodexUsageData(r *http.Request) pages.CodexUsageTabData {
	data := pages.CodexUsageTabData{
		Strategy: codexUsageStrategy(h),
	}
	if h.DB == nil {
		return data
	}
	creds, err := h.DB.ListCredentials(r.Context())
	if err != nil {
		log.Printf("warn: codex-usage: list credentials failed: %v", err)
		return data
	}
	now := time.Now()
	proxy := h.proxyLifecycleHandlers()
	for _, cred := range creds {
		if cred.CredentialType != openAISubscriptionCredentialType {
			continue
		}
		result := proxy.OpenAISubscriptionCodexUsageSnapshot(r.Context(), cred.CredentialID, false)
		data.Cards = append(data.Cards, h.codexUsageCardForCredential(cred, result, now))
	}
	return data
}

func codexUsageStrategy(h *UIHandler) string {
	if h == nil || h.Config == nil {
		return ""
	}
	return string(h.Config.NativeUpstreamStrategy)
}

func codexUsagePrimaryGate(h *UIHandler) float64 {
	if h == nil || h.Config == nil {
		return 0
	}
	return h.Config.RatelimitAlertThreshold
}

func codexUsageWeeklyGate(h *UIHandler) float64 {
	if h == nil || h.Config == nil || h.Config.CodexUsageWeeklyThreshold <= 0 {
		return chatgptcodex.DefaultSecondaryGate
	}
	return h.Config.CodexUsageWeeklyThreshold
}

func (h *UIHandler) handleCodexUsageAPI(w http.ResponseWriter, r *http.Request) {
	data := h.loadCodexUsageData(r)
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(data.Cards); err != nil {
		http.Error(w, "encode error", http.StatusInternalServerError)
	}
}

func codexUsageStatusLabel(result proxyhandler.OpenAISubscriptionCodexUsageResult) string {
	switch result.Status {
	case "fresh":
		return "Fresh"
	case "backoff":
		return "Backoff"
	case "auth_error":
		return "Auth error"
	case "unavailable":
		return "Unavailable"
	default:
		if result.Status != "" {
			return strings.ReplaceAll(result.Status, "_", " ")
		}
		return "No data"
	}
}

func codexUsageStatusVariant(result proxyhandler.OpenAISubscriptionCodexUsageResult) string {
	switch result.Status {
	case "fresh":
		return "default"
	case "backoff":
		return "outline"
	default:
		return "destructive"
	}
}

func codexUsageSafeError(result proxyhandler.OpenAISubscriptionCodexUsageResult) string {
	if result.LastErrorReason == "" {
		return ""
	}
	return redact.String(result.LastErrorReason)
}

func codexUsageCacheState(result proxyhandler.OpenAISubscriptionCodexUsageResult, now time.Time) string {
	if !result.BackoffUntil.IsZero() && result.BackoffUntil.After(now) {
		return "Backoff until " + formatTime(result.BackoffUntil)
	}
	if !result.ExpiresAt.IsZero() && result.ExpiresAt.After(now) {
		return "Cached until " + formatTime(result.ExpiresAt)
	}
	return ""
}

func codexUsageRefreshDisabledReason(result proxyhandler.OpenAISubscriptionCodexUsageResult, now time.Time) string {
	if !result.BackoffUntil.IsZero() && result.BackoffUntil.After(now) {
		return "Refresh deferred until " + formatTime(result.BackoffUntil)
	}
	return ""
}

func codexUsageRefreshMessage(result proxyhandler.OpenAISubscriptionCodexUsageResult) string {
	if result.LastErrorReason != "" {
		return "Codex usage refresh: " + redact.String(result.LastErrorReason)
	}
	if result.RefreshDeferred {
		return "Codex usage refresh deferred"
	}
	return "Codex usage refreshed"
}

func codexUsageResetMessage(result proxyhandler.OpenAISubscriptionCodexUsageResetResult) string {
	code := strings.ToLower(strings.TrimSpace(result.Code))
	if result.LastErrorReason != "" {
		reason := redact.String(result.LastErrorReason)
		switch code {
		case "reset":
			if result.WindowsReset > 0 {
				return fmt.Sprintf("Codex usage reset consumed (%d window(s) reset), but usage refresh failed: %s", result.WindowsReset, reason)
			}
			return "Codex usage reset consumed, but usage refresh failed: " + reason
		case "nothing_to_reset":
			return "Codex usage reset: nothing to reset; usage refresh failed: " + reason
		case "already_redeemed":
			return "Codex usage reset already redeemed; usage refresh failed: " + reason
		case "no_credit":
			return "Codex usage reset failed: no reset credits available"
		}
		return "Codex usage reset failed: " + redact.String(result.LastErrorReason)
	}
	switch code {
	case "reset":
		if result.WindowsReset > 0 {
			return fmt.Sprintf("Codex usage reset consumed (%d window(s) reset)", result.WindowsReset)
		}
		return "Codex usage reset consumed"
	case "nothing_to_reset":
		return "Codex usage reset: nothing to reset"
	case "no_credit":
		return "Codex usage reset failed: no reset credits available"
	case "already_redeemed":
		return "Codex usage reset already redeemed"
	case "":
		return "Codex usage reset completed"
	default:
		return "Codex usage reset completed: " + redact.String(code)
	}
}

func codexUsageResetToastVariant(result proxyhandler.OpenAISubscriptionCodexUsageResetResult) toast.Variant {
	code := strings.ToLower(strings.TrimSpace(result.Code))
	if result.LastErrorReason == "" {
		if code == "no_credit" {
			return toast.VariantError
		}
		return toast.VariantDefault
	}
	switch code {
	case "reset", "nothing_to_reset", "already_redeemed":
		return toast.VariantWarning
	default:
		return toast.VariantError
	}
}

func codexUsageSelectionScore(result proxyhandler.OpenAISubscriptionCodexUsageResult, primaryGate, weeklyGate float64, now time.Time) *float64 {
	score, _, ok := chatgptcodex.SelectionScoreAtWithGates(result.Snapshot, primaryGate, weeklyGate, result.FetchedAt, now)
	if !ok {
		return nil
	}
	return &score
}
