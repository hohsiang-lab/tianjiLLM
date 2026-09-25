package handler

import (
	"fmt"
	"net/http"
	"time"

	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/praxisllmlab/tianjiLLM/internal/model"
)

// CompactResponse handles OpenAI-compatible /v1/responses/compact.
func (h *Handlers) CompactResponse(w http.ResponseWriter, r *http.Request) {
	route, err := h.resolveOpenAIEndpointRouteForCompactResponses(r)
	if err != nil {
		writeOpenAIEndpointRouteError(w, err)
		return
	}
	if route.Transport == openAISubscriptionTransportChatGPTCodexBackend {
		h.handleChatGPTCodexCompactResponse(w, r, route.TianjiParams, route.Candidates)
		return
	}
	if route.ModelName != "" {
		if err := rewriteResponsesRequestModel(r, route.ModelName); err != nil {
			writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
				Error: model.ErrorDetail{Message: err.Error(), Type: "invalid_request_error"},
			})
			return
		}
	}
	if len(route.Candidates) > 0 {
		h.proxyOpenAIUpstreamWithSubscriptionCandidates(w, r, route.Upstream, route.Candidates)
		return
	}
	h.proxyOpenAIUpstream(w, r, route.Upstream, route.APIKey, false)
}

func (h *Handlers) handleChatGPTCodexCompactResponse(w http.ResponseWriter, r *http.Request, params config.TianjiParams, candidates []resolvedOpenAISubscriptionCredential) {
	startTime := time.Now()
	if len(candidates) == 0 {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "OpenAI subscription credential resolution failed: no usable credential candidates", Type: "invalid_request_error"},
		})
		return
	}

	payload, envelope, err := decodeResponsesRequestBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: err.Error(), Type: "invalid_request_error"},
		})
		return
	}
	reasoningEffort := reasoningEffortFromPayload(payload)

	transport := h.chatGPTCodexBackendTransportForParams(r.Context(), params, envelope.Model, candidates)
	preparedPayload, err := transport.PrepareCompactResponsesPayload(payload)
	if err != nil {
		if _, ok := chatGPTCodexBackendClientValidationError(err); ok {
			writeChatGPTCodexBackendPreparationError(w, err)
			return
		}
		h.logResponsesFailure(r.Context(), envelope.Model, reasoningEffort, startTime, fmt.Errorf("upstream request failed: %w", err))
		writeChatGPTCodexBackendPreparationError(w, err)
		return
	}
	upstreamPayload := preparedPayload.Snapshot()

	candidates = h.orderOpenAISubscriptionCodexCandidatesWithIdentity(r.Context(), params, candidates, extractCodexSessionIdentity(payload))
	defer h.refreshOpenAISubscriptionCodexUsageAfterResponse(r.Context(), candidates)
	resp, llmLatency, subscriptionAttribution, err := h.doOpenAISubscriptionProviderRequest(r.Context(), "", candidates, func(attempt openAISubscriptionProviderAttempt) (*http.Request, error) {
		return transport.BuildCompactRequest(r.Context(), preparedPayload, attempt.apiKey, attempt.accountID, r.Header)
	})
	if err != nil {
		logCodexResponsesRequestError(r.Context(), envelope.Model, false, upstreamPayload, startTime, err)
		h.logResponsesFailure(r.Context(), envelope.Model, reasoningEffort, startTime, fmt.Errorf("upstream request failed: %w", err))
		writeJSON(w, http.StatusBadGateway, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "upstream request failed: " + err.Error(), Type: "internal_error"},
		})
		return
	}
	defer resp.Body.Close()

	body := copyHTTPResponse(w, resp)
	endTime := time.Now()
	logDebugCodexResponsesBody(r.Context(), envelope.Model, resp.StatusCode, body)
	if resp.StatusCode >= http.StatusBadRequest {
		logCodexResponsesRequestShape(r.Context(), envelope.Model, false, upstreamPayload)
		h.logResponsesUpstreamHTTPFailure(r.Context(), envelope.Model, reasoningEffort, startTime, resp.StatusCode, body)
		return
	}

	h.logResponsesSuccess(
		r.Context(),
		envelope.Model,
		reasoningEffort,
		extractResponsesUsage(body),
		startTime,
		endTime,
		llmLatency,
		subscriptionAttribution,
		responsesRequestAuditPayload(payload, upstreamPayload),
		responsesResponseAuditPayload(body),
	)
}
