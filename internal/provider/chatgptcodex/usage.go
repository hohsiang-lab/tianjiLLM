package chatgptcodex

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

type UsageFetcher interface {
	Fetch(ctx context.Context, req UsageRequest) (UsageSnapshot, error)
}

type ResetCreditConsumer interface {
	ConsumeResetCredit(ctx context.Context, req ResetCreditRequest) (ResetCreditResult, error)
}

type UsageClient struct {
	BaseURL    string
	Originator string
	HTTPClient *http.Client
}

type UsageRequest struct {
	AccessToken string
	AccountID   string
}

type ResetCreditRequest struct {
	AccessToken     string
	AccountID       string
	RedeemRequestID string
}

type ResetCreditResult struct {
	Code         string `json:"code,omitempty"`
	WindowsReset int64  `json:"windows_reset,omitempty"`
}

type UsageSnapshot struct {
	Email                 string             `json:"email,omitempty"`
	PlanType              string             `json:"plan_type,omitempty"`
	RateLimit             *UsageRateLimit    `json:"rate_limit,omitempty"`
	RateLimitResetCredits *UsageResetCredits `json:"rate_limit_reset_credits,omitempty"`
	PrimaryWindow         UsageWindow        `json:"primary_window,omitempty"`
	WeeklyWindow          UsageWindow        `json:"weekly_window,omitempty"`
	AdditionalBuckets     []UsageBucket      `json:"additional_buckets,omitempty"`
	CreditsStatus         string             `json:"credits_status,omitempty"`
}

type UsageRateLimit struct {
	Allowed         *bool       `json:"allowed,omitempty"`
	LimitReached    *bool       `json:"limit_reached,omitempty"`
	PrimaryWindow   UsageWindow `json:"primary_window,omitempty"`
	SecondaryWindow UsageWindow `json:"secondary_window,omitempty"`
}

type UsageResetCredits struct {
	AvailableCount *int `json:"available_count,omitempty"`
}

type UsageWindow struct {
	Name               string   `json:"name,omitempty"`
	UsedPercent        *float64 `json:"used_percent,omitempty"`
	ResetAt            string   `json:"reset_at,omitempty"`
	ResetAfterSeconds  *int64   `json:"reset_after_seconds,omitempty"`
	LimitWindowSeconds *int64   `json:"limit_window_seconds,omitempty"`
	Status             string   `json:"status,omitempty"`
}

type UsageBucket struct {
	Name          string      `json:"name,omitempty"`
	PrimaryWindow UsageWindow `json:"primary_window,omitempty"`
	WeeklyWindow  UsageWindow `json:"weekly_window,omitempty"`
}

func (b *UsageBucket) UnmarshalJSON(data []byte) error {
	var value struct {
		Name          string       `json:"name,omitempty"`
		PrimaryWindow *UsageWindow `json:"primary_window,omitempty"`
		WeeklyWindow  UsageWindow  `json:"weekly_window,omitempty"`
		UsedPercent   *float64     `json:"used_percent,omitempty"`
		ResetAt       string       `json:"reset_at,omitempty"`
		Status        string       `json:"status,omitempty"`
	}
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}

	b.Name = value.Name
	b.WeeklyWindow = value.WeeklyWindow
	if value.PrimaryWindow != nil {
		b.PrimaryWindow = *value.PrimaryWindow
		return nil
	}
	if value.UsedPercent != nil || value.ResetAt != "" || value.Status != "" {
		b.PrimaryWindow = UsageWindow{
			Name:        "primary_5h",
			UsedPercent: value.UsedPercent,
			ResetAt:     value.ResetAt,
			Status:      value.Status,
		}
		return nil
	}
	b.PrimaryWindow = UsageWindow{}
	return nil
}

type UsageHTTPError struct {
	StatusCode int
	Reason     string
}

func (e *UsageHTTPError) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("codex usage request failed: %s", e.Reason)
}

func (c UsageClient) Fetch(ctx context.Context, usageReq UsageRequest) (UsageSnapshot, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, c.usageURL(), nil)
	if err != nil {
		return UsageSnapshot{}, fmt.Errorf("create Codex usage request: %w", err)
	}
	c.setUsageHeaders(httpReq, usageReq.AccessToken, usageReq.AccountID)

	resp, err := c.httpClient().Do(httpReq)
	if err != nil {
		return UsageSnapshot{}, fmt.Errorf("codex usage request failed: %w", err)
	}
	defer resp.Body.Close()
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return UsageSnapshot{}, &UsageHTTPError{StatusCode: resp.StatusCode, Reason: usageHTTPReason(resp.StatusCode)}
	}
	if readErr != nil {
		return UsageSnapshot{}, fmt.Errorf("read Codex usage response: %w", readErr)
	}
	var raw any
	if err := json.Unmarshal(body, &raw); err != nil {
		return UsageSnapshot{}, fmt.Errorf("parse Codex usage response: %w", err)
	}
	return normalizeUsageSnapshot(raw), nil
}

func (c UsageClient) ConsumeResetCredit(ctx context.Context, resetReq ResetCreditRequest) (ResetCreditResult, error) {
	redeemRequestID := strings.TrimSpace(resetReq.RedeemRequestID)
	if redeemRequestID == "" {
		return ResetCreditResult{}, fmt.Errorf("create Codex reset-credit request: redeem_request_id required")
	}
	body, err := json.Marshal(map[string]string{"redeem_request_id": redeemRequestID})
	if err != nil {
		return ResetCreditResult{}, fmt.Errorf("create Codex reset-credit body: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.resetCreditConsumeURL(), bytes.NewReader(body))
	if err != nil {
		return ResetCreditResult{}, fmt.Errorf("create Codex reset-credit request: %w", err)
	}
	c.setUsageHeaders(httpReq, resetReq.AccessToken, resetReq.AccountID)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient().Do(httpReq)
	if err != nil {
		return ResetCreditResult{}, fmt.Errorf("codex reset-credit request failed: %w", err)
	}
	defer resp.Body.Close()
	responseBody, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return ResetCreditResult{}, &UsageHTTPError{StatusCode: resp.StatusCode, Reason: usageHTTPReason(resp.StatusCode)}
	}
	if readErr != nil {
		return ResetCreditResult{}, fmt.Errorf("read Codex reset-credit response: %w", readErr)
	}
	var raw map[string]any
	if len(strings.TrimSpace(string(responseBody))) > 0 {
		if err := json.Unmarshal(responseBody, &raw); err != nil {
			return ResetCreditResult{}, fmt.Errorf("parse Codex reset-credit response: %w", err)
		}
	}
	return normalizeResetCreditResult(raw), nil
}

func (c UsageClient) usageURL() string {
	return c.whamURL("/wham/usage")
}

func (c UsageClient) resetCreditConsumeURL() string {
	return c.whamURL("/wham/rate-limit-reset-credits/consume")
}

func (c UsageClient) whamURL(path string) string {
	base := strings.TrimRight(strings.TrimSpace(c.BaseURL), "/")
	if base == "" {
		base = "https://chatgpt.com/backend-api"
	}
	if strings.HasSuffix(base, path) {
		return base
	}
	if !strings.HasSuffix(base, "/backend-api") {
		base += "/backend-api"
	}
	return base + path
}

func (c UsageClient) setUsageHeaders(httpReq *http.Request, accessToken, accountID string) {
	httpReq.Header.Set("Authorization", "Bearer "+accessToken)
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("Referer", "https://chatgpt.com/codex/settings/usage")
	httpReq.Header.Set("Originator", c.originator())
	if strings.TrimSpace(accountID) != "" {
		httpReq.Header.Set("ChatGPT-Account-Id", accountID)
	}
}

func (c UsageClient) httpClient() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return http.DefaultClient
}

func (c UsageClient) originator() string {
	if strings.TrimSpace(c.Originator) == "" {
		return "codex_cli_rs"
	}
	return c.Originator
}

func usageHTTPReason(statusCode int) string {
	switch {
	case statusCode == http.StatusUnauthorized:
		return "auth_error"
	case statusCode == http.StatusTooManyRequests:
		return "rate_limited"
	case statusCode >= http.StatusInternalServerError:
		return "upstream_server_error"
	default:
		return "upstream_error"
	}
}

func normalizeResetCreditResult(raw map[string]any) ResetCreditResult {
	code := "unknown"
	if raw != nil {
		if rawCode := firstString(raw, "code", "outcome", "status"); rawCode != "" {
			code = rawCode
		}
	}
	var windowsReset int64
	if raw != nil {
		if value, ok := firstNumber(raw, "windows_reset", "windowsReset"); ok {
			windowsReset = int64(value)
		}
	}
	return ResetCreditResult{Code: code, WindowsReset: windowsReset}
}

func normalizeUsageSnapshot(raw any) UsageSnapshot {
	var s UsageSnapshot
	if root, ok := raw.(map[string]any); ok {
		if rateLimit, ok := usageRateLimitFromMap(root); ok {
			s.RateLimit = &rateLimit
			s.PrimaryWindow = rateLimit.PrimaryWindow
			s.WeeklyWindow = rateLimit.SecondaryWindow
		}
		if resetCredits, ok := usageResetCreditsFromMap(root); ok {
			s.RateLimitResetCredits = &resetCredits
		}
		s.AdditionalBuckets = usageAdditionalBucketsFromMap(root)
	}
	walkUsage(raw, nil, &s)
	return s
}

func usageRateLimitFromMap(root map[string]any) (UsageRateLimit, bool) {
	raw, ok := root["rate_limit"]
	if !ok {
		return UsageRateLimit{}, false
	}
	m, ok := raw.(map[string]any)
	if !ok {
		return UsageRateLimit{}, false
	}
	var rl UsageRateLimit
	if allowed, ok := firstBool(m, "allowed"); ok {
		rl.Allowed = &allowed
	}
	if limitReached, ok := firstBool(m, "limit_reached"); ok {
		rl.LimitReached = &limitReached
	}
	if primary, ok := childMap(m, "primary_window"); ok {
		if window, ok := usageWindowFromMap(primary, []string{"rate_limit", "primary_window"}); ok {
			window.Name = "primary_5h"
			rl.PrimaryWindow = window
		}
	}
	if secondary, ok := childMap(m, "secondary_window"); ok {
		if window, ok := usageWindowFromMap(secondary, []string{"rate_limit", "secondary_window"}); ok {
			window.Name = "weekly"
			rl.SecondaryWindow = window
		}
	}
	return rl, true
}

func usageAdditionalBucketsFromMap(root map[string]any) []UsageBucket {
	raw, ok := root["additional_rate_limits"]
	if !ok {
		return nil
	}
	entries, ok := raw.([]any)
	if !ok {
		return nil
	}

	buckets := make([]UsageBucket, 0, len(entries))
	for _, rawEntry := range entries {
		entry, ok := rawEntry.(map[string]any)
		if !ok {
			continue
		}
		name := usageAdditionalBucketName(entry)
		if name == "" {
			continue
		}

		var bucket UsageBucket
		bucket.Name = name
		if rateLimit, ok := childMap(entry, "rate_limit"); ok {
			if primary, ok := childMap(rateLimit, "primary_window"); ok {
				if window, ok := usageWindowFromMap(primary, []string{"additional_rate_limits", "rate_limit", "primary_window"}); ok {
					window.Name = "primary_5h"
					bucket.PrimaryWindow = window
				}
			}
			if secondary, ok := childMap(rateLimit, "secondary_window"); ok {
				if window, ok := usageWindowFromMap(secondary, []string{"additional_rate_limits", "rate_limit", "secondary_window"}); ok {
					window.Name = "weekly"
					bucket.WeeklyWindow = window
				}
			}
		}
		if bucket.PrimaryWindow.UsedPercent == nil {
			if window, ok := usageWindowFromMap(entry, []string{"additional_rate_limits"}); ok {
				window.Name = "primary_5h"
				bucket.PrimaryWindow = window
			}
		}
		if bucket.PrimaryWindow.UsedPercent == nil && bucket.WeeklyWindow.UsedPercent == nil {
			continue
		}
		buckets = append(buckets, bucket)
	}
	return buckets
}

func usageAdditionalBucketName(entry map[string]any) string {
	for _, key := range []string{"limit_name", "name", "model", "bucket"} {
		if name := redactURLish(firstString(entry, key)); name != "" {
			return name
		}
	}
	return ""
}

func usageResetCreditsFromMap(root map[string]any) (UsageResetCredits, bool) {
	raw, ok := root["rate_limit_reset_credits"]
	if !ok {
		raw, ok = root["rateLimitResetCredits"]
	}
	if !ok {
		return UsageResetCredits{}, false
	}
	m, ok := raw.(map[string]any)
	if !ok {
		return UsageResetCredits{}, false
	}
	available := firstIntPtr(m, "available_count", "availableCount")
	if available == nil {
		return UsageResetCredits{}, false
	}
	return UsageResetCredits{AvailableCount: available}, true
}

func walkUsage(v any, path []string, s *UsageSnapshot) {
	switch typed := v.(type) {
	case map[string]any:
		if s.Email == "" {
			s.Email = firstString(typed, "email", "account_email")
		}
		if s.PlanType == "" {
			s.PlanType = firstString(typed, "plan_type", "planType", "plan")
		}
		if s.CreditsStatus == "" && pathContains(path, "credit", "spend") {
			s.CreditsStatus = firstString(typed, "status", "state")
		}
		if window, ok := usageWindowFromMap(typed, path); ok && !pathContains(path, "additional_rate_limits") {
			switch {
			case isPrimaryWindow(path, typed) && s.PrimaryWindow.UsedPercent == nil:
				window.Name = "primary_5h"
				s.PrimaryWindow = window
			case isWeeklyWindow(path, typed) && s.WeeklyWindow.UsedPercent == nil:
				window.Name = "weekly"
				s.WeeklyWindow = window
			case isAdditionalBucket(path, typed):
				window.Name = "primary_5h"
				s.AdditionalBuckets = append(s.AdditionalBuckets, UsageBucket{
					Name:          bucketName(path, typed),
					PrimaryWindow: window,
				})
			}
		}
		for key, child := range typed {
			walkUsage(child, append(path, key), s)
		}
	case []any:
		for _, child := range typed {
			walkUsage(child, path, s)
		}
	}
}

func usageWindowFromMap(m map[string]any, path []string) (UsageWindow, bool) {
	used, ok := firstUsagePercent(m,
		[]string{"used_percent", "usage_percent", "used_pct", "percent_used"},
		[]string{"utilization", "used"},
	)
	if !ok {
		return UsageWindow{}, false
	}
	return UsageWindow{
		Name:               lastPath(path),
		UsedPercent:        &used,
		ResetAt:            firstResetAt(m),
		ResetAfterSeconds:  firstInt64Ptr(m, "reset_after_seconds"),
		LimitWindowSeconds: firstInt64Ptr(m, "limit_window_seconds"),
		Status:             firstString(m, "status", "state"),
	}, true
}

func childMap(m map[string]any, key string) (map[string]any, bool) {
	raw, ok := m[key]
	if !ok {
		return nil, false
	}
	child, ok := raw.(map[string]any)
	return child, ok
}

func firstBool(m map[string]any, keys ...string) (bool, bool) {
	for _, key := range keys {
		raw, ok := m[key]
		if !ok {
			continue
		}
		value, ok := raw.(bool)
		if ok {
			return value, true
		}
	}
	return false, false
}

func firstInt64Ptr(m map[string]any, keys ...string) *int64 {
	value, ok := firstNumber(m, keys...)
	if !ok {
		return nil
	}
	parsed := int64(value)
	if parsed < 0 {
		return nil
	}
	return &parsed
}

func firstIntPtr(m map[string]any, keys ...string) *int {
	value, ok := firstNumber(m, keys...)
	if !ok {
		return nil
	}
	parsed := int(value)
	if parsed < 0 {
		return nil
	}
	return &parsed
}

func firstResetAt(m map[string]any) string {
	keys := []string{"reset_at", "resets_at", "resetAt", "reset"}
	if value := firstString(m, keys...); value != "" {
		return value
	}
	if value, ok := firstNumber(m, keys...); ok && value > 0 {
		return strconv.FormatInt(int64(value), 10)
	}
	return ""
}

func firstString(m map[string]any, keys ...string) string {
	for _, key := range keys {
		if raw, ok := m[key]; ok {
			switch v := raw.(type) {
			case string:
				if strings.TrimSpace(v) != "" {
					return strings.TrimSpace(v)
				}
			case fmt.Stringer:
				if strings.TrimSpace(v.String()) != "" {
					return strings.TrimSpace(v.String())
				}
			}
		}
	}
	return ""
}

func firstUsagePercent(m map[string]any, percentKeys, ratioKeys []string) (float64, bool) {
	if value, ok := firstNumber(m, percentKeys...); ok {
		return normalizePercentNumber(value), true
	}
	if value, ok := firstNumber(m, ratioKeys...); ok {
		return normalizeRatioNumber(value), true
	}
	return 0, false
}

func firstNumber(m map[string]any, keys ...string) (float64, bool) {
	for _, key := range keys {
		raw, ok := m[key]
		if !ok {
			continue
		}
		var value float64
		switch v := raw.(type) {
		case float64:
			value = v
		case int:
			value = float64(v)
		case json.Number:
			parsed, err := v.Float64()
			if err != nil {
				continue
			}
			value = parsed
		default:
			continue
		}
		return value, true
	}
	return 0, false
}

func normalizePercentNumber(value float64) float64 {
	return clampUsagePercent(value / 100)
}

func normalizeRatioNumber(value float64) float64 {
	if value > 1 {
		value = value / 100
	}
	return clampUsagePercent(value)
}

func clampUsagePercent(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}

func isPrimaryWindow(path []string, m map[string]any) bool {
	if pathContains(path, "primary", "5h", "five_hour", "current") {
		return true
	}
	return strings.Contains(strings.ToLower(firstString(m, "window", "name", "bucket")), "5h")
}

func isWeeklyWindow(path []string, m map[string]any) bool {
	if pathContains(path, "weekly", "7d", "seven_day", "secondary_window") {
		return true
	}
	name := strings.ToLower(firstString(m, "window", "name", "bucket"))
	return strings.Contains(name, "weekly") || strings.Contains(name, "7d")
}

func isAdditionalBucket(path []string, m map[string]any) bool {
	if isPrimaryWindow(path, m) || isWeeklyWindow(path, m) {
		return false
	}
	return pathContains(path, "additional_rate_limits", "additional", "bucket", "model") || firstString(m, "name", "model", "bucket") != ""
}

func bucketName(path []string, m map[string]any) string {
	if name := firstString(m, "name", "model", "bucket"); name != "" {
		return redactURLish(name)
	}
	return redactURLish(lastPath(path))
}

func pathContains(path []string, parts ...string) bool {
	joined := strings.ToLower(strings.Join(path, "."))
	for _, part := range parts {
		if strings.Contains(joined, strings.ToLower(part)) {
			return true
		}
	}
	return false
}

func lastPath(path []string) string {
	if len(path) == 0 {
		return ""
	}
	return path[len(path)-1]
}

func redactURLish(value string) string {
	if _, err := url.ParseRequestURI(value); err == nil && strings.Contains(value, "://") {
		return ""
	}
	return strings.TrimSpace(value)
}
