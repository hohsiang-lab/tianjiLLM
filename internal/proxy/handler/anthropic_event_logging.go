package handler

import (
	"io"
	"net/http"
)

// AnthropicEventLoggingBatch handles POST /api/event_logging/batch.
// Stub endpoint that accepts Claude Code CLI telemetry and returns OK
// to prevent 404 errors in client logs. Unauthenticated by design —
// telemetry should not require an API key.
func (h *Handlers) AnthropicEventLoggingBatch(w http.ResponseWriter, r *http.Request) {
	// Limit and drain body to prevent abuse on this unauthenticated endpoint.
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MB
	_, _ = io.Copy(io.Discard, r.Body)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
