package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/praxisllmlab/tianjiLLM/internal/model"
	"github.com/praxisllmlab/tianjiLLM/internal/provider"
)

// Embedding handles POST /v1/embeddings.
func (h *Handlers) Embedding(w http.ResponseWriter, r *http.Request) {
	var req model.EmbeddingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{
				Message: "invalid request body: " + err.Error(),
				Type:    "invalid_request_error",
			},
		})
		return
	}

	p, apiKey, modelName, err := h.resolveProviderFromConfig(req.Model)
	if err != nil {
		status := http.StatusBadRequest
		if strings.Contains(err.Error(), "not found") {
			status = http.StatusNotFound
		}
		writeJSON(w, status, model.ErrorResponse{
			Error: model.ErrorDetail{
				Message: err.Error(),
				Type:    "invalid_request_error",
			},
		})
		return
	}

	embProvider, ok := p.(provider.EmbeddingProvider)
	if !ok {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{
				Message: "provider does not support embeddings",
				Type:    "invalid_request_error",
			},
		})
		return
	}

	req.Model = modelName

	httpReq, err := embProvider.TransformEmbeddingRequest(r.Context(), &req, apiKey)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{
				Message: "transform request: " + err.Error(),
				Type:    "internal_error",
			},
		})
		return
	}

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, model.ErrorResponse{
			Error: model.ErrorDetail{
				Message: "upstream request failed: " + err.Error(),
				Type:    "internal_error",
			},
		})
		return
	}

	result, err := embProvider.TransformEmbeddingResponse(r.Context(), resp)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, model.ErrorResponse{
			Error: model.ErrorDetail{
				Message: "transform response: " + err.Error(),
				Type:    "internal_error",
			},
		})
		return
	}

	writeJSON(w, http.StatusOK, result)
}
