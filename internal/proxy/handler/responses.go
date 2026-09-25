package handler

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/coder/websocket"
	"github.com/praxisllmlab/tianjiLLM/internal/callback"
	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/praxisllmlab/tianjiLLM/internal/model"
	"github.com/praxisllmlab/tianjiLLM/internal/pricing"
	"github.com/praxisllmlab/tianjiLLM/internal/security/redact"
	"github.com/rs/zerolog"
)

// Codex sends full instructions and tool schemas in a single Responses WebSocket
// frame; keep this bounded while raising it above coder/websocket's 32 KiB default.
const responsesWebSocketReadLimit = 4 << 20

const responsesUpstreamErrorBodyLogLimit = 4096

type responsesRequestEnvelope struct {
	Model  string `json:"model"`
	Stream *bool  `json:"stream,omitempty"`
}

type responsesCreateFrame struct {
	Type string `json:"type"`
	responsesRequestEnvelope
}

// CreateResponse handles OpenAI-compatible /v1/responses.
func (h *Handlers) CreateResponse(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		h.handleResponsesWebSocket(w, r)
		return
	}

	route, err := h.resolveOpenAIEndpointRouteForResponses(r)
	if err != nil {
		writeOpenAIEndpointRouteError(w, err)
		return
	}
	var publicModel string
	var receivedPayload map[string]any
	if route.Transport == openAISubscriptionTransportChatGPTCodexBackend {
		var receivedEnvelope responsesRequestEnvelope
		var err error
		receivedPayload, receivedEnvelope, err = decodeResponsesRequestBody(r)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
				Error: model.ErrorDetail{Message: err.Error(), Type: "invalid_request_error"},
			})
			return
		}
		publicModel = receivedEnvelope.Model
	}
	if route.ModelName != "" {
		if err := rewriteResponsesRequestModel(r, route.ModelName); err != nil {
			writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
				Error: model.ErrorDetail{Message: err.Error(), Type: "invalid_request_error"},
			})
			return
		}
	}
	if route.Transport == openAISubscriptionTransportChatGPTCodexBackend {
		h.handleChatGPTCodexResponse(w, r, route.TianjiParams, route.Candidates, publicModel, receivedPayload)
		return
	}
	if len(route.Candidates) > 0 {
		h.proxyOpenAIUpstreamWithSubscriptionCandidates(w, r, route.Upstream, route.Candidates)
		return
	}
	h.proxyOpenAIUpstream(w, r, route.Upstream, route.APIKey, false)
}

func rewriteResponsesRequestModel(r *http.Request, modelName string) error {
	if r == nil || r.Body == nil || modelName == "" {
		return nil
	}

	body, err := readOpenAIRequestBody(r)
	if err != nil {
		return fmt.Errorf("read request body: %w", err)
	}
	originalBody := body

	var payload map[string]json.RawMessage
	if json.Unmarshal(body, &payload) != nil || payload == nil {
		r.Body = io.NopCloser(bytes.NewReader(body))
		return nil
	}
	modelJSON, err := json.Marshal(modelName)
	if err != nil {
		r.Body = io.NopCloser(bytes.NewReader(originalBody))
		return nil
	}
	if _, ok := payload["model"]; !ok {
		r.Body = io.NopCloser(bytes.NewReader(originalBody))
		return nil
	}
	payload["model"] = modelJSON
	body, err = json.Marshal(payload)
	if err != nil {
		r.Body = io.NopCloser(bytes.NewReader(originalBody))
		return nil
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	r.ContentLength = int64(len(body))
	r.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(body)), nil
	}
	return nil
}

func (h *Handlers) handleChatGPTCodexResponse(w http.ResponseWriter, r *http.Request, params config.TianjiParams, candidates []resolvedOpenAISubscriptionCredential, publicModel string, receivedPayload map[string]any) {
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
	logModel := publicModel
	if strings.TrimSpace(logModel) == "" {
		logModel = envelope.Model
	}

	transport := h.chatGPTCodexBackendTransportForParams(r.Context(), params, envelope.Model, candidates)
	stream := envelope.Stream != nil && *envelope.Stream
	preparedPayload, err := transport.PrepareResponsesPayload(payload, stream)
	if err != nil {
		if _, ok := chatGPTCodexBackendClientValidationError(err); ok {
			writeChatGPTCodexBackendPreparationError(w, err)
			return
		}
		h.logResponsesFailure(r.Context(), logModel, reasoningEffort, startTime, fmt.Errorf("upstream request failed: %w", err))
		writeChatGPTCodexBackendPreparationError(w, err)
		return
	}
	upstreamPayload := preparedPayload.Snapshot()
	candidates = h.orderOpenAISubscriptionCodexCandidatesWithIdentity(r.Context(), params, candidates, extractCodexSessionIdentity(payload))
	defer h.refreshOpenAISubscriptionCodexUsageAfterResponse(r.Context(), candidates)
	resp, llmLatency, subscriptionAttribution, err := h.doOpenAISubscriptionProviderRequest(r.Context(), "", candidates, func(attempt openAISubscriptionProviderAttempt) (*http.Request, error) {
		return transport.BuildPreparedResponsesRequest(r.Context(), preparedPayload, attempt.apiKey, attempt.accountID, r.Header)
	})
	if err != nil {
		logCodexResponsesRequestError(r.Context(), logModel, stream, upstreamPayload, startTime, err)
		h.logResponsesFailure(r.Context(), logModel, reasoningEffort, startTime, fmt.Errorf("upstream request failed: %w", err))
		writeJSON(w, http.StatusBadGateway, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "upstream request failed: " + err.Error(), Type: "internal_error"},
		})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		body := copyHTTPResponse(w, resp)
		logDebugCodexResponsesBody(r.Context(), logModel, resp.StatusCode, body)
		logCodexResponsesRequestShape(r.Context(), logModel, stream, upstreamPayload)
		h.logResponsesUpstreamHTTPFailure(r.Context(), logModel, reasoningEffort, startTime, resp.StatusCode, body)
		return
	}

	body, err := copyHTTPResponseWithError(w, resp)
	logDebugCodexResponsesBody(r.Context(), logModel, resp.StatusCode, body)
	if err != nil {
		h.logResponsesFailure(r.Context(), logModel, reasoningEffort, startTime, fmt.Errorf("copy upstream response: %w", err), subscriptionAttribution)
		return
	}
	if failure := logCodexResponsesFailureEvent(r.Context(), logModel, resp.StatusCode, body); failure != nil {
		h.logResponsesFailure(r.Context(), logModel, reasoningEffort, startTime, fmt.Errorf("upstream response failed: %s", failure.safeMessage()), subscriptionAttribution)
		return
	}
	h.logResponsesSuccess(
		r.Context(),
		logModel,
		reasoningEffort,
		extractResponsesUsage(body),
		startTime,
		time.Now(),
		llmLatency,
		subscriptionAttribution,
		responsesRequestAuditPayload(receivedPayload, upstreamPayload),
		responsesResponseAuditPayload(body),
	)
}

func (h *Handlers) handleResponsesWebSocket(w http.ResponseWriter, r *http.Request) {
	if !isWebSocketUpgrade(r) {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{
				Message: "GET /v1/responses requires a WebSocket upgrade request",
				Type:    "invalid_request_error",
			},
		})
		return
	}

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
	if err != nil {
		return
	}
	defer func() { _ = conn.CloseNow() }()
	conn.SetReadLimit(responsesWebSocketReadLimit)

	ctx := r.Context()
	clientHeaders := make(http.Header)
	for _, name := range []string{"Accept", "OpenAI-Beta", "Version", "originator", "x-codex-turn-state", "x-codex-turn-metadata", "x-codex-routing-hint"} {
		for _, value := range r.Header.Values(name) {
			clientHeaders.Add(name, value)
		}
	}
	state := &responsesWebSocketState{}
	for {
		_, frameData, err := conn.Read(ctx)
		if err != nil {
			return
		}
		if !h.handleResponsesWebSocketFrame(ctx, conn, frameData, state, clientHeaders) {
			return
		}
	}
}

func (h *Handlers) handleResponsesWebSocketFrame(ctx context.Context, conn *websocket.Conn, frameData []byte, state *responsesWebSocketState, clientHeaders http.Header) bool {
	payload, frame, err := decodeResponsesCreateFrame(frameData)
	if err != nil {
		_ = writeWebSocketError(conn, ctx, websocket.StatusInvalidFramePayloadData, err.Error())
		return false
	}
	if frame.Type != "response.create" {
		_ = writeWebSocketError(conn, ctx, websocket.StatusPolicyViolation, "WebSocket frame must be type=response.create")
		return false
	}
	if strings.TrimSpace(frame.Model) == "" {
		_ = writeWebSocketError(conn, ctx, websocket.StatusInvalidFramePayloadData, "response.create frame missing model")
		return false
	}

	route, err := h.resolveOpenAIEndpointRouteForModelStrict(ctx, frame.Model, false)
	if err != nil {
		var requestErr *model.RequestError
		if errors.As(err, &requestErr) {
			_ = writeWebSocketTypedError(conn, ctx, websocket.StatusPolicyViolation, requestErr.Detail)
		} else {
			_ = writeWebSocketError(conn, ctx, websocket.StatusPolicyViolation, err.Error())
		}
		return false
	}
	if route.Transport != openAISubscriptionTransportChatGPTCodexBackend || len(route.Candidates) == 0 {
		_ = writeWebSocketError(conn, ctx, websocket.StatusPolicyViolation, "Responses WebSocket requires chatgpt_codex_backend subscription transport")
		return false
	}
	zerolog.Ctx(ctx).Info().
		Str("event", "responses.websocket.route").
		Str("model", frame.Model).
		Str("transport", string(route.Transport)).
		Msg("responses websocket route selected")

	if isGenerateFalse(payload) {
		payload = cloneMap(payload)
		delete(payload, "generate")
		warmupID := state.storeSyntheticResponse(payload)
		if !writeSyntheticResponsesWebSocketEvent(ctx, conn, "response.created", warmupID, frame.Model) {
			return false
		}
		return writeSyntheticResponsesWebSocketEvent(ctx, conn, "response.completed", warmupID, frame.Model)
	}

	receivedPayload := cloneMap(payload)
	payload, err = state.expandPayload(payload)
	if err != nil {
		if errors.Is(err, errPreviousResponseStateNotFound) {
			_ = writeWebSocketTypedError(conn, ctx, websocket.StatusPolicyViolation, model.ErrorDetail{
				Message: err.Error(),
				Type:    "invalid_request_error",
				Param:   "previous_response_id",
				Code:    "previous_response_not_found",
			})
			return false
		}
		_ = writeWebSocketError(conn, ctx, websocket.StatusPolicyViolation, err.Error())
		return false
	}
	reasoningEffort := reasoningEffortFromPayload(payload)

	transport := h.chatGPTCodexBackendTransportForParams(ctx, route.TianjiParams, frame.Model, route.Candidates)
	startTime := time.Now()
	preparedPayload, err := transport.PrepareResponsesPayload(payload, true)
	if err != nil {
		if _, ok := chatGPTCodexBackendClientValidationError(err); ok {
			_ = writeWebSocketError(conn, ctx, websocket.StatusPolicyViolation, err.Error())
			return false
		}
		h.logResponsesFailure(ctx, frame.Model, reasoningEffort, startTime, fmt.Errorf("upstream request failed: %w", err))
		_ = writeWebSocketError(conn, ctx, websocket.StatusInternalError, "upstream request failed: "+err.Error())
		return false
	}
	upstreamPayload := preparedPayload.Snapshot()
	candidates := h.orderOpenAISubscriptionCodexCandidatesWithIdentity(ctx, route.TianjiParams, route.Candidates, extractCodexSessionIdentity(receivedPayload))
	defer h.refreshOpenAISubscriptionCodexUsageAfterResponse(ctx, candidates)
	resp, llmLatency, subscriptionAttribution, err := h.doOpenAISubscriptionProviderRequest(ctx, "", candidates, func(attempt openAISubscriptionProviderAttempt) (*http.Request, error) {
		return transport.BuildPreparedResponsesRequest(ctx, preparedPayload, attempt.apiKey, attempt.accountID, clientHeaders)
	})
	if err != nil {
		logCodexResponsesRequestError(ctx, frame.Model, true, upstreamPayload, startTime, err)
		h.logResponsesFailure(ctx, frame.Model, reasoningEffort, startTime, fmt.Errorf("upstream request failed: %w", err))
		_ = writeWebSocketError(conn, ctx, websocket.StatusInternalError, "upstream request failed: "+err.Error())
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		logCodexResponsesRequestShape(ctx, frame.Model, true, upstreamPayload)
		h.logResponsesUpstreamHTTPFailure(ctx, frame.Model, reasoningEffort, startTime, resp.StatusCode, body)
		if len(body) > 0 {
			if writeErr := conn.Write(ctx, websocket.MessageText, body); writeErr != nil {
				_ = conn.Close(websocket.StatusInternalError, writeErr.Error())
				return false
			}
		}
		_ = conn.Close(websocket.StatusPolicyViolation, http.StatusText(resp.StatusCode))
		return false
	}

	responseID, usage, responsePayload, completedResponse, completedOutputItems, ok := sendSSEAsWebSocketMessages(ctx, conn, resp.Body)
	if !ok {
		h.logResponsesFailure(ctx, frame.Model, reasoningEffort, startTime, fmt.Errorf("responses websocket stream failed"))
		return false
	}
	h.logResponsesSuccess(
		ctx,
		frame.Model,
		reasoningEffort,
		usage,
		startTime,
		time.Now(),
		llmLatency,
		subscriptionAttribution,
		responsesRequestAuditPayload(receivedPayload, upstreamPayload),
		responsePayload,
	)
	state.storeResponse(responseID, responseConversationPayload(payload, completedResponse, completedOutputItems))
	return true
}

func (h *Handlers) logResponsesSuccess(ctx context.Context, modelName, reasoningEffort string, usage model.Usage, startTime, endTime time.Time, llmLatency time.Duration, subscriptionAttribution *callback.OpenAISubscriptionAttribution, requestPayload, responsePayload any) {
	totalTokens := usage.TotalTokens
	if totalTokens == 0 {
		totalTokens = usage.PromptTokens + usage.CompletionTokens
	}
	regularInputTokens := usage.PromptTokens - usage.CacheReadInputTokens - usage.CacheCreationInputTokens
	if regularInputTokens < 0 {
		regularInputTokens = 0
	}
	cost := pricing.Default().TotalCost(modelName, pricing.TokenUsage{
		PromptTokens:             regularInputTokens,
		CompletionTokens:         usage.CompletionTokens,
		CacheReadInputTokens:     usage.CacheReadInputTokens,
		CacheCreationInputTokens: usage.CacheCreationInputTokens,
	})
	zerolog.Ctx(ctx).Info().
		Str("event", "responses.usage").
		Str("model", modelName).
		Int("prompt_tokens", usage.PromptTokens).
		Int("completion_tokens", usage.CompletionTokens).
		Int("total_tokens", totalTokens).
		Int("cache_read_input_tokens", usage.CacheReadInputTokens).
		Int("cache_creation_input_tokens", usage.CacheCreationInputTokens).
		Float64("cost", cost).
		Msg("responses usage extracted")

	if h.Callbacks == nil {
		return
	}

	data := buildBaseLogData(ctx, startTime)
	data.Model = modelName
	data.Provider = h.lookupProviderName(modelName)
	data.CallType = "responses"
	data.ReasoningEffort = reasoningEffort
	data.EndTime = endTime
	data.Latency = endTime.Sub(startTime)
	data.LLMAPILatency = llmLatency
	applyOpenAISubscriptionLogAttribution(&data, subscriptionAttribution)
	data.PromptTokens = usage.PromptTokens
	data.CompletionTokens = usage.CompletionTokens
	data.TotalTokens = totalTokens
	data.CacheReadInputTokens = usage.CacheReadInputTokens
	data.CacheCreationInputTokens = usage.CacheCreationInputTokens
	data.CacheHit = usage.CacheReadInputTokens > 0 || usage.CacheCreationInputTokens > 0
	data.Cost = cost
	data.RequestPayload = requestPayload
	data.ResponsePayload = responsePayload

	go h.Callbacks.LogSuccess(data)
}

func logDebugCodexResponsesBody(ctx context.Context, modelName string, statusCode int, body []byte) {
	if os.Getenv("TIANJI_DEBUG_CODEX_RESPONSE") != "1" {
		return
	}
	zerolog.Ctx(ctx).Info().
		Str("event", "responses.debug_body").
		Str("model", modelName).
		Int("status_code", statusCode).
		Int("body_bytes", len(body)).
		Str("body", string(body)).
		Msg("responses upstream body")
}

type codexResponsesFailureEvent struct {
	EventType    string
	ResponseID   string
	Status       string
	ErrorType    string
	ErrorCode    string
	ErrorMessage string
}

func (f *codexResponsesFailureEvent) safeMessage() string {
	if f == nil {
		return ""
	}
	parts := make([]string, 0, 4)
	if strings.TrimSpace(f.EventType) != "" {
		parts = append(parts, "event="+strings.TrimSpace(f.EventType))
	}
	if strings.TrimSpace(f.Status) != "" {
		parts = append(parts, "status="+strings.TrimSpace(f.Status))
	}
	if strings.TrimSpace(f.ErrorType) != "" {
		parts = append(parts, "type="+strings.TrimSpace(f.ErrorType))
	}
	if strings.TrimSpace(f.ErrorCode) != "" {
		parts = append(parts, "code="+strings.TrimSpace(f.ErrorCode))
	}
	if strings.TrimSpace(f.ErrorMessage) != "" {
		parts = append(parts, "message="+truncateResponseFailureMessage(f.ErrorMessage))
	}
	return redact.String(strings.Join(parts, " "))
}

func logCodexResponsesFailureEvent(ctx context.Context, modelName string, statusCode int, body []byte) *codexResponsesFailureEvent {
	failure := extractCodexResponsesFailureEvent(body)
	if failure == nil {
		return nil
	}
	if os.Getenv("TIANJI_DEBUG_CODEX_SSE") == "1" {
		zerolog.Ctx(ctx).Warn().
			Str("event", "responses.codex_sse_failed").
			Str("model", modelName).
			Int("status_code", statusCode).
			Str("upstream_event_type", failure.EventType).
			Str("response_id", failure.ResponseID).
			Str("response_status", failure.Status).
			Str("error_type", failure.ErrorType).
			Str("error_code", failure.ErrorCode).
			Str("error_message", redact.String(truncateResponseFailureMessage(failure.ErrorMessage))).
			Msg("responses Codex backend returned failed SSE event")
	}
	return failure
}

func extractCodexResponsesFailureEvent(body []byte) *codexResponsesFailureEvent {
	if len(bytes.TrimSpace(body)) == 0 {
		return nil
	}
	var jsonEvent map[string]any
	if err := json.Unmarshal(body, &jsonEvent); err == nil {
		return codexResponsesFailureEventFromMap(jsonEvent)
	}

	scanner := bufio.NewScanner(bytes.NewReader(body))
	scanner.Buffer(make([]byte, 0, 64<<10), responsesWebSocketReadLimit)
	for scanner.Scan() {
		data, ok := responsesSSEData(strings.TrimSpace(scanner.Text()))
		if !ok || data == "[DONE]" {
			continue
		}
		var event map[string]any
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			continue
		}
		if failure := codexResponsesFailureEventFromMap(event); failure != nil {
			return failure
		}
	}
	return nil
}

func codexResponsesFailureEventFromMap(event map[string]any) *codexResponsesFailureEvent {
	eventType, _ := event["type"].(string)
	response, _ := event["response"].(map[string]any)
	if response == nil {
		response = event
	}
	errorValue, _ := event["error"].(map[string]any)
	if len(errorValue) == 0 && response != nil {
		errorValue, _ = response["error"].(map[string]any)
	}

	status := ""
	responseID := ""
	if response != nil {
		status, _ = response["status"].(string)
		responseID, _ = response["id"].(string)
	}
	if eventType != "response.failed" && eventType != "response.incomplete" && eventType != "error" &&
		status != "failed" && status != "incomplete" {
		return nil
	}

	failure := &codexResponsesFailureEvent{
		EventType:  eventType,
		ResponseID: responseID,
		Status:     status,
	}
	if errorValue != nil {
		failure.ErrorType, _ = errorValue["type"].(string)
		failure.ErrorCode, _ = errorValue["code"].(string)
		failure.ErrorMessage, _ = errorValue["message"].(string)
	}
	return failure
}

func truncateResponseFailureMessage(message string) string {
	message = strings.TrimSpace(message)
	if message == "" {
		return ""
	}
	runes := []rune(message)
	if len(runes) > 512 {
		return string(runes[:512]) + "...[truncated]"
	}
	return message
}

func logCodexResponsesRequestShape(ctx context.Context, modelName string, stream bool, payload any) {
	zerolog.Ctx(ctx).Warn().
		Str("event", "responses.codex_request_shape").
		Str("model", modelName).
		Bool("stream", stream).
		Interface("request_shape", codexResponsesRequestShape(payload)).
		Msg("responses Codex backend request shape")
}

func logCodexResponsesRequestError(ctx context.Context, modelName string, stream bool, payload any, startTime time.Time, err error) {
	zerolog.Ctx(ctx).Warn().
		Str("event", "responses.codex_request_error").
		Str("model", modelName).
		Bool("stream", stream).
		Int64("elapsed_ms", time.Since(startTime).Milliseconds()).
		Str("error", redact.String(err.Error())).
		Interface("request_shape", codexResponsesRequestShape(payload)).
		Msg("responses Codex backend request failed")
}

func codexResponsesRequestShape(payload any) map[string]any {
	shape := map[string]any{"payload_type": fmt.Sprintf("%T", payload)}
	raw, ok := payload.(map[string]any)
	if !ok {
		return shape
	}
	keys := make([]string, 0, len(raw))
	for key := range raw {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	shape["keys"] = keys
	for _, key := range []string{"model", "store", "stream", "max_tokens", "max_output_tokens", "temperature", "top_p"} {
		if value, ok := raw[key]; ok {
			shape[key] = value
		}
	}
	if instructions, ok := raw["instructions"].(string); ok {
		shape["instructions_chars"] = len([]rune(instructions))
		shape["has_instructions"] = strings.TrimSpace(instructions) != ""
	}
	if reasoning, ok := raw["reasoning"]; ok {
		shape["has_reasoning"] = true
		shape["reasoning_type"] = fmt.Sprintf("%T", reasoning)
	}
	if input, ok := raw["input"]; ok {
		shape["input"] = codexResponsesInputShape(input)
	}
	return shape
}

func codexResponsesInputShape(input any) map[string]any {
	shape := map[string]any{"type": fmt.Sprintf("%T", input)}
	items, ok := input.([]any)
	if !ok {
		return shape
	}
	shape["count"] = len(items)
	roles := map[string]int{}
	itemTypes := map[string]int{}
	contentParts := 0
	for _, rawItem := range items {
		item, ok := rawItem.(map[string]any)
		if !ok {
			continue
		}
		if role, ok := item["role"].(string); ok {
			roles[role]++
		}
		if itemType, ok := item["type"].(string); ok {
			itemTypes[itemType]++
		}
		if content, ok := item["content"].([]any); ok {
			contentParts += len(content)
		}
	}
	shape["roles"] = roles
	shape["item_types"] = itemTypes
	shape["content_parts"] = contentParts
	return shape
}

func responsesRequestAuditPayload(receivedPayload map[string]any, upstreamPayload any) map[string]any {
	return map[string]any{
		"type":             "responses_request",
		"received_payload": redactImageInputDataURLs(receivedPayload),
		"upstream_payload": redactImageInputDataURLs(upstreamPayload),
	}
}

func responsesResponseAuditPayload(body []byte) map[string]any {
	result := map[string]any{
		"type":       "responses_response",
		"body_bytes": len(body),
	}
	var jsonPayload map[string]any
	if err := json.Unmarshal(body, &jsonPayload); err == nil {
		result["format"] = "json"
		result["response"] = responsesResponseObjectAuditPayload(jsonPayload)
		return result
	}

	result["format"] = "sse"
	events := make([]map[string]any, 0)
	scanner := bufio.NewScanner(bytes.NewReader(body))
	scanner.Buffer(make([]byte, 0, 64<<10), responsesWebSocketReadLimit)
	for scanner.Scan() {
		data, ok := responsesSSEData(scanner.Text())
		if !ok || data == "[DONE]" {
			continue
		}
		event := responsesEventAuditPayload([]byte(data))
		if event != nil {
			events = append(events, event)
		}
	}
	if err := scanner.Err(); err != nil {
		result["parse_error"] = err.Error()
	}
	result["events_len"] = len(events)
	result["events"] = events
	return result
}

func responsesEventAuditPayload(data []byte) map[string]any {
	var event map[string]any
	if err := json.Unmarshal(data, &event); err != nil {
		return map[string]any{
			"parse_error": err.Error(),
			"raw_len":     len(data),
		}
	}
	out := map[string]any{}
	if value, ok := event["type"]; ok {
		out["type"] = value
	}
	if value, ok := event["sequence_number"]; ok {
		out["sequence_number"] = value
	}
	if value, ok := event["delta"]; ok {
		out["delta"] = value
	}
	if value, ok := event["text"]; ok {
		out["text"] = value
	}
	if item, ok := event["item"].(map[string]any); ok {
		out["item"] = responsesItemAuditPayload(item)
	}
	if part, ok := event["part"].(map[string]any); ok {
		out["part"] = responsesPartAuditPayload(part)
	}
	if response, ok := event["response"].(map[string]any); ok {
		out["response"] = responsesResponseObjectAuditPayload(response)
	}
	return out
}

func responsesItemAuditPayload(item map[string]any) map[string]any {
	out := map[string]any{}
	for _, key := range []string{"id", "type", "status", "phase", "role", "summary"} {
		if value, ok := item[key]; ok {
			out[key] = value
		}
	}
	if content, ok := item["content"].([]any); ok {
		out["content_len"] = len(content)
		out["content"] = summarizeResponsesContent(content)
	}
	return out
}

func responsesPartAuditPayload(part map[string]any) map[string]any {
	out := map[string]any{}
	for _, key := range []string{"type", "text", "annotations", "logprobs"} {
		if value, ok := part[key]; ok {
			out[key] = value
		}
	}
	return out
}

func responsesResponseObjectAuditPayload(response map[string]any) map[string]any {
	out := map[string]any{}
	for _, key := range []string{
		"id", "object", "created_at", "status", "background", "completed_at", "error",
		"frequency_penalty", "incomplete_details", "instructions", "max_output_tokens",
		"max_tool_calls", "model", "moderation", "parallel_tool_calls", "presence_penalty",
		"previous_response_id", "prompt_cache_key", "prompt_cache_retention", "reasoning",
		"service_tier", "store", "temperature", "text", "tool_choice", "tool_usage",
		"tools", "top_logprobs", "top_p", "truncation", "usage", "user", "metadata",
	} {
		if value, ok := response[key]; ok {
			out[key] = value
		}
	}
	if output, ok := response["output"].([]any); ok {
		out["output_len"] = len(output)
		out["output"] = summarizeResponsesOutput(output)
	}
	return out
}

func summarizeResponsesOutput(output []any) []map[string]any {
	summary := make([]map[string]any, 0, len(output))
	for _, raw := range output {
		item, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		summary = append(summary, responsesItemAuditPayload(item))
	}
	return summary
}

func summarizeResponsesContent(content []any) []map[string]any {
	summary := make([]map[string]any, 0, len(content))
	for _, raw := range content {
		part, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		summary = append(summary, responsesPartAuditPayload(part))
	}
	return summary
}

func (h *Handlers) logResponsesFailure(ctx context.Context, modelName, reasoningEffort string, startTime time.Time, err error, subscriptionAttribution ...*callback.OpenAISubscriptionAttribution) {
	req := &model.ChatCompletionRequest{
		Model: modelName,
		ExtraParams: map[string]any{
			"reasoning_effort": reasoningEffort,
		},
	}
	h.recordErrorLog(ctx, req, nil, err)
	if h.Callbacks == nil {
		return
	}

	endTime := time.Now()
	data := buildBaseLogData(ctx, startTime)
	data.Model = modelName
	data.Provider = h.lookupProviderName(modelName)
	data.Request = req
	data.CallType = "responses"
	data.ReasoningEffort = reasoningEffort
	if len(subscriptionAttribution) > 0 {
		applyOpenAISubscriptionLogAttribution(&data, subscriptionAttribution[0])
	}
	data.EndTime = endTime
	data.Latency = endTime.Sub(startTime)
	data.Error = err

	go h.Callbacks.LogFailure(data)
}

func (h *Handlers) logResponsesUpstreamHTTPFailure(ctx context.Context, modelName, reasoningEffort string, startTime time.Time, statusCode int, body []byte) {
	message := responsesUpstreamHTTPFailureMessage(statusCode, body)
	zerolog.Ctx(ctx).Warn().
		Int("upstream_status", statusCode).
		Str("model", modelName).
		Str("upstream_error_body", redactedTruncatedResponseBody(body)).
		Msg("responses upstream returned non-200")
	h.logResponsesFailure(ctx, modelName, reasoningEffort, startTime, fmt.Errorf("%s", message))
}

func responsesUpstreamHTTPFailureMessage(statusCode int, body []byte) string {
	message := fmt.Sprintf("upstream error: status %d", statusCode)
	bodyText := redactedTruncatedResponseBody(body)
	if bodyText != "" {
		message += " body=" + bodyText
	}
	return message
}

func redactedTruncatedResponseBody(body []byte) string {
	bodyText := strings.TrimSpace(string(body))
	if bodyText == "" {
		return ""
	}
	runes := []rune(bodyText)
	if len(runes) > responsesUpstreamErrorBodyLogLimit {
		bodyText = string(runes[:responsesUpstreamErrorBodyLogLimit]) + "...[truncated]"
	}
	return redact.String(bodyText)
}

func singleRequestContentType(r *http.Request) (string, error) {
	contentTypes := r.Header.Values("Content-Type")
	if len(contentTypes) > 1 {
		return "", errors.New("multiple Content-Type values are not supported")
	}
	if len(contentTypes) != 1 || strings.TrimSpace(contentTypes[0]) == "" {
		return "", errors.New("Content-Type is required")
	}
	return strings.TrimSpace(contentTypes[0]), nil
}

func validateJSONRequestContentType(r *http.Request) error {
	contentType, err := singleRequestContentType(r)
	if err != nil {
		return err
	}
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return fmt.Errorf("invalid Content-Type: %w", err)
	}
	if !strings.EqualFold(mediaType, "application/json") {
		return fmt.Errorf("unsupported Content-Type %q; expected application/json", mediaType)
	}
	return nil
}

func decodeResponsesRequestBody(r *http.Request) (map[string]any, responsesRequestEnvelope, error) {
	if err := validateJSONRequestContentType(r); err != nil {
		return nil, responsesRequestEnvelope{}, err
	}
	body, err := readOpenAIRequestBody(r)
	if err != nil {
		return nil, responsesRequestEnvelope{}, err
	}
	r.Body = io.NopCloser(bytes.NewReader(body))

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, responsesRequestEnvelope{}, err
	}
	var envelope responsesRequestEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, responsesRequestEnvelope{}, err
	}
	return payload, envelope, nil
}

func decodeResponsesCreateFrame(frameData []byte) (map[string]any, responsesCreateFrame, error) {
	var payload map[string]any
	if err := json.Unmarshal(frameData, &payload); err != nil {
		return nil, responsesCreateFrame{}, err
	}
	var frame responsesCreateFrame
	if err := json.Unmarshal(frameData, &frame); err != nil {
		return nil, responsesCreateFrame{}, err
	}
	delete(payload, "type")
	return payload, frame, nil
}

func writeSyntheticResponsesWebSocketEvent(ctx context.Context, conn *websocket.Conn, eventType, responseID, modelName string) bool {
	payload, err := json.Marshal(map[string]any{
		"type": eventType,
		"response": map[string]any{
			"id":    responseID,
			"model": strings.TrimPrefix(strings.TrimPrefix(modelName, "openai/"), "chatgpt/"),
		},
	})
	if err != nil {
		_ = conn.Close(websocket.StatusInternalError, err.Error())
		return false
	}
	if err := conn.Write(ctx, websocket.MessageText, payload); err != nil {
		_ = conn.Close(websocket.StatusInternalError, err.Error())
		return false
	}
	return true
}

func isWebSocketUpgrade(r *http.Request) bool {
	return strings.EqualFold(r.Header.Get("Upgrade"), "websocket") &&
		strings.Contains(strings.ToLower(r.Header.Get("Connection")), "upgrade")
}

func sendSSEAsWebSocketMessages(ctx context.Context, conn *websocket.Conn, body io.Reader) (string, model.Usage, map[string]any, map[string]any, []any, bool) {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64<<10), responsesWebSocketReadLimit)
	var responseID string
	var usage model.Usage
	var completed bool
	var completedResponse map[string]any
	completedOutputItems := make([]any, 0)
	responsePayload := map[string]any{
		"type":   "responses_response",
		"format": "websocket_sse",
	}
	events := make([]map[string]any, 0)
	for scanner.Scan() {
		data, ok := responsesSSEData(scanner.Text())
		if !ok {
			continue
		}
		if data == "[DONE]" {
			responsePayload["events_len"] = len(events)
			responsePayload["events"] = events
			return responseID, usage, responsePayload, completedResponse, completedOutputItems, completed
		}
		if event := responsesEventAuditPayload([]byte(data)); event != nil {
			events = append(events, event)
		}
		if item := extractResponsesOutputItemDoneItem(data); item != nil {
			completedOutputItems = append(completedOutputItems, item)
		}
		if eventResponseID := extractResponsesEventID(data); strings.TrimSpace(eventResponseID) != "" {
			responseID = eventResponseID
		}
		if isResponsesCompletedEvent(data) {
			completed = true
			completedResponse = extractResponsesCompletedResponse(data)
		}
		if eventUsage := extractResponsesUsage([]byte(data)); usageHasTokens(eventUsage) {
			usage = eventUsage
		}
		if err := conn.Write(ctx, websocket.MessageText, []byte(data)); err != nil {
			_ = conn.Close(websocket.StatusInternalError, err.Error())
			return "", model.Usage{}, nil, nil, nil, false
		}
	}
	if err := scanner.Err(); err != nil {
		_ = conn.Close(websocket.StatusInternalError, err.Error())
		return "", model.Usage{}, nil, nil, nil, false
	}
	if !completed {
		return "", model.Usage{}, nil, nil, nil, false
	}
	responsePayload["events_len"] = len(events)
	responsePayload["events"] = events
	return responseID, usage, responsePayload, completedResponse, completedOutputItems, true
}

func extractResponsesEventID(data string) string {
	var event struct {
		Type     string `json:"type"`
		Response struct {
			ID string `json:"id"`
		} `json:"response"`
	}
	if err := json.Unmarshal([]byte(data), &event); err != nil {
		return ""
	}
	if event.Type != "response.created" && event.Type != "response.completed" {
		return ""
	}
	return strings.TrimSpace(event.Response.ID)
}

func isResponsesCompletedEvent(data string) bool {
	var event struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal([]byte(data), &event); err != nil {
		return false
	}
	return event.Type == "response.completed"
}

func extractResponsesCompletedResponse(data string) map[string]any {
	var event struct {
		Type     string         `json:"type"`
		Response map[string]any `json:"response"`
	}
	if err := json.Unmarshal([]byte(data), &event); err != nil {
		return nil
	}
	if event.Type != "response.completed" {
		return nil
	}
	return event.Response
}

func extractResponsesOutputItemDoneItem(data string) map[string]any {
	var event struct {
		Type string         `json:"type"`
		Item map[string]any `json:"item"`
	}
	if err := json.Unmarshal([]byte(data), &event); err != nil {
		return nil
	}
	if event.Type != "response.output_item.done" {
		return nil
	}
	return event.Item
}

func writeWebSocketError(conn *websocket.Conn, ctx context.Context, status websocket.StatusCode, message string) error {
	return writeWebSocketTypedError(conn, ctx, status, model.ErrorDetail{
		Message: message,
		Type:    "invalid_request_error",
	})
}

func writeWebSocketTypedError(conn *websocket.Conn, ctx context.Context, status websocket.StatusCode, detail model.ErrorDetail) error {
	payload, err := json.Marshal(model.ErrorResponse{
		Error: detail,
	})
	if err == nil {
		if writeErr := conn.Write(ctx, websocket.MessageText, payload); writeErr != nil {
			_ = conn.Close(status, detail.Message)
			return writeErr
		}
	}
	return conn.Close(status, detail.Message)
}

func extractResponsesUsage(data []byte) model.Usage {
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err == nil {
		if usage, ok := usageFromAny(payload["usage"]); ok {
			return usage
		}
		if response, ok := payload["response"].(map[string]any); ok {
			if usage, ok := usageFromAny(response["usage"]); ok {
				return usage
			}
		}
		return model.Usage{}
	}

	var usage model.Usage
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 0, 64<<10), responsesWebSocketReadLimit)
	for scanner.Scan() {
		line, ok := responsesSSEData(scanner.Text())
		if !ok || line == "[DONE]" {
			continue
		}
		eventUsage := extractResponsesUsage([]byte(line))
		if usageHasTokens(eventUsage) {
			usage = eventUsage
		}
	}
	return usage
}

func responsesSSEData(line string) (string, bool) {
	if !strings.HasPrefix(line, "data:") {
		return "", false
	}
	return strings.TrimSpace(strings.TrimPrefix(line, "data:")), true
}

func usageFromAny(value any) (model.Usage, bool) {
	usageMap, ok := value.(map[string]any)
	if !ok {
		return model.Usage{}, false
	}
	usage := model.Usage{
		PromptTokens:             intFromUsage(usageMap, "prompt_tokens", "input_tokens"),
		CompletionTokens:         intFromUsage(usageMap, "completion_tokens", "output_tokens"),
		TotalTokens:              intFromUsage(usageMap, "total_tokens"),
		CacheReadInputTokens:     intFromNestedUsage(usageMap, "input_tokens_details", "cached_tokens", "cache_read_input_tokens"),
		CacheCreationInputTokens: intFromUsage(usageMap, "cache_creation_input_tokens"),
	}
	if usage.TotalTokens == 0 {
		usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
	}
	return usage, usageHasTokens(usage)
}

func usageHasTokens(usage model.Usage) bool {
	return usage.PromptTokens != 0 || usage.CompletionTokens != 0 || usage.TotalTokens != 0 || usage.CacheReadInputTokens != 0 || usage.CacheCreationInputTokens != 0
}

func intFromUsage(values map[string]any, keys ...string) int {
	for _, key := range keys {
		switch value := values[key].(type) {
		case float64:
			return int(value)
		case int:
			return value
		case json.Number:
			if i, err := value.Int64(); err == nil {
				return int(i)
			}
		}
	}
	return 0
}

func intFromNestedUsage(values map[string]any, objectKey string, nestedKey string, fallbackKeys ...string) int {
	if nested, ok := values[objectKey].(map[string]any); ok {
		if value := intFromUsage(nested, nestedKey); value != 0 {
			return value
		}
	}
	return intFromUsage(values, fallbackKeys...)
}

func copyHTTPResponse(w http.ResponseWriter, resp *http.Response) []byte {
	body, _ := copyHTTPResponseWithError(w, resp)
	return body
}

func copyHTTPResponseWithError(w http.ResponseWriter, resp *http.Response) ([]byte, error) {
	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	w.WriteHeader(resp.StatusCode)
	var body bytes.Buffer
	_, err := io.Copy(io.MultiWriter(w, &body), resp.Body)
	return body.Bytes(), err
}
