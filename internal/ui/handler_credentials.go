package ui

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/praxisllmlab/tianjiLLM/internal/callback"
	"github.com/praxisllmlab/tianjiLLM/internal/db"
	"github.com/praxisllmlab/tianjiLLM/internal/provider/chatgptcodex"
	proxyhandler "github.com/praxisllmlab/tianjiLLM/internal/proxy/handler"
	"github.com/praxisllmlab/tianjiLLM/internal/security/redact"
	"github.com/praxisllmlab/tianjiLLM/internal/ui/components/toast"
	"github.com/praxisllmlab/tianjiLLM/internal/ui/pages"
)

const openAISubscriptionCredentialType = "openai_subscription"

type openAISubscriptionInfo struct {
	Email           string     `json:"email,omitempty"`
	Status          string     `json:"status,omitempty"`
	LastRefreshAt   *time.Time `json:"last_refresh_at,omitempty"`
	LastError       string     `json:"last_error,omitempty"`
	DisabledReason  string     `json:"disabled_reason,omitempty"`
	OperatorMessage string     `json:"operator_message,omitempty"`
}

type credentialActionResponse struct {
	Status        string `json:"status"`
	ReasonCode    string `json:"reason_code,omitempty"`
	ModelsCount   *int   `json:"models_count,omitempty"`
	LastRefreshAt string `json:"last_refresh_at,omitempty"`
}

func (h *UIHandler) handleCredentials(w http.ResponseWriter, r *http.Request) {
	render(r.Context(), w, pages.CredentialsPage(h.loadCredentialsPageData(r)))
}

func (h *UIHandler) handleCredentialDetail(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		http.Error(w, "database not configured", http.StatusServiceUnavailable)
		return
	}

	credentialID := chi.URLParam(r, "credential_id")
	if credentialID == "" {
		http.NotFound(w, r)
		return
	}

	cred, err := h.DB.GetCredential(r.Context(), credentialID)
	if err != nil || cred.CredentialType != openAISubscriptionCredentialType {
		http.NotFound(w, r)
		return
	}

	render(r.Context(), w, pages.CredentialDetailPage(h.buildCredentialDetail(r.Context(), cred, time.Now())))
}

func (h *UIHandler) handleCredentialTest(w http.ResponseWriter, r *http.Request) {
	h.handleCredentialAction(w, r, "test")
}

func (h *UIHandler) handleCredentialRefresh(w http.ResponseWriter, r *http.Request) {
	h.handleCredentialAction(w, r, "refresh")
}

func (h *UIHandler) handleCredentialDisable(w http.ResponseWriter, r *http.Request) {
	h.handleCredentialAction(w, r, "disable")
}

func (h *UIHandler) handleCredentialEnable(w http.ResponseWriter, r *http.Request) {
	h.handleCredentialAction(w, r, "enable")
}

func (h *UIHandler) handleCredentialCodexUsageRefresh(w http.ResponseWriter, r *http.Request) {
	credentialID := chi.URLParam(r, "credential_id")
	if credentialID == "" {
		http.NotFound(w, r)
		return
	}
	proxy := h.proxyLifecycleHandlers()
	result := proxy.OpenAISubscriptionCodexUsageSnapshot(r.Context(), credentialID, true)
	if h.DB != nil && isCredentialDetailAction(r, credentialID) {
		if cred, err := h.DB.GetCredential(r.Context(), credentialID); err == nil && cred.CredentialType == openAISubscriptionCredentialType {
			render(r.Context(), w, pages.CredentialDetailContentWithToast(h.buildCredentialDetailWithCodexUsage(cred, time.Now(), result), codexUsageRefreshMessage(result), toast.VariantDefault))
			return
		}
	}
	data := h.loadCodexUsageData(r)
	render(r.Context(), w, pages.UsageCodexTab(data))
}

func (h *UIHandler) handleCredentialCodexUsageReset(w http.ResponseWriter, r *http.Request) {
	credentialID := chi.URLParam(r, "credential_id")
	if credentialID == "" {
		http.NotFound(w, r)
		return
	}
	proxy := h.proxyLifecycleHandlers()
	result := proxy.OpenAISubscriptionCodexUsageReset(r.Context(), credentialID)
	msg := codexUsageResetMessage(result)
	variant := codexUsageResetToastVariant(result)
	if h.DB != nil && isCredentialDetailAction(r, credentialID) {
		if cred, err := h.DB.GetCredential(r.Context(), credentialID); err == nil && cred.CredentialType == openAISubscriptionCredentialType {
			render(r.Context(), w, pages.CredentialDetailContentWithToast(h.buildCredentialDetailWithCodexUsage(cred, time.Now(), result.Usage), msg, variant))
			return
		}
	}
	data := h.loadCodexUsageData(r)
	render(r.Context(), w, pages.UsageCodexTabWithToast(data, msg, variant))
}

func (h *UIHandler) handleCredentialDelete(w http.ResponseWriter, r *http.Request) {
	h.handleCredentialAction(w, r, "delete")
}

func (h *UIHandler) handleCredentialAction(w http.ResponseWriter, r *http.Request, action string) {
	credentialID := chi.URLParam(r, "credential_id")
	if credentialID == "" {
		http.NotFound(w, r)
		return
	}

	status, result := h.invokeCredentialLifecycle(r, credentialID, action)
	if status >= 200 && status < 300 {
		h.invalidateCodexUsageForCredentialAction(credentialID, action)
		if action == "delete" {
			h.renderCredentialActionResult(w, r, credentialID, action, "Credential deleted", toast.VariantDefault)
			return
		}
		h.renderCredentialActionResult(w, r, credentialID, action, credentialActionSuccessMessage(action, result), toast.VariantDefault)
		return
	}

	h.renderCredentialActionResult(w, r, credentialID, action, credentialActionErrorMessage(action, result), toast.VariantError)
}

func (h *UIHandler) loadCredentialsPageData(r *http.Request) pages.CredentialsPageData {
	data := pages.CredentialsPageData{}
	data.DeviceCSRFToken, _ = h.openAIDeviceCSRFToken(r)
	if h.DB == nil {
		return data
	}

	creds, err := h.DB.ListCredentials(r.Context())
	if err != nil {
		return data
	}

	now := time.Now()
	for _, cred := range creds {
		if cred.CredentialType != openAISubscriptionCredentialType {
			continue
		}
		data.Rows = append(data.Rows, h.buildCredentialRow(cred, now))
	}
	data.TotalCount = len(data.Rows)
	data.ConnectURL, data.ConnectDisabledReason = h.openAIConnectURL(r)
	return data
}

func (h *UIHandler) renderCredentialActionResult(w http.ResponseWriter, r *http.Request, credentialID, action, msg string, variant toast.Variant) {
	if isCodexUsageTabAction(r) {
		render(r.Context(), w, pages.UsageCodexTabWithToast(h.loadCodexUsageData(r), msg, variant))
		return
	}
	if h.DB != nil && isCredentialDetailAction(r, credentialID) && action != "delete" {
		if cred, err := h.DB.GetCredential(r.Context(), credentialID); err == nil && cred.CredentialType == openAISubscriptionCredentialType {
			render(r.Context(), w, pages.CredentialDetailContentWithToast(h.buildCredentialDetail(r.Context(), cred, time.Now()), msg, variant))
			return
		}
	}
	render(r.Context(), w, pages.CredentialsTableWithToast(h.loadCredentialsPageData(r), msg, variant))
}

func isCodexUsageTabAction(r *http.Request) bool {
	return strings.TrimSpace(r.Header.Get("HX-Target")) == "codex-usage-tab-content"
}

func isCredentialDetailAction(r *http.Request, credentialID string) bool {
	target := strings.TrimSpace(r.Header.Get("HX-Target"))
	if target == "credential-detail-content" {
		return true
	}
	ref := r.Referer()
	return strings.Contains(ref, "/ui/credentials/"+credentialID)
}

func (h *UIHandler) invalidateCodexUsageForCredentialAction(credentialID, action string) {
	switch action {
	case "disable", "enable", "delete":
		if h != nil && h.CodexUsageCache != nil {
			h.CodexUsageCache.Invalidate(credentialID)
		}
	}
}

func (h *UIHandler) invokeCredentialLifecycle(r *http.Request, credentialID, action string) (int, credentialActionResponse) {
	proxy := h.proxyLifecycleHandlers()
	method := http.MethodPost
	path := "/credentials/openai-subscription/" + credentialID + "/" + action
	if action == "delete" {
		method = http.MethodDelete
		path = "/credentials/delete/" + credentialID
	}

	req := r.Clone(r.Context())
	req.Method = method
	req.URL = &url.URL{Path: path}
	req.Header = make(http.Header)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("credential_id", credentialID)
	req = req.WithContext(contextWithChiRoute(req.Context(), routeCtx))

	rr := &credentialActionRecorder{header: make(http.Header), status: http.StatusOK}
	switch action {
	case "test":
		proxy.OpenAISubscriptionCredentialTest(rr, req)
	case "refresh":
		proxy.OpenAISubscriptionCredentialRefresh(rr, req)
	case "disable":
		proxy.OpenAISubscriptionCredentialDisable(rr, req)
	case "enable":
		proxy.OpenAISubscriptionCredentialEnable(rr, req)
	case "delete":
		proxy.CredentialDelete(rr, req)
	default:
		rr.WriteHeader(http.StatusBadRequest)
	}

	var result credentialActionResponse
	_ = json.Unmarshal(rr.body.Bytes(), &result)
	return rr.status, result
}

func contextWithChiRoute(ctx context.Context, routeCtx *chi.Context) context.Context {
	return context.WithValue(ctx, chi.RouteCtxKey, routeCtx)
}

type credentialActionRecorder struct {
	header http.Header
	body   bytes.Buffer
	status int
}

func (r *credentialActionRecorder) Header() http.Header {
	return r.header
}

func (r *credentialActionRecorder) Write(body []byte) (int, error) {
	return r.body.Write(body)
}

func (r *credentialActionRecorder) WriteHeader(statusCode int) {
	r.status = statusCode
}

func (h *UIHandler) proxyLifecycleHandlers() *proxyhandler.Handlers {
	if h.CodexUsageCache == nil {
		h.CodexUsageCache = proxyhandler.NewOpenAISubscriptionCodexUsageCache()
	}
	proxy := &proxyhandler.Handlers{
		Config:                   h.Config,
		Cache:                    h.Cache,
		RateLimitStore:           h.RateLimitStore,
		DisabledTokens:           h.DisabledTokens,
		OpenAIOAuthHTTPClient:    h.OpenAIOAuthHTTPClient,
		OpenAIUpstreamHTTPClient: h.OpenAIUpstreamHTTPClient,
		OpenAIUpstreamBaseURL:    h.OpenAIUpstreamBaseURL,
		CodexUsageFetcher:        h.CodexUsageFetcher,
		CodexResetConsumer:       h.CodexResetConsumer,
		CodexUsageCache:          h.CodexUsageCache,
	}
	if h.DB != nil {
		proxy.DB = h.DB
	}
	return proxy
}

func credentialActionSuccessMessage(action string, result credentialActionResponse) string {
	switch action {
	case "test":
		if result.ModelsCount != nil {
			return "Credential test succeeded: " + pages.FormatCount(*result.ModelsCount) + " model(s) available"
		}
		return "Credential test succeeded"
	case "refresh":
		if strings.TrimSpace(result.LastRefreshAt) != "" {
			return "Credential refreshed at " + result.LastRefreshAt
		}
		return "Credential refreshed"
	case "disable":
		return "Credential disabled"
	case "enable":
		return "Credential enabled"
	default:
		return "Credential action completed"
	}
}

func credentialActionErrorMessage(action string, result credentialActionResponse) string {
	reason := strings.TrimSpace(redact.String(result.ReasonCode))
	if reason == "" {
		reason = "action_failed"
	}
	switch action {
	case "test":
		return "Credential test failed: " + reason
	case "refresh":
		return "Credential refresh failed: " + reason
	case "disable":
		return "Credential disable failed: " + reason
	case "enable":
		return "Credential enable failed: " + reason
	case "delete":
		return "Credential delete failed: " + reason
	default:
		return "Credential action failed: " + reason
	}
}

func (h *UIHandler) openAIConnectURL(r *http.Request) (string, string) {
	if h.Config == nil {
		return "", "OpenAI OAuth is not configured"
	}
	if !h.Config.GeneralSettings.OpenAIOAuth.Enabled {
		return "", proxyhandler.ErrOpenAIConnectDisabled.Error()
	}
	if h.DB == nil {
		return "", "Database is not configured"
	}
	return "/ui/openai/connect?scope=global", ""
}

func (h *UIHandler) buildCredentialRow(cred db.CredentialTable, now time.Time) pages.CredentialRow {
	info := parseOpenAISubscriptionInfo(cred.CredentialInfo)
	quota, hasQuota := h.openAIQuotaState(cred.CredentialID, now)
	quotaBadge := quotaStatusBadge(quota.Status, hasQuota)

	return pages.CredentialRow{
		ID:                    cred.CredentialID,
		Name:                  cred.CredentialName,
		Email:                 fallback(info.Email, "Unknown"),
		OrganizationID:        fallbackPtr(cred.OrganizationID, "Master"),
		CredentialStatus:      credentialStatusLabel(info),
		CredentialVariant:     credentialStatusVariant(info),
		QuotaStatus:           quotaBadge.Label,
		QuotaStatusVariant:    quotaBadge.Variant,
		QuotaSummary:          quotaSummary(quota, hasQuota),
		LastRefresh:           formatOptionalTime(info.LastRefreshAt),
		LastError:             fallback(info.LastError, "None"),
		CreatedAt:             formatPgTime(cred.CreatedAt.Valid, cred.CreatedAt.Time),
		UpdatedAt:             formatPgTime(cred.UpdatedAt.Valid, cred.UpdatedAt.Time),
		LifecycleAction:       credentialLifecycleAction(info),
		LifecycleActionLabel:  credentialLifecycleActionLabel(info),
		LifecycleActionPrompt: credentialLifecycleActionPrompt(info),
	}
}

func (h *UIHandler) buildCredentialDetail(ctx context.Context, cred db.CredentialTable, now time.Time) pages.CredentialDetailData {
	result := h.proxyLifecycleHandlers().OpenAISubscriptionCodexUsageSnapshot(ctx, cred.CredentialID, false)
	return h.buildCredentialDetailWithCodexUsage(cred, now, result)
}

func (h *UIHandler) buildCredentialDetailWithCodexUsage(cred db.CredentialTable, now time.Time, codexUsage proxyhandler.OpenAISubscriptionCodexUsageResult) pages.CredentialDetailData {
	info := parseOpenAISubscriptionInfo(cred.CredentialInfo)
	quota, hasQuota := h.openAIQuotaState(cred.CredentialID, now)
	quotaBadge := quotaStatusBadge(quota.Status, hasQuota)

	return pages.CredentialDetailData{
		ID:                 cred.CredentialID,
		Name:               cred.CredentialName,
		Email:              fallback(info.Email, "Unknown"),
		OrganizationID:     fallbackPtr(cred.OrganizationID, "Master"),
		CredentialStatus:   credentialStatusLabel(info),
		CredentialVariant:  credentialStatusVariant(info),
		QuotaStatus:        quotaBadge.Label,
		QuotaStatusVariant: quotaBadge.Variant,
		QuotaSummary:       quotaSummary(quota, hasQuota),
		LastRefresh:        formatOptionalTime(info.LastRefreshAt),
		LastError:          fallback(info.LastError, "None"),
		DisabledReason:     fallback(info.DisabledReason, "None"),
		CreatedAt:          formatPgTime(cred.CreatedAt.Valid, cred.CreatedAt.Time),
		UpdatedAt:          formatPgTime(cred.UpdatedAt.Valid, cred.UpdatedAt.Time),
		HasQuota:           hasQuota,
		QuotaUpdatedAt:     formatOptionalTime(timePtr(quota.UpdatedAt)),
		QuotaReset:         quotaResetLabel(quota),
		Dimensions: []pages.QuotaDimensionView{
			quotaDimensionView("Requests", quota.Requests),
			quotaDimensionView("Tokens", quota.Tokens),
		},
		CodexUsage:            h.codexUsageCardForCredential(cred, codexUsage, now),
		LifecycleAction:       credentialLifecycleAction(info),
		LifecycleActionLabel:  credentialLifecycleActionLabel(info),
		LifecycleActionPrompt: credentialLifecycleActionPrompt(info),
	}
}

func (h *UIHandler) codexUsageCardForCredential(cred db.CredentialTable, result proxyhandler.OpenAISubscriptionCodexUsageResult, now time.Time) pages.CodexUsageCredentialCard {
	info := parseOpenAISubscriptionInfo(cred.CredentialInfo)
	primaryWindow, weeklyWindow := codexUsageDisplayWindows(result.Snapshot)
	card := pages.CodexUsageCredentialCard{
		CredentialID:          cred.CredentialID,
		Name:                  cred.CredentialName,
		Email:                 fallback(result.Snapshot.Email, fallback(info.Email, "Unknown")),
		OrganizationID:        fallbackPtr(cred.OrganizationID, "Master"),
		PlanType:              result.Snapshot.PlanType,
		StatusLabel:           codexUsageStatusLabel(result),
		StatusVariant:         codexUsageStatusVariant(result),
		PrimaryWindow:         codexUsageWindowView("Primary 5h", primaryWindow),
		WeeklyWindow:          codexUsageWindowView("Weekly", weeklyWindow),
		CreditsStatus:         result.Snapshot.CreditsStatus,
		ResetCreditsAvailable: codexUsageResetCreditsAvailable(result.Snapshot),
		LastUpdated:           formatOptionalTime(timePtr(result.FetchedAt)),
		CacheState:            codexUsageCacheState(result, now),
		SafeError:             codexUsageSafeError(result),
		RefreshDisabledReason: codexUsageRefreshDisabledReason(result, now),
		LifecycleAction:       credentialLifecycleAction(info),
		LifecycleActionLabel:  credentialLifecycleActionLabel(info),
		LifecycleActionPrompt: credentialLifecycleActionPrompt(info),
		SelectionScore:        codexUsageSelectionScore(result, codexUsagePrimaryGate(h), codexUsageWeeklyGate(h), now),
	}
	for _, bucket := range result.Snapshot.AdditionalBuckets {
		card.AdditionalBuckets = append(card.AdditionalBuckets, pages.CodexUsageBucketView{
			Name:          fallback(bucket.Name, "Additional bucket"),
			PrimaryWindow: codexUsageWindowView("5-hour limit", bucket.PrimaryWindow),
			WeeklyWindow:  codexUsageWindowView("Weekly limit", bucket.WeeklyWindow),
		})
	}
	return card
}

const codexWeeklyUsageWindowSeconds int64 = 7 * 24 * 60 * 60

func codexUsageDisplayWindows(snapshot chatgptcodex.UsageSnapshot) (chatgptcodex.UsageWindow, chatgptcodex.UsageWindow) {
	primaryWindow := snapshot.PrimaryWindow
	weeklyWindow := snapshot.WeeklyWindow

	// OpenAI temporarily exposes the 7-day limit as primary_window when the
	// normal 5-hour limit is disabled. Keep the UI grouped by the actual
	// window duration rather than the upstream primary/secondary field names.
	if primaryWindow.LimitWindowSeconds != nil && *primaryWindow.LimitWindowSeconds == codexWeeklyUsageWindowSeconds {
		if weeklyWindow.UsedPercent == nil {
			weeklyWindow = primaryWindow
		}
		primaryWindow = chatgptcodex.UsageWindow{}
	}

	return primaryWindow, weeklyWindow
}

func codexUsageResetCreditsAvailable(snapshot chatgptcodex.UsageSnapshot) *int {
	if snapshot.RateLimitResetCredits == nil {
		return nil
	}
	return snapshot.RateLimitResetCredits.AvailableCount
}

func codexUsageWindowView(label string, window chatgptcodex.UsageWindow) pages.CodexUsageWindowView {
	return pages.CodexUsageWindowView{
		Name:        label,
		UsedPercent: window.UsedPercent,
		Reset:       window.ResetAt,
		Status:      window.Status,
	}
}

func (h *UIHandler) openAIQuotaState(credentialID string, now time.Time) (callback.OpenAIQuotaState, bool) {
	if h.RateLimitStore == nil {
		return callback.OpenAIQuotaState{}, false
	}
	return h.RateLimitStore.GetOpenAIQuotaState(credentialID, now)
}

func parseOpenAISubscriptionInfo(raw []byte) openAISubscriptionInfo {
	var info openAISubscriptionInfo
	if len(raw) == 0 {
		return info
	}
	if err := json.Unmarshal(raw, &info); err != nil {
		return openAISubscriptionInfo{}
	}
	info.Email = strings.TrimSpace(redact.String(info.Email))
	info.Status = strings.TrimSpace(redact.String(info.Status))
	info.LastError = strings.TrimSpace(redact.String(info.LastError))
	info.DisabledReason = strings.TrimSpace(redact.String(info.DisabledReason))
	info.OperatorMessage = strings.TrimSpace(redact.String(info.OperatorMessage))
	return info
}

func credentialStatusLabel(info openAISubscriptionInfo) string {
	status := strings.ToLower(strings.TrimSpace(info.Status))
	switch {
	case info.OperatorMessage != "":
		return info.OperatorMessage
	case status == "refresh_failed" || status == "auth_failed":
		return "Refresh failed"
	case status == "disabled" || info.DisabledReason != "":
		return "Disabled"
	case status == "active" || status == "allowed":
		return "Active"
	case status != "":
		return strings.ReplaceAll(status, "_", " ")
	default:
		return "Unknown"
	}
}

func credentialStatusVariant(info openAISubscriptionInfo) string {
	status := strings.ToLower(strings.TrimSpace(info.Status))
	switch {
	case status == "disabled" || info.DisabledReason != "":
		return "destructive"
	case status == "refresh_failed" || status == "auth_failed":
		return "outline"
	case status == "active" || status == "allowed":
		return "default"
	default:
		return "secondary"
	}
}

func credentialLifecycleAction(info openAISubscriptionInfo) string {
	if credentialIsDisabled(info) {
		return "enable"
	}
	return "disable"
}

func credentialLifecycleActionLabel(info openAISubscriptionInfo) string {
	if credentialIsDisabled(info) {
		return "Enable"
	}
	return "Disable"
}

func credentialLifecycleActionPrompt(info openAISubscriptionInfo) string {
	if credentialIsDisabled(info) {
		return ""
	}
	return "Disable this OpenAI subscription credential?"
}

func credentialIsDisabled(info openAISubscriptionInfo) bool {
	return strings.EqualFold(strings.TrimSpace(info.Status), "disabled") || strings.TrimSpace(info.DisabledReason) != ""
}

type statusBadgeView struct {
	Label   string
	Variant string
}

func quotaStatusBadge(status callback.OpenAIQuotaStatus, known bool) statusBadgeView {
	if !known {
		return statusBadgeView{Label: "Unknown", Variant: "secondary"}
	}
	switch status {
	case callback.OpenAIQuotaStatusAllowed:
		return statusBadgeView{Label: "Allowed", Variant: "default"}
	case callback.OpenAIQuotaStatusRejected:
		return statusBadgeView{Label: "Rejected", Variant: "destructive"}
	case callback.OpenAIQuotaStatusExhausted:
		return statusBadgeView{Label: "Exhausted", Variant: "destructive"}
	default:
		return statusBadgeView{Label: "Unknown", Variant: "secondary"}
	}
}

func quotaSummary(state callback.OpenAIQuotaState, known bool) string {
	if !known {
		return "No upstream quota data"
	}
	if state.QuotaUtilizationKnown {
		return formatPercent(state.QuotaUtilization) + " quota used"
	}
	if state.Requests.UtilizationKnown {
		return formatPercent(state.Requests.Utilization) + " requests used"
	}
	if state.Tokens.UtilizationKnown {
		return formatPercent(state.Tokens.Utilization) + " tokens used"
	}
	return "Quota status recorded"
}

func quotaDimensionView(name string, d callback.OpenAIQuotaDimension) pages.QuotaDimensionView {
	utilization, utilizationKnown := d.Utilization, d.UtilizationKnown
	if !utilizationKnown && d.LimitKnown && d.RemainingKnown {
		utilization, utilizationKnown = callback.DeriveOpenAIQuotaUtilization(d.Limit, d.Remaining)
	}

	return pages.QuotaDimensionView{
		Name:               name,
		Limit:              formatKnownInt(d.Limit, d.LimitKnown),
		Remaining:          formatKnownInt(d.Remaining, d.RemainingKnown),
		Reset:              formatKnownTime(d.ResetAt, d.ResetKnown),
		Utilization:        formatKnownPercent(utilization, utilizationKnown),
		UtilizationPercent: callback.ClampOpenAIQuotaUtilization(utilization) * 100,
		UtilizationKnown:   utilizationKnown,
	}
}

func quotaResetLabel(state callback.OpenAIQuotaState) string {
	if state.QuotaResetKnown {
		return formatTime(state.QuotaResetAt)
	}
	if state.Requests.ResetKnown {
		return formatTime(state.Requests.ResetAt)
	}
	if state.Tokens.ResetKnown {
		return formatTime(state.Tokens.ResetAt)
	}
	return "Not recorded"
}

func formatKnownInt(value int, known bool) string {
	if !known {
		return "Unknown"
	}
	return pages.FormatCount(value)
}

func formatKnownPercent(value float64, known bool) string {
	if !known {
		return "Unknown"
	}
	return formatPercent(value)
}

func formatPercent(value float64) string {
	return pages.FormatPercent(callback.ClampOpenAIQuotaUtilization(value))
}

func formatKnownTime(value time.Time, known bool) string {
	if !known || value.IsZero() {
		return "Not recorded"
	}
	return formatTime(value)
}

func formatOptionalTime(value *time.Time) string {
	if value == nil || value.IsZero() {
		return "Not recorded"
	}
	return formatTime(*value)
}

func formatPgTime(valid bool, value time.Time) string {
	if !valid || value.IsZero() {
		return "Not recorded"
	}
	return formatTime(value)
}

func formatTime(value time.Time) string {
	return value.UTC().Format("2006-01-02 15:04 UTC")
}

func fallback(value, unknown string) string {
	if strings.TrimSpace(value) == "" {
		return unknown
	}
	return value
}

func fallbackPtr(value *string, unknown string) string {
	if value == nil || strings.TrimSpace(*value) == "" {
		return unknown
	}
	return *value
}

func timePtr(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	return &value
}
