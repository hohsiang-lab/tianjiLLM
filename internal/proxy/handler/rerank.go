package handler

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/praxisllmlab/tianjiLLM/internal/callback"
	"github.com/praxisllmlab/tianjiLLM/internal/model"
	"github.com/praxisllmlab/tianjiLLM/internal/proxy/middleware"
)

// Rerank handles POST /v1/rerank — rerank documents.
func (h *Handlers) Rerank(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()

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

	// Phase 2: provider.resolved
	middleware.LogProviderResolved(r.Context(), h.lookupProviderName(req.Model), baseURL+"/rerank", "rerank", req.Model)

	upstreamReq, err := http.NewRequestWithContext(r.Context(), http.MethodPost, baseURL+"/rerank", bytes.NewReader(body))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{
				Message: "create upstream request: " + err.Error(),
				Type:    "internal_error",
			},
		})
		return
	}
	upstreamReq.Header.Set("Content-Type", "application/json")
	upstreamReq.Header.Set("Authorization", "Bearer "+apiKey)

	upstreamStart := time.Now()
	resp, err := http.DefaultClient.Do(upstreamReq)
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
	defer resp.Body.Close()

	// Phase 3: upstream.responded
	middleware.LogUpstreamResponded(r.Context(), middleware.UpstreamResult{
		StatusCode: resp.StatusCode,
		LatencyMs:  upstreamLatency,
	})

	respBody, _ := io.ReadAll(resp.Body)

	for k, vv := range resp.Header {
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	_, _ = w.Write(respBody)

	if resp.StatusCode >= 200 && resp.StatusCode < 300 && h.Callbacks != nil {
		var parsed model.RerankResponse
		_ = json.Unmarshal(respBody, &parsed)

		totalTokens := 0
		if parsed.Usage != nil {
			totalTokens = parsed.Usage.TotalTokens
		} else if parsed.Meta != nil && parsed.Meta.Tokens != nil {
			// Cohere-style response: meta.tokens.input_tokens
			totalTokens = parsed.Meta.Tokens.InputTokens + parsed.Meta.Tokens.OutputTokens
		}

		endTime := time.Now()
		// For rerank, there is no prompt/completion token split.
		// Set PromptTokens = TotalTokens so the UI can display token usage.
		go h.Callbacks.LogSuccess(callback.LogData{
			Model:        req.Model,
			APIKey:       apiKey,
			PromptTokens: totalTokens,
			TotalTokens:  totalTokens,
			StartTime:    startTime,
			EndTime:      endTime,
			Latency:      endTime.Sub(startTime),
			CallType:     "rerank",
		})
	}
}
