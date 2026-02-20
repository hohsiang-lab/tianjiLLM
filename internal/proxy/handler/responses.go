package handler

import (
	"net/http"
)

// CreateResponse handles POST /v1/responses.
// Proxies to upstream OpenAI Responses API via the same mechanism as Assistants API.
func (h *Handlers) CreateResponse(w http.ResponseWriter, r *http.Request) {
	h.assistantsProxy(w, r)
}
