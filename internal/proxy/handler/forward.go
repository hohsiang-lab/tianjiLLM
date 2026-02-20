package handler

import (
	"fmt"
	"io"
	"net/http"

	"github.com/praxisllmlab/tianjiLLM/internal/model"
)

// forwardToProvider proxies a request to the upstream provider, copying the
// request body and returning the upstream response verbatim. This is used by
// Files, Batches, Fine-tuning, and similar pass-through endpoints.
func (h *Handlers) forwardToProvider(w http.ResponseWriter, r *http.Request, upstreamURL, apiKey, contentType string) {
	upstreamReq, err := http.NewRequestWithContext(r.Context(), r.Method, upstreamURL, r.Body)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{
				Message: "create upstream request: " + err.Error(),
				Type:    "internal_error",
			},
		})
		return
	}

	if contentType != "" {
		upstreamReq.Header.Set("Content-Type", contentType)
	} else {
		upstreamReq.Header.Set("Content-Type", r.Header.Get("Content-Type"))
	}
	upstreamReq.Header.Set("Authorization", "Bearer "+apiKey)

	// Copy Content-Length for multipart uploads
	if r.ContentLength > 0 {
		upstreamReq.ContentLength = r.ContentLength
	}

	resp, err := http.DefaultClient.Do(upstreamReq)
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

	// Copy upstream response headers
	for k, vv := range resp.Header {
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}

// resolveProviderBaseURL finds the model config and returns the provider's base URL and API key.
// For APIs like Files/Batches, the model param may be empty, so we fall back to
// finding any OpenAI-compatible provider in the config.
func (h *Handlers) resolveProviderBaseURL(modelName string) (baseURL, apiKey string, err error) {
	if modelName != "" {
		cfg, _ := h.findModelConfig(modelName)
		if cfg != nil {
			apiKey := ""
			if cfg.TianjiParams.APIKey != nil {
				apiKey = *cfg.TianjiParams.APIKey
			}
			baseURL := "https://api.openai.com/v1"
			if cfg.TianjiParams.APIBase != nil && *cfg.TianjiParams.APIBase != "" {
				baseURL = *cfg.TianjiParams.APIBase
			}
			return baseURL, apiKey, nil
		}
	}

	// Fallback: find first OpenAI model in config
	for _, m := range h.Config.ModelList {
		if m.TianjiParams.Model == "" {
			continue
		}
		provName, _ := parseProviderFromModel(m.TianjiParams.Model)
		if provName == "openai" || provName == "" {
			apiKey := ""
			if m.TianjiParams.APIKey != nil {
				apiKey = *m.TianjiParams.APIKey
			}
			baseURL := "https://api.openai.com/v1"
			if m.TianjiParams.APIBase != nil && *m.TianjiParams.APIBase != "" {
				baseURL = *m.TianjiParams.APIBase
			}
			return baseURL, apiKey, nil
		}
	}

	return "", "", fmt.Errorf("no OpenAI-compatible provider configured")
}

// parseProviderFromModel splits "provider/model" and returns provider name.
func parseProviderFromModel(model string) (string, string) {
	for i := range model {
		if model[i] == '/' {
			return model[:i], model[i+1:]
		}
	}
	return "", model
}
