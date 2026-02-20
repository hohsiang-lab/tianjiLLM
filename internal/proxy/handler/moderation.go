package handler

import (
	"net/http"

	"github.com/praxisllmlab/tianjiLLM/internal/model"
)

// Moderation handles POST /v1/moderations.
func (h *Handlers) Moderation(w http.ResponseWriter, r *http.Request) {
	var req model.ModerationRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{
				Message: "invalid request body: " + err.Error(),
				Type:    "invalid_request_error",
			},
		})
		return
	}

	modelName := req.Model
	if modelName == "" {
		modelName = "text-moderation-latest"
	}

	p, apiKey, _, err := h.resolveProviderFromConfig(modelName)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{
				Message: err.Error(),
				Type:    "invalid_request_error",
			},
		})
		return
	}

	url := p.GetRequestURL(modelName)
	url = url[:len(url)-len("/chat/completions")] + "/moderations"

	proxyUpstream(w, r, url, apiKey, p)
}
