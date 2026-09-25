package handler

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/praxisllmlab/tianjiLLM/internal/model"
	"github.com/praxisllmlab/tianjiLLM/internal/provider"
	"github.com/praxisllmlab/tianjiLLM/internal/proxy/middleware"
	"github.com/rs/zerolog"
)

// Embedding handles POST /v1/embeddings.
func (h *Handlers) Embedding(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()

	var req model.EmbeddingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeRequestError(w, model.InvalidRequest("", "Invalid request body"))
		return
	}
	if strings.TrimSpace(req.Model) == "" {
		writeRequestError(w, model.InvalidValue("model", "model must be a non-empty string"))
		return
	}

	originalModel := req.Model
	clientEncodingFormat := req.EncodingFormat
	if h.requestUsesOfficialOpenAIModel(originalModel) {
		writeUnsupportedChatGPTCodexEndpoint(w, "embeddings")
		return
	}
	route, err := h.resolveProviderFromConfigRouteWithContext(r.Context(), originalModel)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeRequestError(w, model.ModelNotFound(originalModel))
		} else {
			writeRequestError(w, model.InvalidRequest("model", "Unable to resolve requested model"))
		}
		return
	}
	if route.ChatGPTCodexBackend {
		writeUnsupportedChatGPTCodexEndpoint(w, "embeddings")
		return
	}
	if requestErr := validateEmbeddingCapabilities(&req, route.Capability); requestErr != nil {
		writeRequestError(w, requestErr)
		return
	}

	embProvider, ok := route.Provider.(provider.EmbeddingProvider)
	if !ok {
		writeRequestError(w, model.UnsupportedParameter("model", "This model does not support embeddings"))
		return
	}

	// Log warnings for truly unknown parameters (not declared in GetSupportedParams).
	// Known provider-specific params (e.g. Jina's task/normalized) are intentional pass-through.
	if len(req.ExtraParams) > 0 {
		supportedSet := make(map[string]bool)
		for _, k := range route.Provider.GetSupportedParams() {
			supportedSet[k] = true
		}
		var unknownKeys []string
		for k := range req.ExtraParams {
			if !supportedSet[k] {
				unknownKeys = append(unknownKeys, k)
			}
		}
		if len(unknownKeys) > 0 {
			zerolog.Ctx(r.Context()).Warn().Strs("unknown_params", unknownKeys).Msg("unknown parameters forwarded to upstream")
		}
	}

	// Phase 2: provider.resolved
	middleware.LogProviderResolved(r.Context(), h.lookupProviderName(originalModel), route.Provider.GetRequestURL(route.ModelName), "embedding", route.ModelName)

	req.Model = route.ModelName
	if clientEncodingFormat == "base64" {
		req.EncodingFormat = "float"
	}

	resp, upstreamLatencyDuration, subscriptionAttribution, err := h.doOpenAISubscriptionProviderRequest(r.Context(), route.APIKey, route.SubscriptionCandidates, func(attempt openAISubscriptionProviderAttempt) (*http.Request, error) {
		return embProvider.TransformEmbeddingRequest(r.Context(), &req, attempt.apiKey)
	})
	upstreamLatency := float64(upstreamLatencyDuration.Milliseconds())
	if err != nil {
		middleware.LogUpstreamResponded(r.Context(), middleware.UpstreamResult{
			LatencyMs: upstreamLatency,
			Error:     err.Error(),
		})
		writeEmbeddingError(w, err, originalModel)
		return
	}

	// Phase 3: upstream.responded
	middleware.LogUpstreamResponded(r.Context(), middleware.UpstreamResult{
		StatusCode: resp.StatusCode,
		LatencyMs:  upstreamLatency,
	})

	result, err := embProvider.TransformEmbeddingResponse(r.Context(), resp)
	if err != nil {
		zerolog.Ctx(r.Context()).Error().Err(err).Str("model", route.ModelName).Msg("embedding: TransformEmbeddingResponse failed")
		writeEmbeddingError(w, err, originalModel)
		return
	}
	if err := validateEmbeddingResult(result, embeddingInputCount(req.Input), req.Dimensions, clientEncodingFormat); err != nil {
		zerolog.Ctx(r.Context()).Error().Err(err).Str("model", route.ModelName).Msg("embedding: invalid upstream response")
		writeEmbeddingError(w, err, originalModel)
		return
	}
	result.Model = originalModel

	endTime := time.Now()
	if h.Callbacks != nil {
		data := buildBaseLogData(r.Context(), startTime)
		data.Model = route.ModelName
		data.Provider = h.lookupProviderName(originalModel)
		data.CallType = "embedding"
		data.EndTime = endTime
		data.Latency = endTime.Sub(startTime)
		applyOpenAISubscriptionLogAttribution(&data, openAISubscriptionAttributionForAction(subscriptionAttribution, "embedding"))
		data.PromptTokens = result.Usage.PromptTokens
		data.TotalTokens = result.Usage.TotalTokens
		go h.Callbacks.LogSuccess(data)
	}

	writeEmbeddingResult(w, result, clientEncodingFormat)
}

func embeddingInputCount(input any) int {
	switch value := input.(type) {
	case string:
		return 1
	case []string:
		return len(value)
	default:
		return 0
	}
}

func validateEmbeddingResult(result *model.EmbeddingResponse, expected int, dimensions *int, encodingFormat string) error {
	if result == nil {
		return fmt.Errorf("embedding response is nil")
	}
	if len(result.Data) != expected {
		return fmt.Errorf("embedding response contains %d items; expected %d", len(result.Data), expected)
	}
	if dimensions != nil {
		for i, item := range result.Data {
			if len(item.Embedding) != *dimensions {
				return fmt.Errorf(
					"embedding response data[%d] has %d dimensions; expected %d",
					i,
					len(item.Embedding),
					*dimensions,
				)
			}
		}
	}
	if encodingFormat == "base64" {
		for i, item := range result.Data {
			for j, value := range item.Embedding {
				converted := float32(value)
				if math.IsNaN(float64(converted)) || math.IsInf(float64(converted), 0) {
					return fmt.Errorf("embedding response data[%d].embedding[%d] cannot be represented as float32", i, j)
				}
			}
		}
	}
	return nil
}

func writeEmbeddingResult(w http.ResponseWriter, result *model.EmbeddingResponse, encodingFormat string) {
	if encodingFormat != "base64" {
		writeJSON(w, http.StatusOK, result)
		return
	}

	data := make([]base64EmbeddingData, len(result.Data))
	for i, item := range result.Data {
		data[i] = base64EmbeddingData{
			Object:    item.Object,
			Index:     item.Index,
			Embedding: encodeEmbeddingVector(item.Embedding),
		}
	}
	writeJSON(w, http.StatusOK, base64EmbeddingResponse{
		Object: result.Object,
		Data:   data,
		Model:  result.Model,
		Usage:  result.Usage,
	})
}

type base64EmbeddingResponse struct {
	Object string                `json:"object"`
	Data   []base64EmbeddingData `json:"data"`
	Model  string                `json:"model"`
	Usage  model.EmbeddingUsage  `json:"usage"`
}

type base64EmbeddingData struct {
	Object    string `json:"object"`
	Index     int    `json:"index"`
	Embedding string `json:"embedding"`
}

func encodeEmbeddingVector(vector []float64) string {
	raw := make([]byte, len(vector)*4)
	for i, value := range vector {
		binary.LittleEndian.PutUint32(raw[i*4:], math.Float32bits(float32(value)))
	}
	return base64.StdEncoding.EncodeToString(raw)
}
