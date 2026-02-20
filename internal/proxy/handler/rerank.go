package handler

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"

	"github.com/praxisllmlab/tianjiLLM/internal/model"
)

// Rerank handles POST /v1/rerank — rerank documents.
func (h *Handlers) Rerank(w http.ResponseWriter, r *http.Request) {
	// Read body to extract model field for provider resolution
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{
				Message: "read request body: " + err.Error(),
				Type:    "invalid_request_error",
			},
		})
		return
	}

	var req model.RerankRequest
	if err = json.Unmarshal(body, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{
				Message: "invalid request body: " + err.Error(),
				Type:    "invalid_request_error",
			},
		})
		return
	}

	baseURL, apiKey, err := h.resolveProviderBaseURL(req.Model)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: err.Error(), Type: "invalid_request_error"},
		})
		return
	}

	// Replace request body for forwarding
	r.Body = io.NopCloser(bytes.NewReader(body))
	h.forwardToProvider(w, r, baseURL+"/rerank", apiKey, "application/json")
}
