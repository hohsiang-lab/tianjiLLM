package handler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/rs/zerolog"

	"github.com/praxisllmlab/tianjiLLM/internal/callback"
	"github.com/praxisllmlab/tianjiLLM/internal/guardrail"
	"github.com/praxisllmlab/tianjiLLM/internal/model"
	"github.com/praxisllmlab/tianjiLLM/internal/pricing"
	"github.com/praxisllmlab/tianjiLLM/internal/provider"
	"github.com/praxisllmlab/tianjiLLM/internal/proxy/middleware"
	"github.com/praxisllmlab/tianjiLLM/internal/router"
	"github.com/praxisllmlab/tianjiLLM/internal/security/redact"
)

// ChatCompletion handles POST /v1/chat/completions.
func (h *Handlers) ChatCompletion(w http.ResponseWriter, r *http.Request) {
	var req model.ChatCompletionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{
				Message: "invalid request body: " + err.Error(),
				Type:    "invalid_request_error",
			},
		})
		return
	}

	// Resolve prompt template if PromptName is set
	if req.PromptName != "" {
		if err := resolvePromptTemplate(r.Context(), h.DB, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
				Error: model.ErrorDetail{
					Message: err.Error(),
					Type:    "invalid_request_error",
				},
			})
			return
		}
	}

	originalModel := req.Model
	route, err := h.resolveProviderRoute(r.Context(), &req, providerRouteResolutionOptions{
		skipCredentialResolutionForUnsupportedCodexChat: true,
	})
	if err != nil {
		var status int
		var code string
		switch {
		case errors.Is(err, router.ErrNoDeployments):
			status = http.StatusNotFound
			code = "model_not_found"
		case strings.Contains(err.Error(), "not found"):
			status = http.StatusNotFound
			code = "model_not_found"
		default:
			status = http.StatusBadRequest
		}
		writeJSON(w, status, model.ErrorResponse{
			Error: model.ErrorDetail{
				Message: err.Error(),
				Type:    "invalid_request_error",
				Code:    code,
			},
		})
		return
	}
	if route.ChatGPTCodexBackend {
		writeUnsupportedChatGPTCodexEndpoint(w, "chat completions")
		return
	}
	if requestErr := validateChatCapabilities(&req, route.Capability); requestErr != nil {
		writeRequestError(w, requestErr)
		return
	}

	req.Model = route.ModelName

	// Phase 2: provider.resolved
	middleware.LogProviderResolved(r.Context(), h.lookupProviderName(originalModel), route.Provider.GetRequestURL(route.ModelName), "chat", route.ModelName)

	// Evaluate policy engine — merge guardrails from policies
	guardrailNames := h.getGuardrailNames(r.Context())
	if h.PolicyEngine != nil {
		policyReq := h.buildPolicyRequest(r.Context(), &req)
		policyResult := h.PolicyEngine.Evaluate(policyReq)
		if len(policyResult.Guardrails) > 0 {
			guardrailNames = mergeStrings(guardrailNames, policyResult.Guardrails)
		}
	}

	// Run pre-call guardrails
	if len(guardrailNames) > 0 && h.Guardrails != nil {
		modified, err := h.Guardrails.RunPreCall(r.Context(), guardrailNames, &req)
		if err != nil {
			status := http.StatusBadRequest
			if _, ok := err.(*guardrail.BlockedError); ok {
				status = http.StatusForbidden
			}
			writeJSON(w, status, model.ErrorResponse{
				Error: model.ErrorDetail{
					Message: err.Error(),
					Type:    "guardrail_error",
				},
			})
			return
		}
		if modified != nil {
			req = *modified
		}
		if requestErr := validateChatCapabilities(&req, route.Capability); requestErr != nil {
			writeRequestError(w, requestErr)
			return
		}
	}

	// Log warnings for unknown parameters that will be passed through
	if len(req.ExtraParams) > 0 {
		keys := make([]string, 0, len(req.ExtraParams))
		for k := range req.ExtraParams {
			keys = append(keys, k)
		}
		zerolog.Ctx(r.Context()).Warn().Strs("unknown_params", keys).Msg("unknown parameters forwarded to upstream")
	}

	if req.IsStreaming() {
		h.handleStreamingCompletion(w, r, route.Provider, &req, route.APIKey, route.SubscriptionCandidates)
		return
	}

	if !route.Capability.SupportsNonStream && route.Capability.SupportsStream {
		h.handleAggregatedStreamingCompletion(w, r, route.Provider, &req, route.APIKey, route.SubscriptionCandidates, route.Capability.SupportsStreamOptionsIncludeUsage)
		return
	}

	h.handleNonStreamingCompletion(w, r, route.Provider, &req, route.APIKey, route.SubscriptionCandidates)
}

// resolveProvider resolves the model to a provider, using Router if available.
// On failure, tries general fallback chain before returning an error.
func (h *Handlers) resolveProvider(ctx context.Context, req *model.ChatCompletionRequest) (provider.Provider, string, string, error) {
	route, err := h.resolveProviderRoute(ctx, req)
	if err != nil {
		return nil, "", "", err
	}
	return route.Provider, route.APIKey, route.ModelName, nil
}

func (h *Handlers) resolveProviderRoute(ctx context.Context, req *model.ChatCompletionRequest, options ...providerRouteResolutionOptions) (resolvedProviderRoute, error) {
	var routeOptions providerRouteResolutionOptions
	if len(options) > 0 {
		routeOptions = options[0]
	}
	// Use Router if configured (multi-deployment load balancing)
	if runtimeRouter := h.runtimeRouter(ctx); runtimeRouter != nil {
		d, p, err := runtimeRouter.Route(ctx, req.Model, req)
		if err == nil {
			routeParams := normalizeOfficialOpenAIParams(d.Config.TianjiParams)
			auth, resolveErr := h.resolveOpenAISubscriptionRouteForResolvedModel(ctx, routeParams.Model, routeParams, routeOptions)
			if resolveErr != nil {
				return resolvedProviderRoute{}, resolveErr
			}
			route := resolvedProviderRoute{
				Provider:               p,
				APIKey:                 auth.APIKey,
				PublicModel:            d.Config.ModelName,
				ModelName:              d.ModelName,
				Backend:                model.BackendDirectOpenAIHTTP,
				TianjiParams:           routeParams,
				SubscriptionCandidates: auth.Candidates,
				ChatGPTCodexBackend:    isChatGPTCodexBackendTransport(routeParams),
			}
			if route.ChatGPTCodexBackend {
				route.Backend = model.BackendChatGPTCodex
			}
			return h.attachRouteCapabilities(route), nil
		}

		// Try general fallback chain
		d, p, fbErr := runtimeRouter.GeneralFallback(ctx, req.Model)
		if fbErr == nil {
			zerolog.Ctx(ctx).Info().Str("event", "model.fallback").Str("from", req.Model).Str("to", d.ModelName).Msg("fallback activated")
			routeParams := normalizeOfficialOpenAIParams(d.Config.TianjiParams)
			auth, resolveErr := h.resolveOpenAISubscriptionRouteForResolvedModel(ctx, routeParams.Model, routeParams, routeOptions)
			if resolveErr != nil {
				return resolvedProviderRoute{}, resolveErr
			}
			route := resolvedProviderRoute{
				Provider:               p,
				APIKey:                 auth.APIKey,
				PublicModel:            d.Config.ModelName,
				ModelName:              d.ModelName,
				Backend:                model.BackendDirectOpenAIHTTP,
				TianjiParams:           routeParams,
				SubscriptionCandidates: auth.Candidates,
				ChatGPTCodexBackend:    isChatGPTCodexBackendTransport(routeParams),
			}
			if route.ChatGPTCodexBackend {
				route.Backend = model.BackendChatGPTCodex
			}
			return h.attachRouteCapabilities(route), nil
		}

		return resolvedProviderRoute{}, err
	}

	// Direct resolution (single deployment)
	return h.resolveProviderFromConfigRouteWithContext(ctx, req.Model, routeOptions)
}

// cacheKey generates a deterministic cache key from model name and messages.
func cacheKey(modelName string, messages []model.Message) string {
	h := sha256.New()
	h.Write([]byte(modelName))
	// Sort messages by role for determinism (same messages in any order → same key)
	sorted := make([]model.Message, len(messages))
	copy(sorted, messages)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Role != sorted[j].Role {
			return sorted[i].Role < sorted[j].Role
		}
		ci, _ := json.Marshal(sorted[i].Content)
		cj, _ := json.Marshal(sorted[j].Content)
		return string(ci) < string(cj)
	})
	data, _ := json.Marshal(sorted)
	h.Write(data)
	return "tianji:cache:" + hex.EncodeToString(h.Sum(nil))
}

// defaultCacheTTL is the default cache TTL for LLM responses.
const defaultCacheTTL = 5 * time.Minute

func (h *Handlers) handleNonStreamingCompletion(w http.ResponseWriter, r *http.Request, p provider.Provider, req *model.ChatCompletionRequest, apiKey string, candidates []resolvedOpenAISubscriptionCredential) {
	startTime := time.Now()

	if h.writeCachedCompletion(r.Context(), w, req) {
		return
	}

	resp, llmLatency, subscriptionAttribution, err := h.doOpenAISubscriptionProviderRequest(r.Context(), apiKey, candidates, func(attempt openAISubscriptionProviderAttempt) (*http.Request, error) {
		return p.TransformRequest(r.Context(), req, attempt.apiKey)
	})
	if err != nil {
		// Phase 3: upstream.responded (error)
		middleware.LogUpstreamResponded(r.Context(), middleware.UpstreamResult{
			StatusCode: 0,
			LatencyMs:  float64(llmLatency.Milliseconds()),
			Error:      err.Error(),
		})
		h.logFailure(r.Context(), req, p, startTime, fmt.Errorf("upstream request failed: %w", err))
		writeUpstreamRequestFailure(w, err, req.Model)
		return
	}

	// Phase 3: upstream.responded
	middleware.LogUpstreamResponded(r.Context(), middleware.UpstreamResult{
		StatusCode: resp.StatusCode,
		LatencyMs:  float64(llmLatency.Milliseconds()),
	})

	result, err := p.TransformResponse(r.Context(), resp)
	if err != nil {
		h.logFailure(r.Context(), req, p, startTime, fmt.Errorf("transform response: %w", err))
		writeTransformError(w, err, req.Model)
		return
	}

	endTime := time.Now()
	h.logSuccess(r.Context(), req, result, p, startTime, endTime, llmLatency, subscriptionAttribution)

	h.cacheCompletionResponse(r.Context(), req, result)

	writeJSON(w, http.StatusOK, result)
}

func (h *Handlers) handleAggregatedStreamingCompletion(w http.ResponseWriter, r *http.Request, p provider.Provider, req *model.ChatCompletionRequest, apiKey string, candidates []resolvedOpenAISubscriptionCredential, includeUsage bool) {
	startTime := time.Now()
	upstreamReq := requestWithStreaming(req, includeUsage)
	resp, llmLatency, subscriptionAttribution, err := h.doOpenAISubscriptionProviderRequest(r.Context(), apiKey, candidates, func(attempt openAISubscriptionProviderAttempt) (*http.Request, error) {
		return p.TransformRequest(r.Context(), upstreamReq, attempt.apiKey)
	})
	if err != nil {
		middleware.LogUpstreamResponded(r.Context(), middleware.UpstreamResult{
			LatencyMs: float64(llmLatency.Milliseconds()),
			Error:     err.Error(),
		})
		h.logFailure(r.Context(), req, p, startTime, fmt.Errorf("upstream request failed: %w", err))
		writeUpstreamRequestFailure(w, err, req.Model)
		return
	}
	defer resp.Body.Close()

	middleware.LogUpstreamResponded(r.Context(), middleware.UpstreamResult{
		StatusCode: resp.StatusCode,
		LatencyMs:  float64(llmLatency.Milliseconds()),
	})
	if resp.StatusCode != http.StatusOK {
		_, transformErr := p.TransformResponse(r.Context(), resp)
		if transformErr == nil {
			transformErr = fmt.Errorf("upstream error: status %d", resp.StatusCode)
		}
		h.logFailure(r.Context(), req, p, startTime, transformErr)
		writeUpstreamHTTPFailure(w, transformErr, req.Model)
		return
	}

	var aggregate streamAggregate
	err = consumeCompletionStream(
		resp.Body,
		func(data []byte) (*model.StreamChunk, bool, error) {
			return p.TransformStreamChunk(r.Context(), data)
		},
		func(chunk *model.StreamChunk, done bool) error {
			aggregate.Add(chunk)
			if done {
				aggregate.Complete()
			}
			return nil
		},
	)
	var result *model.ModelResponse
	if err == nil {
		if includeUsage {
			result, err = aggregate.ResponseWithUsage(req.Model)
		} else {
			result, err = aggregate.Response(req.Model)
		}
	}
	if err != nil {
		h.logFailure(r.Context(), req, p, startTime, err)
		writeUpstreamStreamFailure(w, err)
		return
	}

	h.logSuccess(r.Context(), req, result, p, startTime, time.Now(), llmLatency, subscriptionAttribution)
	writeJSON(w, http.StatusOK, result)
}

func requestWithStreaming(req *model.ChatCompletionRequest, includeUsage bool) *model.ChatCompletionRequest {
	upstream := *req
	stream := true
	upstream.Stream = &stream
	upstream.StreamOptions = nil
	if includeUsage {
		upstream.StreamOptions = &model.StreamOptions{IncludeUsage: true}
	}
	return &upstream
}

func (h *Handlers) writeCachedCompletion(ctx context.Context, w http.ResponseWriter, req *model.ChatCompletionRequest) bool {
	if h.Cache == nil {
		return false
	}
	cached, err := h.Cache.Get(ctx, cacheKey(req.Model, req.Messages))
	if err != nil || len(cached) == 0 {
		return false
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Cache", "HIT")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(cached)
	return true
}

func (h *Handlers) cacheCompletionResponse(ctx context.Context, req *model.ChatCompletionRequest, response *model.ModelResponse) {
	if h.Cache == nil || response == nil {
		return
	}
	if data, err := json.Marshal(response); err == nil {
		_ = h.Cache.Set(ctx, cacheKey(req.Model, req.Messages), data, defaultCacheTTL)
	}
}

func writeTransformError(w http.ResponseWriter, err error, fallbackModel string) {
	var upstreamErr *model.TianjiError
	if errors.As(err, &upstreamErr) &&
		(shouldExposeTransformError(upstreamErr) || shouldExposeActionableUpstreamError(upstreamErr)) {
		code := upstreamErr.Code
		if upstreamErr.StatusCode == http.StatusUnauthorized &&
			strings.Contains(upstreamErr.Message, "OpenAI subscription reauthorization required") {
			code = "openai_subscription_reauthorization_required"
		}
		modelName := upstreamErr.Model
		if modelName == "" {
			modelName = fallbackModel
		}
		writeJSON(w, upstreamErr.StatusCode, model.ErrorResponse{
			Error: model.ErrorDetail{
				Message:  redact.String(upstreamErr.Message),
				Type:     upstreamErr.Type,
				Code:     code,
				Provider: upstreamErr.Provider,
				Model:    modelName,
			},
		})
		return
	}
	writeJSON(w, http.StatusBadGateway, model.ErrorResponse{
		Error: model.ErrorDetail{
			Message: redact.String("transform response: " + err.Error()),
			Type:    "internal_error",
		},
	})
}

func shouldExposeTransformError(err *model.TianjiError) bool {
	if err.StatusCode < 400 || err.StatusCode >= 500 {
		return false
	}
	if err.Provider == "chatgpt_codex_backend" {
		return true
	}
	return err.StatusCode == http.StatusUnauthorized &&
		strings.Contains(err.Message, "OpenAI subscription reauthorization required")
}

func shouldExposeActionableUpstreamError(err *model.TianjiError) bool {
	return err.StatusCode >= 400 && err.StatusCode < 500 &&
		err.StatusCode != http.StatusUnauthorized &&
		err.StatusCode != http.StatusForbidden &&
		(err.Code != "" || (err.Type != "" && err.Type != "api_error"))
}

func (h *Handlers) handleStreamingCompletion(w http.ResponseWriter, r *http.Request, p provider.Provider, req *model.ChatCompletionRequest, apiKey string, subscriptionCandidates ...[]resolvedOpenAISubscriptionCredential) {
	startTime := time.Now()
	var candidates []resolvedOpenAISubscriptionCredential
	if len(subscriptionCandidates) > 0 {
		candidates = subscriptionCandidates[0]
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		h.logFailure(r.Context(), req, p, startTime, fmt.Errorf("streaming not supported"))
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{
				Message: "streaming not supported",
				Type:    "internal_error",
			},
		})
		return
	}

	resp, llmLatency, subscriptionAttribution, err := h.doOpenAISubscriptionProviderRequest(r.Context(), apiKey, candidates, func(attempt openAISubscriptionProviderAttempt) (*http.Request, error) {
		return p.TransformRequest(r.Context(), req, attempt.apiKey)
	})
	if err != nil {
		// Phase 3: upstream.responded (error)
		middleware.LogUpstreamResponded(r.Context(), middleware.UpstreamResult{
			StatusCode: 0,
			LatencyMs:  float64(llmLatency.Milliseconds()),
			Error:      err.Error(),
		})
		h.logFailure(r.Context(), req, p, startTime, fmt.Errorf("upstream request failed: %w", err))
		writeUpstreamStreamFailure(w, err)
		return
	}
	defer resp.Body.Close()

	// Phase 3: upstream.responded
	middleware.LogUpstreamResponded(r.Context(), middleware.UpstreamResult{
		StatusCode: resp.StatusCode,
		LatencyMs:  float64(llmLatency.Milliseconds()),
	})

	if resp.StatusCode != http.StatusOK {
		_, transformErr := p.TransformResponse(r.Context(), resp)
		if transformErr == nil {
			transformErr = fmt.Errorf("upstream error: status %d", resp.StatusCode)
		}
		h.logFailure(r.Context(), req, p, startTime, transformErr)
		writeUpstreamHTTPFailure(w, transformErr, req.Model)
		return
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	var lastChunk *model.StreamChunk
	var accUsage model.Usage
	var aggregate streamAggregate
	var timeToFirstToken time.Duration
	err = consumeCompletionStream(
		resp.Body,
		func(data []byte) (*model.StreamChunk, bool, error) {
			return p.TransformStreamChunk(r.Context(), data)
		},
		func(chunk *model.StreamChunk, done bool) error {
			if chunk != nil {
				aggregate.Add(chunk)
				lastChunk = chunk
				mergeStreamUsage(&accUsage, chunk.Usage)
				if timeToFirstToken == 0 && streamChunkHasOutput(chunk) {
					timeToFirstToken = time.Since(startTime)
				}
				chunkData, marshalErr := json.Marshal(chunk)
				if marshalErr != nil {
					return fmt.Errorf("marshal stream chunk: %w", marshalErr)
				}
				if _, writeErr := fmt.Fprintf(w, "data: %s\n\n", chunkData); writeErr != nil {
					return fmt.Errorf("write stream chunk: %w", writeErr)
				}
				flusher.Flush()
			}
			if done {
				aggregate.Complete()
			}
			return nil
		},
	)
	if err == nil {
		if req.StreamOptions != nil && req.StreamOptions.IncludeUsage {
			_, err = aggregate.ResponseWithUsage(req.Model)
		} else {
			_, err = aggregate.Response(req.Model)
		}
	}
	if err != nil {
		h.logFailure(r.Context(), req, p, startTime, err)
		writeUpstreamStreamFailureEvent(w, err)
		flusher.Flush()
		return
	}

	_, _ = fmt.Fprintf(w, "data: [DONE]\n\n")
	flusher.Flush()
	h.logStreamSuccess(r.Context(), req, lastChunk, accUsage, p, startTime, time.Now(), llmLatency, timeToFirstToken, subscriptionAttribution)
}

func mergeStreamUsage(acc *model.Usage, usage *model.Usage) {
	if usage == nil {
		return
	}
	if usage.PromptTokens > 0 {
		acc.PromptTokens = usage.PromptTokens
	}
	if usage.CompletionTokens > 0 {
		acc.CompletionTokens = usage.CompletionTokens
	}
	if usage.CacheReadInputTokens > 0 {
		acc.CacheReadInputTokens = usage.CacheReadInputTokens
	}
	if usage.CacheCreationInputTokens > 0 {
		acc.CacheCreationInputTokens = usage.CacheCreationInputTokens
	}
	if usage.TotalTokens > 0 {
		acc.TotalTokens = usage.TotalTokens
	}
	if acc.PromptTokens > 0 || acc.CompletionTokens > 0 {
		acc.TotalTokens = acc.PromptTokens + acc.CompletionTokens
	}
}

func streamChunkHasOutput(chunk *model.StreamChunk) bool {
	for _, choice := range chunk.Choices {
		if choice.Delta.Content != nil && *choice.Delta.Content != "" {
			return true
		}
		if choice.Delta.Refusal != nil && *choice.Delta.Refusal != "" {
			return true
		}
		if len(choice.Delta.ToolCalls) > 0 {
			return true
		}
	}
	return false
}

func upstreamStreamErrorDetail(err error) model.ErrorDetail {
	detail := model.ErrorDetail{
		Message: "upstream stream failed",
		Type:    "api_error",
		Code:    "upstream_stream_error",
	}
	var upstreamErr *model.TianjiError
	if errors.As(err, &upstreamErr) {
		if upstreamErr.Type != "" {
			detail.Type = upstreamErr.Type
		}
		if upstreamErr.Code != "" {
			detail.Code = upstreamErr.Code
		}
	}
	return detail
}

func writeUpstreamStreamFailure(w http.ResponseWriter, err error) {
	writeJSON(w, http.StatusBadGateway, model.ErrorResponse{Error: upstreamStreamErrorDetail(err)})
}

func writeUpstreamHTTPFailure(w http.ResponseWriter, err error, fallbackModel string) {
	var upstreamErr *model.TianjiError
	if errors.As(err, &upstreamErr) &&
		(shouldExposeTransformError(upstreamErr) || shouldExposeActionableUpstreamError(upstreamErr)) {
		writeTransformError(w, err, fallbackModel)
		return
	}
	if errors.As(err, &upstreamErr) && upstreamErr.StatusCode >= 400 && upstreamErr.StatusCode < 600 {
		reason := http.StatusText(upstreamErr.StatusCode)
		if reason == "" {
			reason = redact.String(upstreamErr.Message)
		}
		if strings.TrimSpace(reason) == "" {
			reason = "unknown upstream error"
		}
		writeJSON(w, upstreamErr.StatusCode, model.ErrorResponse{Error: model.ErrorDetail{
			Message: "upstream request failed: " + reason,
			Type:    "api_error",
			Code:    "upstream_error",
		}})
		return
	}
	writeUpstreamRequestFailure(w, err, fallbackModel)
}

func writeUpstreamRequestFailure(w http.ResponseWriter, err error, fallbackModel string) {
	var upstreamErr *model.TianjiError
	if errors.As(err, &upstreamErr) && shouldExposeTransformError(upstreamErr) {
		writeTransformError(w, err, fallbackModel)
		return
	}
	writeJSON(w, http.StatusBadGateway, model.ErrorResponse{Error: model.ErrorDetail{
		Message: "upstream request failed",
		Type:    "api_error",
		Code:    "upstream_error",
	}})
}

func writeUpstreamStreamFailureEvent(w io.Writer, err error) {
	data, marshalErr := json.Marshal(model.ErrorResponse{Error: upstreamStreamErrorDetail(err)})
	if marshalErr != nil {
		return
	}
	_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
	_, _ = fmt.Fprintf(w, "data: [DONE]\n\n")
}

// buildLogData constructs the common callback.LogData from request context.
func (h *Handlers) buildLogData(ctx context.Context, req *model.ChatCompletionRequest, p provider.Provider, startTime time.Time) callback.LogData {
	data := buildBaseLogData(ctx, startTime)
	data.Model = req.Model
	data.Request = req
	data.ReasoningEffort = reasoningEffortFromChatRequest(req)

	if p != nil {
		providerName := fmt.Sprintf("%T", p)
		if idx := strings.LastIndex(providerName, "."); idx >= 0 {
			providerName = providerName[:idx]
		}
		if idx := strings.LastIndex(providerName, "/"); idx >= 0 {
			providerName = providerName[idx+1:]
		}
		data.Provider = strings.TrimPrefix(providerName, "*")
	}

	return data
}

// logSuccess fires success callbacks for non-streaming responses.
func (h *Handlers) logSuccess(ctx context.Context, req *model.ChatCompletionRequest, result *model.ModelResponse, p provider.Provider, startTime, endTime time.Time, llmLatency time.Duration, subscriptionAttribution ...*callback.OpenAISubscriptionAttribution) {
	if h.Callbacks == nil {
		return
	}

	data := h.buildLogData(ctx, req, p, startTime)
	data.Response = result
	data.EndTime = endTime
	data.Latency = endTime.Sub(startTime)
	data.LLMAPILatency = llmLatency
	if len(subscriptionAttribution) > 0 {
		applyOpenAISubscriptionLogAttribution(&data, subscriptionAttribution[0])
	}

	if result != nil {
		data.PromptTokens = result.Usage.PromptTokens
		data.CompletionTokens = result.Usage.CompletionTokens
		data.TotalTokens = result.Usage.TotalTokens
		data.CacheReadInputTokens = result.Usage.CacheReadInputTokens
		data.CacheCreationInputTokens = result.Usage.CacheCreationInputTokens
		data.Cost = pricing.Default().TotalCost(req.Model, pricing.TokenUsage{
			PromptTokens:             result.Usage.PromptTokens,
			CompletionTokens:         result.Usage.CompletionTokens,
			CacheReadInputTokens:     result.Usage.CacheReadInputTokens,
			CacheCreationInputTokens: result.Usage.CacheCreationInputTokens,
		})
	}

	go h.Callbacks.LogSuccess(data)
}

// logStreamSuccess fires success callbacks for streaming responses.
// accUsage carries prompt/completion tokens accumulated across all chunks
// (different providers emit them in different events). Falls back to
// lastChunk.Usage when accUsage is empty.
func (h *Handlers) logStreamSuccess(ctx context.Context, req *model.ChatCompletionRequest, lastChunk *model.StreamChunk, accUsage model.Usage, p provider.Provider, startTime, endTime time.Time, llmLatency, timeToFirstToken time.Duration, subscriptionAttribution ...*callback.OpenAISubscriptionAttribution) {
	if h.Callbacks == nil {
		return
	}

	data := h.buildLogData(ctx, req, p, startTime)
	data.EndTime = endTime
	data.Latency = endTime.Sub(startTime)
	data.LLMAPILatency = llmLatency
	data.TimeToFirstToken = timeToFirstToken
	if len(subscriptionAttribution) > 0 {
		applyOpenAISubscriptionLogAttribution(&data, subscriptionAttribution[0])
	}

	promptTokens := accUsage.PromptTokens
	completionTokens := accUsage.CompletionTokens
	cacheReadTokens := accUsage.CacheReadInputTokens
	cacheCreationTokens := accUsage.CacheCreationInputTokens
	if promptTokens == 0 && completionTokens == 0 && lastChunk != nil && lastChunk.Usage != nil {
		promptTokens = lastChunk.Usage.PromptTokens
		completionTokens = lastChunk.Usage.CompletionTokens
		cacheReadTokens = lastChunk.Usage.CacheReadInputTokens
		cacheCreationTokens = lastChunk.Usage.CacheCreationInputTokens
	}
	if promptTokens > 0 || completionTokens > 0 {
		data.PromptTokens = promptTokens
		data.CompletionTokens = completionTokens
		data.TotalTokens = promptTokens + completionTokens
		data.CacheReadInputTokens = cacheReadTokens
		data.CacheCreationInputTokens = cacheCreationTokens
		data.Cost = pricing.Default().TotalCost(req.Model, pricing.TokenUsage{
			PromptTokens:             promptTokens,
			CompletionTokens:         completionTokens,
			CacheReadInputTokens:     cacheReadTokens,
			CacheCreationInputTokens: cacheCreationTokens,
		})
	}

	go h.Callbacks.LogSuccess(data)
}

// logFailure fires failure callbacks.
func (h *Handlers) logFailure(ctx context.Context, req *model.ChatCompletionRequest, p provider.Provider, startTime time.Time, err error) {
	// Record error to ErrorLogs table (fire-and-forget)
	h.recordErrorLog(ctx, req, p, err)

	if h.Callbacks == nil {
		return
	}

	data := h.buildLogData(ctx, req, p, startTime)
	data.EndTime = time.Now()
	data.Latency = data.EndTime.Sub(startTime)
	data.Error = err

	go h.Callbacks.LogFailure(data)
}

// getGuardrailNames extracts guardrail names from the request context.
// These are set by auth middleware from key/team guardrail configuration.
func (h *Handlers) getGuardrailNames(ctx context.Context) []string {
	if names, ok := ctx.Value(middleware.ContextKeyGuardrails).([]string); ok {
		return names
	}
	return nil
}

// buildPolicyRequest constructs a PolicyRequest from the request context.
func (h *Handlers) buildPolicyRequest(ctx context.Context, req *model.ChatCompletionRequest) router.PolicyRequest {
	pr := router.PolicyRequest{
		Model: req.Model,
	}
	if teamID, ok := ctx.Value(middleware.ContextKeyTeamID).(string); ok {
		pr.TeamAlias = teamID
	}
	return pr
}

// mergeStrings merges two string slices, deduplicating entries.
func mergeStrings(a, b []string) []string {
	seen := make(map[string]struct{}, len(a)+len(b))
	result := make([]string, 0, len(a)+len(b))
	for _, s := range a {
		if _, ok := seen[s]; !ok {
			seen[s] = struct{}{}
			result = append(result, s)
		}
	}
	for _, s := range b {
		if _, ok := seen[s]; !ok {
			seen[s] = struct{}{}
			result = append(result, s)
		}
	}
	return result
}

// recordErrorLog persists an error to the ErrorLogs table (fire-and-forget).
func (h *Handlers) recordErrorLog(ctx context.Context, req *model.ChatCompletionRequest, p provider.Provider, err error) {
	if h.DB == nil || err == nil {
		return
	}

	providerName := ""
	if p != nil {
		providerName = fmt.Sprintf("%T", p)
		if idx := strings.LastIndex(providerName, "."); idx >= 0 {
			providerName = providerName[:idx]
		}
		if idx := strings.LastIndex(providerName, "/"); idx >= 0 {
			providerName = providerName[idx+1:]
		}
		providerName = strings.TrimPrefix(providerName, "*")
	}

	params := errorLogParamsFromContext(ctx)
	params.Provider = providerName
	params.StatusCode = 500
	params.ErrorType = "provider_error"
	params.ErrorMessage = redact.String(err.Error())
	if req != nil {
		params.Model = req.Model
		params.ReasoningEffort = reasoningEffortFromChatRequest(req)
	}

	logger := zerolog.Ctx(ctx)
	go func() {
		if dbErr := h.DB.InsertErrorLog(context.Background(), params); dbErr != nil {
			logger.Error().Err(dbErr).Str("request_id", params.RequestID).Msg("InsertErrorLog failed")
		}
	}()
}
