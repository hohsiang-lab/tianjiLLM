package handler

import (
	"net/http"
	"strings"

	"github.com/praxisllmlab/tianjiLLM/internal/model"
)

// proxyPassthrough resolves the provider from the request model or config,
// then forwards the full request to the upstream endpoint at the given path suffix.
func (h *Handlers) proxyPassthrough(w http.ResponseWriter, r *http.Request, pathSuffix string) {
	modelName := r.URL.Query().Get("model")
	baseURL, apiKey, err := h.resolveProviderBaseURL(modelName)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "no provider found: " + err.Error(), Type: "invalid_request_error"},
		})
		return
	}

	upstreamURL := strings.TrimRight(baseURL, "/") + "/" + pathSuffix
	if r.URL.RawQuery != "" {
		upstreamURL += "?" + r.URL.RawQuery
	}

	h.forwardToProvider(w, r, upstreamURL, apiKey, "")
}
