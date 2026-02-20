package handler

import (
	"encoding/json"
	"net/http"

	"github.com/praxisllmlab/tianjiLLM/internal/model"
)

// ImageGeneration handles POST /v1/images/generations.
func (h *Handlers) ImageGeneration(w http.ResponseWriter, r *http.Request) {
	var req model.ImageGenerationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{
				Message: "invalid request body: " + err.Error(),
				Type:    "invalid_request_error",
			},
		})
		return
	}

	p, apiKey, _, err := h.resolveProviderFromConfig(req.Model)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{
				Message: err.Error(),
				Type:    "invalid_request_error",
			},
		})
		return
	}

	url := p.GetRequestURL(req.Model)
	url = url[:len(url)-len("/chat/completions")] + "/images/generations"

	proxyUpstream(w, r, url, apiKey, p)
}
