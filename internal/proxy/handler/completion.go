package handler

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/praxisllmlab/tianjiLLM/internal/callback"
	"github.com/praxisllmlab/tianjiLLM/internal/model"
	"github.com/praxisllmlab/tianjiLLM/internal/provider"
	"github.com/praxisllmlab/tianjiLLM/internal/proxy/middleware"
)

// Completion handles POST /v1/completions (legacy text completion).
// It proxies the request to the upstream provider and records spend.
func (h *Handlers) Completion(w http.ResponseWriter, r *http.Request) {
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

	var req model.CompletionRequest
	if err = json.Unmarshal(body, &req); err != nil {
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

	// Phase 2: provider.resolved
	middleware.LogProviderResolved(r.Context(), h.lookupProviderName(req.Model), url, "completion", req.Model)
	// Replace /chat/completions with /completions for legacy endpoint
	url = url[:len(url)-len("/chat/completions")] + "/completions"

	upstreamStart := time.Now()
	resp, err := doUpstreamWithRetry(r.Context(), http.DefaultClient, func() (*http.Request, error) {
		req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, url, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		p.SetupHeaders(req, apiKey)
		req.Header.Set("Content-Type", r.Header.Get("Content-Type"))
		return req, nil
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
	defer resp.Body.Close()

	// Phase 3: upstream.responded
	middleware.LogUpstreamResponded(r.Context(), middleware.UpstreamResult{
		StatusCode: resp.StatusCode,
		LatencyMs:  upstreamLatency,
	})

	respBody := mustReadAll(resp.Body)

	for k, vv := range resp.Header {
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	_, _ = w.Write(respBody)

	if resp.StatusCode >= 200 && resp.StatusCode < 300 && h.Callbacks != nil {
		var parsed struct {
			Usage *struct {
				PromptTokens     int `json:"prompt_tokens"`
				CompletionTokens int `json:"completion_tokens"`
				TotalTokens      int `json:"total_tokens"`
			} `json:"usage"`
		}
		_ = json.Unmarshal(respBody, &parsed)

		promptTokens, completionTokens, totalTokens := 0, 0, 0
		if parsed.Usage != nil {
			promptTokens = parsed.Usage.PromptTokens
			completionTokens = parsed.Usage.CompletionTokens
			totalTokens = parsed.Usage.TotalTokens
		}

		endTime := time.Now()
		go h.Callbacks.LogSuccess(callback.LogData{
			Model:            req.Model,
			APIKey:           apiKey,
			PromptTokens:     promptTokens,
			CompletionTokens: completionTokens,
			TotalTokens:      totalTokens,
			StartTime:        startTime,
			EndTime:          endTime,
			Latency:          endTime.Sub(startTime),
			CallType:         "completion",
		})
	}
}

// proxyUpstream forwards the request body to the upstream URL and pipes the response back.
func proxyUpstream(w http.ResponseWriter, r *http.Request, url, apiKey string, p provider.Provider, maxRetries int) {
	// Buffer body so it can be re-read on each retry attempt.
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{
				Message: "read request body: " + err.Error(),
				Type:    "internal_error",
			},
		})
		return
	}
	contentType := r.Header.Get("Content-Type")

	upstreamStart2 := time.Now()
	resp, err := doUpstreamWithRetry(r.Context(), http.DefaultClient, func() (*http.Request, error) {
		req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, url, bytes.NewReader(bodyBytes))
		if err != nil {
			return nil, err
		}
		p.SetupHeaders(req, apiKey)
		req.Header.Set("Content-Type", contentType)
		return req, nil
	}, maxRetries)
	upstreamLatency2 := middleware.UpstreamLatencyMs(upstreamStart2)
	if err != nil {
		middleware.LogUpstreamResponded(r.Context(), middleware.UpstreamResult{
			LatencyMs: upstreamLatency2,
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
		LatencyMs:  upstreamLatency2,
	})

	// Copy upstream response headers and body
	for k, vv := range resp.Header {
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	_, _ = w.Write(mustReadAll(resp.Body))
}
