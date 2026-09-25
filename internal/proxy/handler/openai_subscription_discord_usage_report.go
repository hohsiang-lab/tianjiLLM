package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/praxisllmlab/tianjiLLM/internal/callback"
	"github.com/praxisllmlab/tianjiLLM/internal/db"
	"github.com/praxisllmlab/tianjiLLM/internal/provider/chatgptcodex"
	"github.com/praxisllmlab/tianjiLLM/internal/security/redact"
)

type OpenAISubscriptionDiscordUsageReportResult struct {
	CredentialCount int
	Parts           []string
	FailureReason   string
}

const openAISubscriptionDiscordUsageWeeklyWindowSeconds int64 = 7 * 24 * 60 * 60

func (h *Handlers) OpenAISubscriptionDiscordUsageReport(ctx context.Context) OpenAISubscriptionDiscordUsageReportResult {
	var report OpenAISubscriptionDiscordUsageReportResult
	if h == nil || h.DB == nil {
		report.FailureReason = "database_not_configured"
		return report
	}
	credentials, err := h.DB.ListCredentials(ctx)
	if err != nil {
		report.FailureReason = "credential_list_failed"
		return report
	}
	subscriptions := make([]db.CredentialTable, 0, len(credentials))
	for _, credential := range credentials {
		if credential.CredentialType == CredentialTypeOpenAISubscription {
			subscriptions = append(subscriptions, credential)
		}
	}
	sort.Slice(subscriptions, func(i, j int) bool {
		return subscriptions[i].CredentialID < subscriptions[j].CredentialID
	})

	report.CredentialCount = len(subscriptions)
	rows := make([]string, 0, len(subscriptions))
	for _, credential := range subscriptions {
		if ctx.Err() != nil {
			report.FailureReason = "context_cancelled"
			return report
		}
		rows = append(rows, h.openAISubscriptionDiscordUsageReportRow(ctx, credential))
		if ctx.Err() != nil {
			report.FailureReason = "context_cancelled"
			return report
		}
	}
	report.Parts = openAISubscriptionDiscordUsageReportParts(h.openAISubscriptionNowUTC(), len(subscriptions), rows)
	return report
}

func (h *Handlers) openAISubscriptionDiscordUsageReportRow(ctx context.Context, credential db.CredentialTable) string {
	current, err := h.DB.GetCredential(ctx, credential.CredentialID)
	if err != nil {
		reason := "credential_lookup_failed"
		if errors.Is(err, pgx.ErrNoRows) {
			reason = "credential_missing"
		}
		return openAISubscriptionDiscordUsageReportRowText(credential.CredentialID, "unavailable", reason, chatgptcodex.UsageSnapshot{})
	}
	if current.CredentialType != CredentialTypeOpenAISubscription {
		return openAISubscriptionDiscordUsageReportRowText(credential.CredentialID, "unavailable", "credential_wrong_type", chatgptcodex.UsageSnapshot{})
	}
	credential = current
	if status, reason, selectable := openAISubscriptionDiscordUsageCredentialState(credential); !selectable {
		return openAISubscriptionDiscordUsageReportRowText(credential.CredentialID, status, reason, chatgptcodex.UsageSnapshot{})
	}
	usage := h.OpenAISubscriptionCodexUsageSnapshot(ctx, credential.CredentialID, false)
	status, reason := openAISubscriptionDiscordUsageResultState(usage)
	return openAISubscriptionDiscordUsageReportRowText(credential.CredentialID, status, reason, usage.Snapshot)
}

func openAISubscriptionDiscordUsageCredentialState(credential db.CredentialTable) (string, string, bool) {
	if len(credential.CredentialInfo) == 0 {
		return "", "", true
	}
	var info OpenAISubscriptionCredentialInfo
	if err := json.Unmarshal(credential.CredentialInfo, &info); err != nil {
		return "malformed", "credential_malformed", false
	}
	if strings.TrimSpace(info.Status) == "disabled" {
		return "disabled", "credential_disabled", false
	}
	if openAISubscriptionCredentialInfoSelectable(info) {
		return "", "", true
	}
	if strings.TrimSpace(info.DisabledReason) == "operator_disabled" {
		return "disabled", "credential_disabled", false
	}
	if strings.TrimSpace(info.DisabledReason) == string(OpenAISubscriptionCredentialReconnect) {
		return "reconnect_required", "refresh_token_invalidated", false
	}
	return "unavailable", "refresh_failed", false
}

func openAISubscriptionDiscordUsageResultState(usage OpenAISubscriptionCodexUsageResult) (string, string) {
	reason := openAISubscriptionDiscordUsageReason(usage.LastErrorReason)
	switch usage.Status {
	case "fresh":
		return "fresh", reason
	case "backoff":
		if codexUsageResultHasSnapshot(usage) {
			return "stale", "backoff"
		}
		return "backoff", reason
	case "auth_error":
		return "unavailable", "auth_failed_after_refresh"
	case "unavailable":
		switch reason {
		case "credential_disabled":
			return "disabled", reason
		case "refresh_token_invalidated":
			return "reconnect_required", reason
		case "credential_malformed":
			return "malformed", reason
		default:
			return "unavailable", reason
		}
	default:
		return "unavailable", reason
	}
}

func openAISubscriptionDiscordUsageReason(reason string) string {
	switch strings.TrimSpace(reason) {
	case "",
		"credential_missing",
		"credential_wrong_type",
		"credential_disabled",
		"credential_malformed",
		"credential_expired",
		"credential_lookup_failed",
		"refresh_failed",
		"auth_failed_after_refresh",
		"refresh_token_invalidated",
		"rate_limited",
		"upstream_server_error",
		"upstream_error",
		"upstream_request_failed",
		"empty_snapshot":
		return strings.TrimSpace(reason)
	default:
		return "unavailable"
	}
}

func openAISubscriptionDiscordUsageReportRowText(credentialID, status, reason string, snapshot chatgptcodex.UsageSnapshot) string {
	parts := []string{
		"credential=" + openAISubscriptionDiscordUsageText(credentialID),
		"status=" + status,
	}
	if plan := openAISubscriptionDiscordUsageText(snapshot.PlanType); plan != "" {
		parts = append(parts, "plan="+plan)
	}
	primaryWindow := snapshot.PrimaryWindow
	weeklyWindow := snapshot.WeeklyWindow
	if primaryWindow.LimitWindowSeconds != nil &&
		*primaryWindow.LimitWindowSeconds == openAISubscriptionDiscordUsageWeeklyWindowSeconds &&
		weeklyWindow.UsedPercent == nil {
		weeklyWindow = primaryWindow
		primaryWindow = chatgptcodex.UsageWindow{}
	}
	parts = append(parts, openAISubscriptionDiscordUsageWindowParts("primary_5h", primaryWindow)...)
	parts = append(parts, openAISubscriptionDiscordUsageWindowParts("weekly_7d", weeklyWindow)...)
	buckets := append([]chatgptcodex.UsageBucket(nil), snapshot.AdditionalBuckets...)
	sort.Slice(buckets, func(i, j int) bool { return buckets[i].Name < buckets[j].Name })
	for _, bucket := range buckets {
		name := openAISubscriptionDiscordUsageText(bucket.Name)
		if name == "" {
			continue
		}
		parts = append(parts, openAISubscriptionDiscordUsageWindowParts("additional_"+name+"_primary_5h", bucket.PrimaryWindow)...)
		parts = append(parts, openAISubscriptionDiscordUsageWindowParts("additional_"+name+"_weekly_7d", bucket.WeeklyWindow)...)
	}
	if reason != "" {
		parts = append(parts, "reason="+reason)
	}
	return strings.Join(parts, " | ")
}

func openAISubscriptionDiscordUsageWindowParts(name string, window chatgptcodex.UsageWindow) []string {
	var parts []string
	if window.UsedPercent != nil {
		parts = append(parts, fmt.Sprintf("%s=%.1f%%", name, *window.UsedPercent*100))
	}
	if reset := openAISubscriptionDiscordUsageReset(window.ResetAt); reset != "" {
		parts = append(parts, name+"_reset="+reset)
	}
	if status := openAISubscriptionDiscordUsageText(window.Status); status != "" {
		parts = append(parts, name+"_state="+status)
	}
	return parts
}

func openAISubscriptionDiscordUsageReset(value string) string {
	value = strings.TrimSpace(value)
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return parsed.UTC().Format(time.RFC3339)
	}
	if seconds, err := strconv.ParseInt(value, 10, 64); err == nil && seconds > 0 {
		return time.Unix(seconds, 0).UTC().Format(time.RFC3339)
	}
	return ""
}

func openAISubscriptionDiscordUsageText(value string) string {
	value = strings.TrimSpace(redact.String(value))
	if value == "" || strings.Contains(value, "@") {
		return ""
	}
	var out strings.Builder
	for _, char := range value {
		if (char >= 'a' && char <= 'z') ||
			(char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') ||
			strings.ContainsRune("._:-/", char) {
			out.WriteRune(char)
		}
	}
	return openAISubscriptionDiscordUsageTruncate(out.String(), 160)
}

func openAISubscriptionDiscordUsageReportParts(now time.Time, credentialCount int, rows []string) []string {
	period := now.UTC().Truncate(30 * time.Minute).Format(time.RFC3339)
	partCountHint := max(1, len(rows)+1)
	maxBodyRunes := callback.DiscordUsageReportContentLimit - utf8.RuneCountInString(openAISubscriptionDiscordUsagePartPrefix(period, partCountHint, partCountHint))
	header := fmt.Sprintf("credentials=%d", credentialCount)
	if credentialCount == 0 {
		header += " | zero-credential report"
	}
	bodies := []string{header}
	for _, row := range rows {
		row = openAISubscriptionDiscordUsageTruncate(row, maxBodyRunes)
		last := len(bodies) - 1
		if utf8.RuneCountInString(bodies[last])+1+utf8.RuneCountInString(row) <= maxBodyRunes {
			bodies[last] += "\n" + row
			continue
		}
		bodies = append(bodies, row)
	}
	parts := make([]string, len(bodies))
	for i, body := range bodies {
		parts[i] = openAISubscriptionDiscordUsagePartPrefix(period, i+1, len(bodies)) + body
	}
	return parts
}

func openAISubscriptionDiscordUsagePartPrefix(period string, part, total int) string {
	return fmt.Sprintf("Tianji OpenAI credential usage operational snapshot (not billing) | period=%s | part %d/%d\n", period, part, total)
}

func openAISubscriptionDiscordUsageTruncate(value string, maxRunes int) string {
	if maxRunes <= 0 || utf8.RuneCountInString(value) <= maxRunes {
		return value
	}
	if maxRunes == 1 {
		return "…"
	}
	runes := []rune(value)
	return string(runes[:maxRunes-1]) + "…"
}
