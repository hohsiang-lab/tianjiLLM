package middleware

import (
	"net/http"
	"strings"
	"time"

	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog"
)

// StructuredLogging returns a chi-compatible middleware that logs
// request lifecycle events using zerolog structured JSON.
// It also injects a per-request zerolog.Logger (with request_id) into
// the request context so downstream handlers can retrieve it via
// zerolog.Ctx(r.Context()).
func StructuredLogging(logger zerolog.Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			reqID := chiMiddleware.GetReqID(r.Context())
			path := r.URL.Path

			// Build a sub-logger carrying request_id and inject into context.
			reqLogger := logger.With().Str("request_id", reqID).Logger()
			r = r.WithContext(reqLogger.WithContext(r.Context()))

			// Phase 1: request.received
			reqLogger.Info().
				Str("event", "request.received").
				Str("method", r.Method).
				Str("path", path).
				Msg("")

			ww := chiMiddleware.NewWrapResponseWriter(w, r.ProtoMajor)

			next.ServeHTTP(ww, r)

			latency := time.Since(start)

			// Phase 4: request.completed
			reqLogger.Info().
				Str("event", "request.completed").
				Str("method", r.Method).
				Str("path", path).
				Int("status", ww.Status()).
				Float64("latency_ms", float64(latency.Milliseconds())).
				Str("handler_type", resolveHandlerType(path)).
				Msg("")
		})
	}
}

// resolveHandlerType determines the handler type from the request path.
func resolveHandlerType(path string) string {
	switch {
	case strings.HasPrefix(path, "/v1/chat/"):
		return "chat"
	case strings.HasPrefix(path, "/v1/"):
		return "native"
	default:
		return "passthrough"
	}
}
