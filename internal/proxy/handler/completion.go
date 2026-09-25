package handler

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/praxisllmlab/tianjiLLM/internal/model"
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

	if h.requestUsesOfficialOpenAIModel(req.Model) {
		writeUnsupportedChatGPTCodexEndpoint(w, "legacy completions")
		return
	}
	route, err := h.resolveProviderFromConfigRouteWithContext(r.Context(), req.Model)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{
				Message: err.Error(),
				Type:    "invalid_request_error",
			},
		})
		return
	}
	if route.ChatGPTCodexBackend {
		writeUnsupportedChatGPTCodexEndpoint(w, "legacy completions")
		return
	}

	// Build the upstream URL for /completions
	url := route.Provider.GetRequestURL(req.Model)

	// Phase 2: provider.resolved
	middleware.LogProviderResolved(r.Context(), h.lookupProviderName(req.Model), url, "completion", req.Model)
	// Replace /chat/completions with /completions for legacy endpoint
	url = url[:len(url)-len("/chat/completions")] + "/completions"

	resp, upstreamLatencyDuration, subscriptionAttribution, err := h.doOpenAISubscriptionProviderRequest(r.Context(), route.APIKey, route.SubscriptionCandidates, func(attempt openAISubscriptionProviderAttempt) (*http.Request, error) {
		httpReq, buildErr := http.NewRequestWithContext(r.Context(), http.MethodPost, url, bytes.NewReader(body))
		if buildErr != nil {
			return nil, buildErr
		}
		route.Provider.SetupHeaders(httpReq, attempt.apiKey)
		httpReq.Header.Set("Content-Type", r.Header.Get("Content-Type"))
		return httpReq, nil
	})
	upstreamLatency := float64(upstreamLatencyDuration.Milliseconds())
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

	respBody := copyResponseBody(w, resp)
	if respBody == nil {
		return
	}

	if resp.StatusCode >= 200 && resp.StatusCode < 300 && h.Callbacks != nil {
		var parsed struct {
			Usage *struct {
				PromptTokens     int `json:"prompt_tokens"`
				CompletionTokens int `json:"completion_tokens"`
				TotalTokens      int `json:"total_tokens"`
			} `json:"usage"`
		}
		if err := json.Unmarshal(respBody, &parsed); err != nil {
			log.Printf("warn: failed to parse completion response for usage: %v", err)
		}

		promptTokens, completionTokens, totalTokens := 0, 0, 0
		if parsed.Usage != nil {
			promptTokens = parsed.Usage.PromptTokens
			completionTokens = parsed.Usage.CompletionTokens
			totalTokens = parsed.Usage.TotalTokens
		}

		endTime := time.Now()
		data := buildBaseLogData(r.Context(), startTime)
		data.Model = req.Model
		data.Provider = h.lookupProviderName(req.Model)
		data.CallType = "completion"
		data.EndTime = endTime
		data.Latency = endTime.Sub(startTime)
		applyOpenAISubscriptionLogAttribution(&data, openAISubscriptionAttributionForAction(subscriptionAttribution, "completion"))
		data.PromptTokens = promptTokens
		data.CompletionTokens = completionTokens
		data.TotalTokens = totalTokens
		go h.Callbacks.LogSuccess(data)
	}
}
