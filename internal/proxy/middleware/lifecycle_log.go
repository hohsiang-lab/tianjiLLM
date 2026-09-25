package middleware

import (
	"context"
	"time"

	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog/log"
)

// LogProviderResolved emits a "provider.resolved" lifecycle log event.
// Call this in each handler after the provider has been resolved and auth is complete.
func LogProviderResolved(ctx context.Context, providerName, upstreamURL, handlerType, model string) {
	requestID := chiMiddleware.GetReqID(ctx)

	evt := log.Info().
		Str("event", "provider.resolved").
		Str("request_id", requestID).
		Str("provider_name", providerName).
		Str("upstream_url", upstreamURL).
		Str("handler_type", handlerType).
		Str("model", model)

	// Add auth context fields if available (Bug #2 fix: model, key_hash, user_id, team_id)
	if userID, ok := ctx.Value(ContextKeyUserID).(string); ok && userID != "" {
		evt = evt.Str("user_id", userID)
	}
	if teamID, ok := ctx.Value(ContextKeyTeamID).(string); ok && teamID != "" {
		evt = evt.Str("team_id", teamID)
	}
	if tokenHash, ok := ctx.Value(ContextKeyTokenHash).(string); ok && tokenHash != "" {
		evt = evt.Str("key_hash", tokenHash)
	}

	evt.Msg("")
}

// UpstreamResult holds the data for an "upstream.responded" log event.
type UpstreamResult struct {
	StatusCode int
	LatencyMs  float64
	TokenUsage *TokenUsage
	Error      string
}

// TokenUsage captures token counts for upstream response logging.
type TokenUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// LogUpstreamResponded emits an "upstream.responded" lifecycle log event.
// Call this in each handler after receiving the upstream HTTP response.
func LogUpstreamResponded(ctx context.Context, result UpstreamResult) {
	requestID := chiMiddleware.GetReqID(ctx)

	evt := log.Info().
		Str("event", "upstream.responded").
		Str("request_id", requestID).
		Int("status_code", result.StatusCode).
		Float64("latency_ms", result.LatencyMs)

	if result.TokenUsage != nil {
		evt = evt.Interface("token_usage", result.TokenUsage)
	}

	if result.Error != "" {
		evt = evt.Str("error", result.Error)
	}

	evt.Msg("")
}

// NewUpstreamTimer returns the current time for measuring upstream latency.
func NewUpstreamTimer() time.Time {
	return time.Now()
}

// UpstreamLatencyMs returns the elapsed milliseconds since the given start time.
func UpstreamLatencyMs(start time.Time) float64 {
	return float64(time.Since(start).Milliseconds())
}
