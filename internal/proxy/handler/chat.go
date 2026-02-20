package handler

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/praxisllmlab/tianjiLLM/internal/callback"
	"github.com/praxisllmlab/tianjiLLM/internal/db"
	"github.com/praxisllmlab/tianjiLLM/internal/guardrail"
	"github.com/praxisllmlab/tianjiLLM/internal/model"
	"github.com/praxisllmlab/tianjiLLM/internal/pricing"
	"github.com/praxisllmlab/tianjiLLM/internal/provider"
	"github.com/praxisllmlab/tianjiLLM/internal/proxy/middleware"
	"github.com/praxisllmlab/tianjiLLM/internal/router"
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

	p, apiKey, modelName, err := h.resolveProvider(r.Context(), &req)
	if err != nil {
		status := http.StatusNotFound
		code := "model_not_found"
		if !strings.Contains(err.Error(), "not found") {
			status = http.StatusBadRequest
			code = ""
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

	req.Model = modelName

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
	}

	// Log warnings for unknown parameters that will be passed through
	if len(req.ExtraParams) > 0 {
		keys := make([]string, 0, len(req.ExtraParams))
		for k := range req.ExtraParams {
			keys = append(keys, k)
		}
		log.Printf("warn: unknown parameters forwarded to upstream: %v", keys)
	}

	if req.IsStreaming() {
		h.handleStreamingCompletion(w, r, p, &req, apiKey)
		return
	}

	h.handleNonStreamingCompletion(w, r, p, &req, apiKey)
}

// resolveProvider resolves the model to a provider, using Router if available.
// On failure, tries general fallback chain before returning an error.
func (h *Handlers) resolveProvider(ctx context.Context, req *model.ChatCompletionRequest) (provider.Provider, string, string, error) {
	// Use Router if configured (multi-deployment load balancing)
	if h.Router != nil {
		d, p, err := h.Router.Route(ctx, req.Model, req)
		if err == nil {
			return p, d.APIKey(), d.ModelName, nil
		}

		// Try general fallback chain
		d, p, fbErr := h.Router.GeneralFallback(req.Model)
		if fbErr == nil {
			log.Printf("fallback: %s → %s", req.Model, d.ModelName)
			return p, d.APIKey(), d.ModelName, nil
		}

		return nil, "", "", err
	}

	// Direct resolution (single deployment)
	return h.resolveProviderFromConfig(req.Model)
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

func (h *Handlers) handleNonStreamingCompletion(w http.ResponseWriter, r *http.Request, p provider.Provider, req *model.ChatCompletionRequest, apiKey string) {
	startTime := time.Now()

	// Pre-call cache check
	if h.Cache != nil {
		key := cacheKey(req.Model, req.Messages)
		if cached, err := h.Cache.Get(r.Context(), key); err == nil && len(cached) > 0 {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-Cache", "HIT")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(cached)
			return
		}
	}

	httpReq, err := p.TransformRequest(r.Context(), req, apiKey)
	if err != nil {
		h.logFailure(r.Context(), req, p, startTime, fmt.Errorf("transform request: %w", err))
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{
				Message: "transform request: " + err.Error(),
				Type:    "internal_error",
			},
		})
		return
	}

	llmStart := time.Now()
	resp, err := http.DefaultClient.Do(httpReq)
	llmLatency := time.Since(llmStart)
	if err != nil {
		h.logFailure(r.Context(), req, p, startTime, fmt.Errorf("upstream request failed: %w", err))
		writeJSON(w, http.StatusBadGateway, model.ErrorResponse{
			Error: model.ErrorDetail{
				Message: "upstream request failed: " + err.Error(),
				Type:    "internal_error",
			},
		})
		return
	}

	result, err := p.TransformResponse(r.Context(), resp)
	if err != nil {
		h.logFailure(r.Context(), req, p, startTime, fmt.Errorf("transform response: %w", err))
		writeJSON(w, http.StatusBadGateway, model.ErrorResponse{
			Error: model.ErrorDetail{
				Message: "transform response: " + err.Error(),
				Type:    "internal_error",
			},
		})
		return
	}

	endTime := time.Now()
	h.logSuccess(r.Context(), req, result, p, startTime, endTime, llmLatency)

	// Post-call cache store
	if h.Cache != nil {
		key := cacheKey(req.Model, req.Messages)
		if data, err := json.Marshal(result); err == nil {
			_ = h.Cache.Set(r.Context(), key, data, defaultCacheTTL)
		}
	}

	writeJSON(w, http.StatusOK, result)
}

func (h *Handlers) handleStreamingCompletion(w http.ResponseWriter, r *http.Request, p provider.Provider, req *model.ChatCompletionRequest, apiKey string) {
	startTime := time.Now()

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

	httpReq, err := p.TransformRequest(r.Context(), req, apiKey)
	if err != nil {
		h.logFailure(r.Context(), req, p, startTime, fmt.Errorf("transform request: %w", err))
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{
				Message: "transform request: " + err.Error(),
				Type:    "internal_error",
			},
		})
		return
	}

	llmStart := time.Now()
	resp, err := http.DefaultClient.Do(httpReq)
	llmLatency := time.Since(llmStart)
	if err != nil {
		h.logFailure(r.Context(), req, p, startTime, fmt.Errorf("upstream request failed: %w", err))
		writeJSON(w, http.StatusBadGateway, model.ErrorResponse{
			Error: model.ErrorDetail{
				Message: "upstream request failed: " + err.Error(),
				Type:    "internal_error",
			},
		})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		h.logFailure(r.Context(), req, p, startTime, fmt.Errorf("upstream error: status %d", resp.StatusCode))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(resp.StatusCode)
		_, _ = w.Write(body)
		return
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	var lastChunk *model.StreamChunk
	var assembledContent strings.Builder
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()

		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")

		chunk, done, err := p.TransformStreamChunk(r.Context(), []byte(data))
		if err != nil {
			log.Printf("stream chunk error: %v", err)
			continue
		}

		if done {
			fmt.Fprintf(w, "data: [DONE]\n\n")
			flusher.Flush()
			endTime := time.Now()
			h.logStreamSuccess(r.Context(), req, lastChunk, p, startTime, endTime, llmLatency)
			// Cache assembled streaming response
			h.cacheStreamResult(r.Context(), req, lastChunk, assembledContent.String())
			return
		}

		if chunk != nil {
			lastChunk = chunk
			// Accumulate content for caching
			if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != nil {
				assembledContent.WriteString(*chunk.Choices[0].Delta.Content)
			}
			chunkData, err := json.Marshal(chunk)
			if err != nil {
				log.Printf("marshal chunk error: %v", err)
				continue
			}
			fmt.Fprintf(w, "data: %s\n\n", chunkData)
			flusher.Flush()
		}
	}

	// Stream ended without [DONE] — still log
	endTime := time.Now()
	h.logStreamSuccess(r.Context(), req, lastChunk, p, startTime, endTime, llmLatency)
}

// cacheStreamResult assembles a non-streaming response from accumulated stream data and caches it.
func (h *Handlers) cacheStreamResult(ctx context.Context, req *model.ChatCompletionRequest, lastChunk *model.StreamChunk, content string) {
	if h.Cache == nil || content == "" {
		return
	}
	// Build a synthetic ModelResponse from the stream content + usage from final chunk
	stopReason := "stop"
	assembled := &model.ModelResponse{
		ID:      "",
		Object:  "chat.completion",
		Model:   req.Model,
		Choices: []model.Choice{{Message: &model.Message{Role: "assistant", Content: content}, Index: 0, FinishReason: &stopReason}},
	}
	if lastChunk != nil {
		assembled.ID = lastChunk.ID
		if lastChunk.Usage != nil {
			assembled.Usage = *lastChunk.Usage
		}
	}
	key := cacheKey(req.Model, req.Messages)
	if data, err := json.Marshal(assembled); err == nil {
		_ = h.Cache.Set(ctx, key, data, defaultCacheTTL)
	}
}

// buildLogData constructs the common callback.LogData from request context.
func (h *Handlers) buildLogData(ctx context.Context, req *model.ChatCompletionRequest, p provider.Provider, startTime time.Time) callback.LogData {
	providerName := ""
	if p != nil {
		providerName = fmt.Sprintf("%T", p)
		// Strip package prefix: *openai.Provider → openai
		if idx := strings.LastIndex(providerName, "."); idx >= 0 {
			providerName = providerName[:idx]
		}
		if idx := strings.LastIndex(providerName, "/"); idx >= 0 {
			providerName = providerName[idx+1:]
		}
		providerName = strings.TrimPrefix(providerName, "*")
	}

	data := callback.LogData{
		Model:     req.Model,
		Provider:  providerName,
		Request:   req,
		StartTime: startTime,
	}

	if userID, ok := ctx.Value(middleware.ContextKeyUserID).(string); ok {
		data.UserID = userID
	}
	if teamID, ok := ctx.Value(middleware.ContextKeyTeamID).(string); ok {
		data.TeamID = teamID
	}

	return data
}

// logSuccess fires success callbacks for non-streaming responses.
func (h *Handlers) logSuccess(ctx context.Context, req *model.ChatCompletionRequest, result *model.ModelResponse, p provider.Provider, startTime, endTime time.Time, llmLatency time.Duration) {
	if h.Callbacks == nil {
		return
	}

	data := h.buildLogData(ctx, req, p, startTime)
	data.Response = result
	data.EndTime = endTime
	data.Latency = endTime.Sub(startTime)
	data.LLMAPILatency = llmLatency

	if result != nil {
		data.PromptTokens = result.Usage.PromptTokens
		data.CompletionTokens = result.Usage.CompletionTokens
		data.TotalTokens = result.Usage.TotalTokens
		data.Cost = pricing.Default().TotalCost(req.Model, result.Usage.PromptTokens, result.Usage.CompletionTokens)
	}

	go h.Callbacks.LogSuccess(data)
}

// logStreamSuccess fires success callbacks for streaming responses.
func (h *Handlers) logStreamSuccess(ctx context.Context, req *model.ChatCompletionRequest, lastChunk *model.StreamChunk, p provider.Provider, startTime, endTime time.Time, llmLatency time.Duration) {
	if h.Callbacks == nil {
		return
	}

	data := h.buildLogData(ctx, req, p, startTime)
	data.EndTime = endTime
	data.Latency = endTime.Sub(startTime)
	data.LLMAPILatency = llmLatency

	if lastChunk != nil && lastChunk.Usage != nil {
		data.PromptTokens = lastChunk.Usage.PromptTokens
		data.CompletionTokens = lastChunk.Usage.CompletionTokens
		data.TotalTokens = lastChunk.Usage.TotalTokens
		data.Cost = pricing.Default().TotalCost(req.Model, lastChunk.Usage.PromptTokens, lastChunk.Usage.CompletionTokens)
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

	apiKeyHash := ""
	if v, ok := ctx.Value(middleware.ContextKeyTokenHash).(string); ok {
		apiKeyHash = v
	}

	modelName := ""
	if req != nil {
		modelName = req.Model
	}

	go func() {
		_ = h.DB.InsertErrorLog(context.Background(), db.InsertErrorLogParams{
			RequestID:    fmt.Sprintf("%p", ctx),
			ApiKeyHash:   apiKeyHash,
			Model:        modelName,
			Provider:     providerName,
			StatusCode:   500,
			ErrorType:    "provider_error",
			ErrorMessage: err.Error(),
		})
	}()
}
