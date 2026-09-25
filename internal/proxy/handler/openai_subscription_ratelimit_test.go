package handler

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseOpenAIRateLimitHeaders_RequestsExhausted(t *testing.T) {
	now := time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)
	headers := http.Header{}
	headers.Set("x-ratelimit-limit-requests", "100")
	headers.Set("x-ratelimit-remaining-requests", "0")
	headers.Set("x-ratelimit-reset-requests", "60s")

	state, ok := parseOpenAIRateLimitHeaders(headers, "cred-a", now)

	require.True(t, ok)
	assert.Equal(t, 100, state.Requests.Limit)
	assert.Equal(t, 0, state.Requests.Remaining)
	assert.True(t, state.Gated(now))
	assert.False(t, state.Gated(now.Add(61*time.Second)))
}

func TestParseOpenAIRateLimitHeaders_TokensExhausted(t *testing.T) {
	now := time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)
	headers := http.Header{}
	headers.Set("x-ratelimit-limit-tokens", "200000")
	headers.Set("x-ratelimit-remaining-tokens", "0")
	headers.Set("x-ratelimit-reset-tokens", "6m0s")

	state, ok := parseOpenAIRateLimitHeaders(headers, "cred-a", now)

	require.True(t, ok)
	assert.Equal(t, 200000, state.Tokens.Limit)
	assert.Equal(t, 0, state.Tokens.Remaining)
	assert.True(t, state.Gated(now.Add(5*time.Minute)))
	assert.False(t, state.Gated(now.Add(7*time.Minute)))
}

func TestParseOpenAIRateLimitHeaders_MissingHeadersNoGate(t *testing.T) {
	state, ok := parseOpenAIRateLimitHeaders(http.Header{}, "cred-a", time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC))

	assert.False(t, ok)
	assert.False(t, state.Gated(time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)))
}

func TestParseOpenAIRateLimitHeaders_DerivesUtilizationAndClamps(t *testing.T) {
	now := time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)
	headers := http.Header{}
	headers.Set("x-ratelimit-limit-requests", "100")
	headers.Set("x-ratelimit-remaining-requests", "25")
	headers.Set("x-ratelimit-limit-tokens", "100")
	headers.Set("x-ratelimit-remaining-tokens", "125")

	state, ok := parseOpenAIRateLimitHeaders(headers, "cred-a", now)

	require.True(t, ok)
	assert.True(t, state.Requests.UtilizationKnown)
	assert.Equal(t, 0.75, state.Requests.Utilization)
	assert.True(t, state.Tokens.UtilizationKnown)
	assert.Equal(t, 0.0, state.Tokens.Utilization)
	assert.False(t, state.Gated(now))
}

func TestParseOpenAIRateLimitHeaders_MalformedValuesStayUnknown(t *testing.T) {
	now := time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)
	headers := http.Header{}
	headers.Set("x-ratelimit-limit-requests", "not-a-number")
	headers.Set("x-ratelimit-remaining-requests", "0")
	headers.Set("x-ratelimit-reset-requests", "not-a-duration")

	state, ok := parseOpenAIRateLimitHeaders(headers, "cred-a", now)

	require.True(t, ok)
	assert.False(t, state.Requests.LimitKnown)
	assert.True(t, state.Requests.RemainingKnown)
	assert.False(t, state.Requests.ResetKnown)
	assert.False(t, state.Gated(now), "remaining zero without a future reset must not gate forever")
}

func TestParseOpenAIRateLimitHeaders_MockQuotaRejectedAndUtilization(t *testing.T) {
	now := time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)
	headers := http.Header{}
	headers.Set("x-openai-mock-quota-status", "rejected")
	headers.Set("x-openai-mock-quota-reset", "2m")
	headers.Set("x-openai-mock-quota-utilization", "1.4")

	state, ok := parseOpenAIRateLimitHeaders(headers, "cred-a", now)

	require.True(t, ok)
	assert.Equal(t, "cred-a", state.SubjectID)
	assert.Equal(t, "rejected", string(state.Status))
	assert.True(t, state.QuotaResetKnown)
	assert.Equal(t, now.Add(2*time.Minute), state.QuotaResetAt)
	assert.True(t, state.QuotaUtilizationKnown)
	assert.Equal(t, 1.0, state.QuotaUtilization)
	assert.True(t, state.Gated(now))
	assert.False(t, state.Gated(now.Add(3*time.Minute)))
}

func TestParseOpenAIRateLimitHeaders_DoesNotPersistMalformedTokenMaterial(t *testing.T) {
	now := time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)
	headers := http.Header{}
	headers.Set("x-ratelimit-limit-requests", "Bearer sk-secret-token-material")
	headers.Set("x-ratelimit-remaining-requests", "0")
	headers.Set("x-ratelimit-reset-requests", "60s")

	state, ok := parseOpenAIRateLimitHeaders(headers, "cred-a", now)

	require.True(t, ok)
	assert.NotContains(t, fmt.Sprint(state), "sk-secret-token-material")
	assert.False(t, strings.Contains(fmt.Sprint(state), "Bearer "))
}
