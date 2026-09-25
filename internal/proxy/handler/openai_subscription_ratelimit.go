package handler

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/praxisllmlab/tianjiLLM/internal/callback"
)

const (
	openAIMockQuotaStatusHeader      = "x-openai-mock-quota-status"
	openAIMockQuotaResetHeader       = "x-openai-mock-quota-reset"
	openAIMockQuotaUtilizationHeader = "x-openai-mock-quota-utilization"
)

func parseOpenAIRateLimitHeaders(headers http.Header, subjectID string, now time.Time) (callback.OpenAIQuotaState, bool) {
	state := callback.OpenAIQuotaState{SubjectID: subjectID}
	updated := false

	if v, ok := parseOpenAIRateLimitInt(headers.Get("x-ratelimit-limit-requests")); ok {
		state.Requests.Limit = v
		state.Requests.LimitKnown = true
		updated = true
	}
	if v, ok := parseOpenAIRateLimitInt(headers.Get("x-ratelimit-remaining-requests")); ok {
		state.Requests.Remaining = v
		state.Requests.RemainingKnown = true
		updated = true
	}
	if v, ok := parseOpenAIRateLimitReset(headers.Get("x-ratelimit-reset-requests"), now); ok {
		state.Requests.ResetAt = v
		state.Requests.ResetKnown = true
		updated = true
	}
	if v, ok := parseOpenAIRateLimitInt(headers.Get("x-ratelimit-limit-tokens")); ok {
		state.Tokens.Limit = v
		state.Tokens.LimitKnown = true
		updated = true
	}
	if v, ok := parseOpenAIRateLimitInt(headers.Get("x-ratelimit-remaining-tokens")); ok {
		state.Tokens.Remaining = v
		state.Tokens.RemainingKnown = true
		updated = true
	}
	if v, ok := parseOpenAIRateLimitReset(headers.Get("x-ratelimit-reset-tokens"), now); ok {
		state.Tokens.ResetAt = v
		state.Tokens.ResetKnown = true
		updated = true
	}
	if utilization, ok := callback.DeriveOpenAIQuotaUtilization(state.Requests.Limit, state.Requests.Remaining); ok && state.Requests.LimitKnown && state.Requests.RemainingKnown {
		state.Requests.Utilization = utilization
		state.Requests.UtilizationKnown = true
	}
	if utilization, ok := callback.DeriveOpenAIQuotaUtilization(state.Tokens.Limit, state.Tokens.Remaining); ok && state.Tokens.LimitKnown && state.Tokens.RemainingKnown {
		state.Tokens.Utilization = utilization
		state.Tokens.UtilizationKnown = true
	}
	if state.Requests.RemainingKnown && state.Requests.Remaining == 0 || state.Tokens.RemainingKnown && state.Tokens.Remaining == 0 {
		state.Status = callback.OpenAIQuotaStatusExhausted
	}
	if status, ok := parseOpenAIMockQuotaStatus(headers.Get(openAIMockQuotaStatusHeader)); ok {
		state.Status = status
		updated = true
	}
	if v, ok := parseOpenAIRateLimitReset(headers.Get(openAIMockQuotaResetHeader), now); ok {
		state.QuotaResetAt = v
		state.QuotaResetKnown = true
		updated = true
	}
	if v, ok := parseOpenAIMockQuotaUtilization(headers.Get(openAIMockQuotaUtilizationHeader)); ok {
		state.QuotaUtilization = v
		state.QuotaUtilizationKnown = true
		updated = true
	}
	if updated {
		state.UpdatedAt = now
	}
	return state, updated
}

func parseOpenAIRateLimitInt(raw string) (int, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, false
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, false
	}
	return value, true
}

func parseOpenAIRateLimitReset(raw string, now time.Time) (time.Time, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, false
	}
	if d, err := time.ParseDuration(raw); err == nil {
		return now.Add(d), true
	}
	if ts, err := time.Parse(time.RFC3339, raw); err == nil {
		return ts, true
	}
	return time.Time{}, false
}

func parseOpenAIMockQuotaStatus(raw string) (callback.OpenAIQuotaStatus, bool) {
	switch strings.TrimSpace(strings.ToLower(raw)) {
	case "allowed":
		return callback.OpenAIQuotaStatusAllowed, true
	case "rejected":
		return callback.OpenAIQuotaStatusRejected, true
	case "exhausted":
		return callback.OpenAIQuotaStatusExhausted, true
	default:
		return callback.OpenAIQuotaStatusUnknown, false
	}
}

func parseOpenAIMockQuotaUtilization(raw string) (float64, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, false
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, false
	}
	return callback.ClampOpenAIQuotaUtilization(value), true
}

func (h *Handlers) recordOpenAISubscriptionRateLimit(credentialID string, state callback.OpenAIQuotaState) {
	if credentialID == "" {
		return
	}
	h.openAIQuotaStore().SetOpenAIQuotaState(credentialID, state)
}

func (h *Handlers) openAIQuotaStore() callback.RateLimitStore {
	h.openAISubscriptionRoutingMu.Lock()
	defer h.openAISubscriptionRoutingMu.Unlock()
	if h.RateLimitStore == nil {
		h.RateLimitStore = callback.NewInMemoryRateLimitStore()
	}
	return h.RateLimitStore
}

func (h *Handlers) openAISubscriptionRateLimitState(credentialID string) (callback.OpenAIQuotaState, bool) {
	return h.openAIQuotaStore().GetOpenAIQuotaState(credentialID, h.openAISubscriptionNowUTC())
}
