package handler

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/praxisllmlab/tianjiLLM/internal/model"
	"github.com/praxisllmlab/tianjiLLM/internal/proxy/middleware"
)

// Moderation handles POST /v1/moderations.
func (h *Handlers) Moderation(w http.ResponseWriter, r *http.Request) {
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
	var req model.ModerationRequest
	if err = json.Unmarshal(body, &req); err != nil {
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

	route, err := h.resolveProviderFromConfigRouteWithContext(r.Context(), modelName)
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
		writeUnsupportedChatGPTCodexEndpoint(w, "moderation")
		return
	}
	p, apiKey := route.Provider, route.APIKey

	url := p.GetRequestURL(modelName)
	url = url[:len(url)-len("/chat/completions")] + "/moderations"

	middleware.LogProviderResolved(r.Context(), h.lookupProviderName(modelName), url, "moderation", modelName)

	httpReq, err := http.NewRequestWithContext(r.Context(), http.MethodPost, url, bytes.NewReader(body))
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
	httpReq.Header.Set("Content-Type", "application/json")

	upstreamStart := time.Now()
	resp, err := http.DefaultClient.Do(httpReq)
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

	middleware.LogUpstreamResponded(r.Context(), middleware.UpstreamResult{
		StatusCode: resp.StatusCode,
		LatencyMs:  upstreamLatency,
	})

	respBody := copyResponseBody(w, resp)
	if respBody == nil {
		return
	}

	if resp.StatusCode >= 200 && resp.StatusCode < 300 && h.Callbacks != nil {
		endTime := time.Now()
		data := buildBaseLogData(r.Context(), startTime)
		data.Model = modelName
		data.Provider = h.lookupProviderName(modelName)
		data.CallType = "moderation"
		data.EndTime = endTime
		data.Latency = endTime.Sub(startTime)
		go h.Callbacks.LogSuccess(data)
	}
}
