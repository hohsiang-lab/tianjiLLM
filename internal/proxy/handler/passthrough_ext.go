package handler

import "net/http"

// AnthropicBatchesCreate handles POST /anthropic/v1/messages/batches — pass-through.
func (h *Handlers) AnthropicBatchesCreate(w http.ResponseWriter, r *http.Request) {
	h.proxyPassthrough(w, r, "v1/messages/batches")
}

// AnthropicBatchesGet handles GET /anthropic/v1/messages/batches/{id} — pass-through.
func (h *Handlers) AnthropicBatchesGet(w http.ResponseWriter, r *http.Request) {
	h.proxyPassthrough(w, r, "v1/messages/batches")
}

// AnthropicBatchesResults handles GET /anthropic/v1/messages/batches/{id}/results — pass-through.
func (h *Handlers) AnthropicBatchesResults(w http.ResponseWriter, r *http.Request) {
	h.proxyPassthrough(w, r, "v1/messages/batches")
}
