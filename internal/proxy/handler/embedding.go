package handler

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/praxisllmlab/tianjiLLM/internal/callback"
	"github.com/praxisllmlab/tianjiLLM/internal/model"
	"github.com/praxisllmlab/tianjiLLM/internal/provider"
	"github.com/praxisllmlab/tianjiLLM/internal/proxy/middleware"
)

// Embedding handles POST /v1/embeddings.
func (h *Handlers) Embedding(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()

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

	// Phase 2: provider.resolved
	middleware.LogProviderResolved(r.Context(), h.lookupProviderName(req.Model), p.GetRequestURL(modelName), "embedding", modelName)

	req.Model = modelName

	_embProvider, _embReq, _embApiKey := embProvider, &req, apiKey
	upstreamStart := time.Now()
	resp, err := doUpstreamWithRetry(r.Context(), http.DefaultClient, func() (*http.Request, error) {
		return _embProvider.TransformEmbeddingRequest(r.Context(), _embReq, _embApiKey)
	}, h.MaxUpstreamRetries)
	upstreamLatency := middleware.UpstreamLatencyMs(upstreamStart)
	if err != nil {
		middleware.LogUpstreamResponded(r.Context(), middleware.UpstreamResult{
			LatencyMs: upstreamLatency,
			Error:     err.Error(),
		})
		writeJSON(w, http.StatusBadGateway, model.ErrorResponse{
			Error: model.ErrorDetail{
				Message: "upstream request failed: " + err.Error(),
				Type:    "internal_error",
			},
		})
		return
	}

	// Phase 3: upstream.responded
	middleware.LogUpstreamResponded(r.Context(), middleware.UpstreamResult{
		StatusCode: resp.StatusCode,
		LatencyMs:  upstreamLatency,
	})

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

	endTime := time.Now()
	if h.Callbacks != nil {
		go h.Callbacks.LogSuccess(callback.LogData{
			Model:        modelName,
			APIKey:       apiKey,
			PromptTokens: result.Usage.PromptTokens,
			TotalTokens:  result.Usage.TotalTokens,
			StartTime:    startTime,
			EndTime:      endTime,
			Latency:      endTime.Sub(startTime),
			CallType:     "embedding",
		})
	}

	writeJSON(w, http.StatusOK, result)
}
