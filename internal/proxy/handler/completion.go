package handler

import (
	"encoding/json"
	"net/http"

	"github.com/praxisllmlab/tianjiLLM/internal/model"
	"github.com/praxisllmlab/tianjiLLM/internal/provider"
)

// Completion handles POST /v1/completions (legacy text completion).
// It proxies the request as-is to the upstream provider.
func (h *Handlers) Completion(w http.ResponseWriter, r *http.Request) {
	var req model.CompletionRequest
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

	// Build the upstream URL for /completions
	url := p.GetRequestURL(req.Model)
	// Replace /chat/completions with /completions for legacy endpoint
	url = url[:len(url)-len("/chat/completions")] + "/completions"

	proxyUpstream(w, r, url, apiKey, p)
}

// proxyUpstream forwards the request body to the upstream URL and pipes the response back.
func proxyUpstream(w http.ResponseWriter, r *http.Request, url, apiKey string, p provider.Provider) {
	httpReq, err := http.NewRequestWithContext(r.Context(), http.MethodPost, url, r.Body)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{
				Message: "create upstream request: " + err.Error(),
				Type:    "internal_error",
			},
		})
		return
	}

	p.SetupHeaders(httpReq, apiKey)
	httpReq.Header.Set("Content-Type", r.Header.Get("Content-Type"))

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
	defer resp.Body.Close()

	// Copy upstream response headers and body
	for k, vv := range resp.Header {
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	_, _ = w.Write(mustReadAll(resp.Body))
}
