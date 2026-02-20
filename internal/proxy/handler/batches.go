package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/praxisllmlab/tianjiLLM/internal/model"
)

// BatchesCreate handles POST /v1/batches — create a batch.
func (h *Handlers) BatchesCreate(w http.ResponseWriter, r *http.Request) {
	baseURL, apiKey, err := h.resolveProviderBaseURL("")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: err.Error(), Type: "invalid_request_error"},
		})
		return
	}
	h.forwardToProvider(w, r, baseURL+"/batches", apiKey, "application/json")
}

// BatchesGet handles GET /v1/batches/{batch_id} — get batch status.
func (h *Handlers) BatchesGet(w http.ResponseWriter, r *http.Request) {
	batchID := chi.URLParam(r, "batch_id")
	baseURL, apiKey, err := h.resolveProviderBaseURL("")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: err.Error(), Type: "invalid_request_error"},
		})
		return
	}
	h.forwardToProvider(w, r, baseURL+"/batches/"+batchID, apiKey, "")
}

// BatchesCancel handles POST /v1/batches/{batch_id}/cancel — cancel a batch.
func (h *Handlers) BatchesCancel(w http.ResponseWriter, r *http.Request) {
	batchID := chi.URLParam(r, "batch_id")
	baseURL, apiKey, err := h.resolveProviderBaseURL("")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: err.Error(), Type: "invalid_request_error"},
		})
		return
	}
	h.forwardToProvider(w, r, baseURL+"/batches/"+batchID+"/cancel", apiKey, "application/json")
}

// BatchesList handles GET /v1/batches — list batches.
func (h *Handlers) BatchesList(w http.ResponseWriter, r *http.Request) {
	baseURL, apiKey, err := h.resolveProviderBaseURL("")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: err.Error(), Type: "invalid_request_error"},
		})
		return
	}
	// Forward query params (limit, after)
	url := baseURL + "/batches"
	if r.URL.RawQuery != "" {
		url += "?" + r.URL.RawQuery
	}
	h.forwardToProvider(w, r, url, apiKey, "")
}
