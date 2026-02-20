package handler

import (
	"net/http"
)

// GetResponse handles GET /v1/responses/{response_id}.
func (h *Handlers) GetResponse(w http.ResponseWriter, r *http.Request) {
	h.assistantsProxy(w, r)
}

// CancelResponse handles POST /v1/responses/{response_id}/cancel.
func (h *Handlers) CancelResponse(w http.ResponseWriter, r *http.Request) {
	h.assistantsProxy(w, r)
}

// ListResponseInputItems handles GET /v1/responses/{response_id}/input_items.
func (h *Handlers) ListResponseInputItems(w http.ResponseWriter, r *http.Request) {
	h.assistantsProxy(w, r)
}
