package handler

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/praxisllmlab/tianjiLLM/internal/callback"
	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/praxisllmlab/tianjiLLM/internal/model"
	"github.com/praxisllmlab/tianjiLLM/internal/pricing"
	"github.com/praxisllmlab/tianjiLLM/internal/provider/chatgptcodex"
	"github.com/praxisllmlab/tianjiLLM/internal/proxy/middleware"
)

// ImageGeneration handles POST /v1/images/generations.
func (h *Handlers) ImageGeneration(w http.ResponseWriter, r *http.Request) {
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
	var req model.ImageGenerationRequest
	if err = json.Unmarshal(body, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{
				Message: "invalid request body: " + err.Error(),
				Type:    "invalid_request_error",
			},
		})
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
		if contentTypeErr := validateJSONRequestContentType(r); contentTypeErr != nil {
			writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
				Error: model.ErrorDetail{Message: contentTypeErr.Error(), Type: "invalid_request_error"},
			})
			return
		}
		codexReq := req
		codexReq.Model = route.ModelName
		h.handleChatGPTCodexImageGeneration(w, r, &codexReq, route.TianjiParams, route.SubscriptionCandidates)
		return
	}

	url := route.Provider.GetRequestURL(req.Model)
	url = url[:len(url)-len("/chat/completions")] + "/images/generations"

	middleware.LogProviderResolved(r.Context(), h.lookupProviderName(req.Model), url, "image_generation", req.Model)

	resp, upstreamLatencyDuration, subscriptionAttribution, err := h.doOpenAISubscriptionProviderRequest(r.Context(), route.APIKey, route.SubscriptionCandidates, func(attempt openAISubscriptionProviderAttempt) (*http.Request, error) {
		httpReq, buildErr := http.NewRequestWithContext(r.Context(), http.MethodPost, url, bytes.NewReader(body))
		if buildErr != nil {
			return nil, buildErr
		}
		route.Provider.SetupHeaders(httpReq, attempt.apiKey)
		httpReq.Header.Set("Content-Type", "application/json")
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

	middleware.LogUpstreamResponded(r.Context(), middleware.UpstreamResult{
		StatusCode: resp.StatusCode,
		LatencyMs:  upstreamLatency,
	})

	respBody := copyResponseBody(w, resp)
	if respBody == nil {
		return
	}

	// Image responses have no usage field — record request for duration/cost tracking
	if resp.StatusCode >= 200 && resp.StatusCode < 300 && h.Callbacks != nil {
		endTime := time.Now()
		data := buildBaseLogData(r.Context(), startTime)
		data.Model = req.Model
		data.Provider = h.lookupProviderName(req.Model)
		data.CallType = "image_generation"
		data.EndTime = endTime
		data.Latency = endTime.Sub(startTime)
		data.RequestPayload = imageGenerationRequestAuditPayload(&req, h.lookupProviderName(req.Model), nil)
		data.ResponsePayload = imageGenerationResponseAuditPayload(respBody)
		applyOpenAISubscriptionLogAttribution(&data, openAISubscriptionAttributionForAction(subscriptionAttribution, "image_generation"))
		go h.Callbacks.LogSuccess(data)
	}
}

func (h *Handlers) handleImagesEdit(w http.ResponseWriter, r *http.Request) {
	contentType, contentTypeErr := singleRequestContentType(r)
	if contentTypeErr != nil {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: contentTypeErr.Error(), Type: "invalid_request_error"},
		})
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "read request body: " + err.Error(), Type: "invalid_request_error"},
		})
		return
	}

	modelName, err := imageEditRequestModel(body, contentType)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: err.Error(), Type: "invalid_request_error"},
		})
		return
	}

	route, err := h.resolveProviderFromConfigRouteWithContext(r.Context(), modelName)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: err.Error(), Type: "invalid_request_error"},
		})
		return
	}
	if !route.ChatGPTCodexBackend {
		r.Body = io.NopCloser(bytes.NewReader(body))
		h.openAIEndpointProxy(w, r)
		return
	}

	req, err := parseImageEditRequest(body, contentType)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: err.Error(), Type: "invalid_request_error"},
		})
		return
	}
	req.Model = route.ModelName
	h.handleChatGPTCodexImageEdit(w, r, req, route.TianjiParams, route.SubscriptionCandidates)
}

func (h *Handlers) handleChatGPTCodexImageGeneration(w http.ResponseWriter, r *http.Request, req *model.ImageGenerationRequest, params config.TianjiParams, candidates []resolvedOpenAISubscriptionCredential) {
	startTime := time.Now()
	if len(candidates) == 0 {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "OpenAI subscription credential resolution failed: no usable credential candidates", Type: "invalid_request_error"},
		})
		return
	}

	count := 1
	if req.N != nil {
		if *req.N != 1 {
			writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
				Error: model.ErrorDetail{
					Message: "n must be 1 for ChatGPT Codex image generation",
					Type:    "invalid_request_error",
					Param:   "n",
				},
			})
			return
		}
		count = *req.N
	}
	transport := h.chatGPTCodexBackendTransportForParams(r.Context(), params, req.Model, candidates)
	upstreamRequestPayload, err := chatgptcodex.BuildImageGenerationPayload(req)
	if err != nil {
		writeChatGPTCodexBackendPreparationError(w, err)
		return
	}
	preparedPayload, err := transport.PrepareResponsesPayload(upstreamRequestPayload, true)
	if err != nil {
		writeChatGPTCodexBackendPreparationError(w, err)
		return
	}
	upstreamPayload := preparedPayload.Snapshot()
	candidates = h.orderOpenAISubscriptionCodexCandidates(r.Context(), params, candidates)
	defer h.refreshOpenAISubscriptionCodexUsageAfterResponse(r.Context(), candidates)
	result := model.ImageGenerationResponse{Created: time.Now().Unix()}
	var totalLatency time.Duration
	var totalUsage model.Usage
	var subscriptionAttribution *callback.OpenAISubscriptionAttribution

	for i := 0; i < count; i++ {
		resp, llmLatency, attribution, err := h.doOpenAISubscriptionProviderRequest(r.Context(), "", candidates, func(attempt openAISubscriptionProviderAttempt) (*http.Request, error) {
			return transport.BuildImageGenerationRequest(r.Context(), preparedPayload, attempt.apiKey, attempt.accountID, r.Header)
		})
		totalLatency += llmLatency
		if attribution != nil {
			subscriptionAttribution = attribution
		}
		if err != nil {
			var clientErr *model.TianjiError
			if errors.As(err, &clientErr) && errors.Is(clientErr.Err, model.ErrClientValidation) {
				writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
					Error: model.ErrorDetail{
						Message: clientErr.Message,
						Type:    clientErr.Type,
					},
				})
				return
			}
			middleware.LogUpstreamResponded(r.Context(), middleware.UpstreamResult{
				LatencyMs: float64(totalLatency.Milliseconds()),
				Error:     err.Error(),
			})
			writeJSON(w, http.StatusBadGateway, model.ErrorResponse{
				Error: model.ErrorDetail{Message: "upstream request failed: " + err.Error(), Type: "internal_error"},
			})
			return
		}
		middleware.LogUpstreamResponded(r.Context(), middleware.UpstreamResult{
			StatusCode: resp.StatusCode,
			LatencyMs:  float64(totalLatency.Milliseconds()),
		})
		if resp.StatusCode != http.StatusOK {
			parseErr := chatgptcodex.ParseErrorResponse(resp)
			_ = resp.Body.Close()
			writeTransformError(w, parseErr, req.Model)
			return
		}
		streamResult, err := chatgptcodex.TransformImageGenerationStream(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			writeTransformError(w, err, req.Model)
			return
		}
		result.Data = append(result.Data, streamResult.Images...)
		totalUsage.PromptTokens += streamResult.Usage.PromptTokens
		totalUsage.CompletionTokens += streamResult.Usage.CompletionTokens
		totalUsage.TotalTokens += streamResult.Usage.TotalTokens
	}

	if h.Callbacks != nil {
		endTime := time.Now()
		data := buildBaseLogData(r.Context(), startTime)
		data.Model = req.Model
		data.Provider = "chatgpt_codex_backend"
		data.CallType = "image_generation"
		data.EndTime = endTime
		data.Latency = endTime.Sub(startTime)
		data.LLMAPILatency = totalLatency
		data.PromptTokens = totalUsage.PromptTokens
		data.CompletionTokens = totalUsage.CompletionTokens
		data.TotalTokens = totalUsage.TotalTokens
		data.Cost = pricing.Default().TotalCost(req.Model, pricing.TokenUsage{
			PromptTokens:     totalUsage.PromptTokens,
			CompletionTokens: totalUsage.CompletionTokens,
		})
		data.RequestPayload = imageGenerationRequestAuditPayload(req, "chatgpt_codex_backend", upstreamPayload)
		data.ResponsePayload = imageGenerationResponseAuditPayloadFromResult(result)
		applyOpenAISubscriptionLogAttribution(&data, openAISubscriptionAttributionForAction(subscriptionAttribution, "image_generation"))
		go h.Callbacks.LogSuccess(data)
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handlers) handleChatGPTCodexImageEdit(w http.ResponseWriter, r *http.Request, req *model.ImageEditRequest, params config.TianjiParams, candidates []resolvedOpenAISubscriptionCredential) {
	startTime := time.Now()
	if len(candidates) == 0 {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "OpenAI subscription credential resolution failed: no usable credential candidates", Type: "invalid_request_error"},
		})
		return
	}

	count := 1
	if req.N != nil {
		if *req.N != 1 {
			writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
				Error: model.ErrorDetail{
					Message: "n must be 1 for ChatGPT Codex image edit",
					Type:    "invalid_request_error",
					Param:   "n",
				},
			})
			return
		}
		count = *req.N
	}
	transport := h.chatGPTCodexBackendTransportForParams(r.Context(), params, req.Model, candidates)
	upstreamPayload, err := chatgptcodex.BuildImageEditPayload(req)
	if err != nil {
		writeChatGPTCodexBackendPreparationError(w, err)
		return
	}
	preparedPayload, err := transport.PrepareResponsesPayload(upstreamPayload, true)
	if err != nil {
		writeChatGPTCodexBackendPreparationError(w, err)
		return
	}
	candidates = h.orderOpenAISubscriptionCodexCandidates(r.Context(), params, candidates)
	defer h.refreshOpenAISubscriptionCodexUsageAfterResponse(r.Context(), candidates)

	result := model.ImageGenerationResponse{Created: time.Now().Unix()}
	var totalLatency time.Duration
	var totalUsage model.Usage
	var subscriptionAttribution *callback.OpenAISubscriptionAttribution
	for i := 0; i < count; i++ {
		resp, llmLatency, attribution, err := h.doOpenAISubscriptionProviderRequest(r.Context(), "", candidates, func(attempt openAISubscriptionProviderAttempt) (*http.Request, error) {
			return transport.BuildPreparedResponsesRequest(r.Context(), preparedPayload, attempt.apiKey, attempt.accountID, r.Header)
		})
		totalLatency += llmLatency
		if attribution != nil {
			subscriptionAttribution = attribution
		}
		if err != nil {
			var clientErr *model.TianjiError
			if errors.As(err, &clientErr) && errors.Is(clientErr.Err, model.ErrClientValidation) {
				writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
					Error: model.ErrorDetail{Message: clientErr.Message, Type: clientErr.Type},
				})
				return
			}
			middleware.LogUpstreamResponded(r.Context(), middleware.UpstreamResult{
				LatencyMs: float64(totalLatency.Milliseconds()),
				Error:     err.Error(),
			})
			writeJSON(w, http.StatusBadGateway, model.ErrorResponse{
				Error: model.ErrorDetail{Message: "upstream request failed: " + err.Error(), Type: "internal_error"},
			})
			return
		}
		middleware.LogUpstreamResponded(r.Context(), middleware.UpstreamResult{
			StatusCode: resp.StatusCode,
			LatencyMs:  float64(totalLatency.Milliseconds()),
		})
		if resp.StatusCode != http.StatusOK {
			parseErr := chatgptcodex.ParseErrorResponse(resp)
			_ = resp.Body.Close()
			writeTransformError(w, parseErr, req.Model)
			return
		}
		streamResult, err := chatgptcodex.TransformImageGenerationStream(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			writeTransformError(w, err, req.Model)
			return
		}
		result.Data = append(result.Data, streamResult.Images...)
		totalUsage.PromptTokens += streamResult.Usage.PromptTokens
		totalUsage.CompletionTokens += streamResult.Usage.CompletionTokens
		totalUsage.TotalTokens += streamResult.Usage.TotalTokens
	}

	if h.Callbacks != nil {
		endTime := time.Now()
		data := buildBaseLogData(r.Context(), startTime)
		data.Model = req.Model
		data.Provider = "chatgpt_codex_backend"
		data.CallType = "image_edit"
		data.EndTime = endTime
		data.Latency = endTime.Sub(startTime)
		data.LLMAPILatency = totalLatency
		data.PromptTokens = totalUsage.PromptTokens
		data.CompletionTokens = totalUsage.CompletionTokens
		data.TotalTokens = totalUsage.TotalTokens
		data.Cost = pricing.Default().TotalCost(req.Model, pricing.TokenUsage{
			PromptTokens:     totalUsage.PromptTokens,
			CompletionTokens: totalUsage.CompletionTokens,
		})
		data.RequestPayload = imageEditRequestAuditPayload(req, "chatgpt_codex_backend", redactImageInputDataURLs(preparedPayload.Snapshot()))
		data.ResponsePayload = imageGenerationResponseAuditPayloadFromResult(result)
		applyOpenAISubscriptionLogAttribution(&data, openAISubscriptionAttributionForAction(subscriptionAttribution, "image_edit"))
		go h.Callbacks.LogSuccess(data)
	}
	writeJSON(w, http.StatusOK, result)
}

func imageGenerationRequestAuditPayload(req *model.ImageGenerationRequest, provider string, upstreamPayload any) map[string]any {
	payload := map[string]any{
		"type":     "image_generation",
		"provider": provider,
		"request": map[string]any{
			"model":              req.Model,
			"prompt":             req.Prompt,
			"n":                  req.N,
			"size":               req.Size,
			"quality":            req.Quality,
			"output_format":      req.OutputFormat,
			"background":         req.Background,
			"moderation":         req.Moderation,
			"output_compression": req.OutputCompression,
			"response_format":    req.ResponseFormat,
			"style":              req.Style,
			"user":               req.User,
		},
	}
	if upstreamPayload != nil {
		payload["upstream_payload"] = upstreamPayload
	}
	return payload
}

func imageEditRequestAuditPayload(req *model.ImageEditRequest, provider string, upstreamPayload any) map[string]any {
	payload := map[string]any{
		"type":     "image_edit",
		"provider": provider,
		"request": map[string]any{
			"model":              req.Model,
			"prompt":             req.Prompt,
			"image_count":        len(req.Images),
			"mask":               req.Mask != nil,
			"n":                  req.N,
			"size":               req.Size,
			"quality":            req.Quality,
			"output_format":      req.OutputFormat,
			"background":         req.Background,
			"output_compression": req.OutputCompression,
			"response_format":    req.ResponseFormat,
			"user":               req.User,
		},
	}
	if upstreamPayload != nil {
		payload["upstream_payload"] = upstreamPayload
	}
	return payload
}

func redactImageInputDataURLs(value any) any {
	switch v := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(v))
		for key, item := range v {
			if key == "image_url" {
				switch imageURL := item.(type) {
				case map[string]any:
					out[key] = redactImageURLMap(imageURL)
					continue
				case string:
					out[key] = redactImageURLString(imageURL)
					continue
				}
			}
			out[key] = redactImageInputDataURLs(item)
		}
		return out
	case []map[string]any:
		out := make([]map[string]any, len(v))
		for i, item := range v {
			redacted, _ := redactImageInputDataURLs(item).(map[string]any)
			out[i] = redacted
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = redactImageInputDataURLs(item)
		}
		return out
	default:
		return value
	}
}

func redactImageURLString(urlValue string) any {
	if !strings.HasPrefix(urlValue, "data:") {
		return urlValue
	}
	sum := sha256.Sum256([]byte(urlValue))
	return map[string]any{
		"redacted": true,
		"len":      len(urlValue),
		"sha256":   hex.EncodeToString(sum[:]),
	}
}

func redactImageURLMap(imageURL map[string]any) map[string]any {
	out := make(map[string]any, len(imageURL))
	for key, value := range imageURL {
		if key != "url" {
			out[key] = redactImageInputDataURLs(value)
			continue
		}
		urlValue, ok := value.(string)
		if !ok || !strings.HasPrefix(urlValue, "data:") {
			out[key] = value
			continue
		}
		out[key] = redactImageURLString(urlValue)
	}
	return out
}

func imageGenerationResponseAuditPayload(raw []byte) map[string]any {
	var result model.ImageGenerationResponse
	if err := json.Unmarshal(raw, &result); err != nil {
		return map[string]any{
			"type":        "image_generation_response",
			"parse_error": err.Error(),
			"raw_len":     len(raw),
		}
	}
	return imageGenerationResponseAuditPayloadFromResult(result)
}

func imageGenerationResponseAuditPayloadFromResult(result model.ImageGenerationResponse) map[string]any {
	data := make([]map[string]any, 0, len(result.Data))
	for _, image := range result.Data {
		item := map[string]any{}
		if image.URL != "" {
			item["url"] = image.URL
		}
		if image.B64JSON != "" {
			sum := sha256.Sum256([]byte(image.B64JSON))
			item["b64_json_len"] = len(image.B64JSON)
			item["b64_json_sha256"] = hex.EncodeToString(sum[:])
		}
		if image.RevisedPrompt != "" {
			item["revised_prompt"] = image.RevisedPrompt
		}
		data = append(data, item)
	}
	return map[string]any{
		"type":     "image_generation_response",
		"created":  result.Created,
		"data_len": len(result.Data),
		"data":     data,
	}
}

func imageEditRequestModel(body []byte, contentType string) (string, error) {
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil || mediaType != "multipart/form-data" {
		return "", errors.New("image edit request must use multipart/form-data")
	}
	boundary := params["boundary"]
	if strings.TrimSpace(boundary) == "" {
		return "", errors.New("multipart/form-data boundary is required")
	}
	form, err := multipart.NewReader(bytes.NewReader(body), boundary).ReadForm(32 << 20)
	if err != nil {
		return "", err
	}
	defer func() { _ = form.RemoveAll() }()
	modelName := firstFormValue(form, "model")
	if strings.TrimSpace(modelName) == "" {
		return "", errors.New("model is required")
	}
	return modelName, nil
}

func parseImageEditRequest(body []byte, contentType string) (*model.ImageEditRequest, error) {
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil || mediaType != "multipart/form-data" {
		return nil, errors.New("image edit request must use multipart/form-data")
	}
	boundary := params["boundary"]
	if strings.TrimSpace(boundary) == "" {
		return nil, errors.New("multipart/form-data boundary is required")
	}
	form, err := multipart.NewReader(bytes.NewReader(body), boundary).ReadForm(64 << 20)
	if err != nil {
		return nil, err
	}
	defer func() { _ = form.RemoveAll() }()

	n, err := intFormValue(form, "n")
	if err != nil {
		return nil, err
	}
	outputCompression, err := intFormValue(form, "output_compression")
	if err != nil {
		return nil, err
	}
	req := &model.ImageEditRequest{
		Model:             firstFormValue(form, "model"),
		Prompt:            firstFormValue(form, "prompt"),
		N:                 n,
		Size:              stringPtrFormValue(form, "size"),
		Quality:           stringPtrFormValue(form, "quality"),
		OutputFormat:      stringPtrFormValue(form, "output_format"),
		Background:        stringPtrFormValue(form, "background"),
		OutputCompression: outputCompression,
		ResponseFormat:    stringPtrFormValue(form, "response_format"),
		User:              stringPtrFormValue(form, "user"),
	}
	if strings.TrimSpace(req.Model) == "" {
		return nil, errors.New("model is required")
	}
	if strings.TrimSpace(req.Prompt) == "" {
		return nil, errors.New("prompt is required")
	}
	for _, fileHeader := range imageEditImageFileHeaders(form) {
		file, err := readImageEditFile(fileHeader)
		if err != nil {
			return nil, err
		}
		req.Images = append(req.Images, file)
	}
	if maskHeaders := form.File["mask"]; len(maskHeaders) > 0 {
		mask, err := readImageEditFile(maskHeaders[0])
		if err != nil {
			return nil, err
		}
		req.Mask = &mask
	}
	req.ExtraParams = unknownImageEditValues(form.Value)
	return req, nil
}

func firstFormValue(form *multipart.Form, key string) string {
	if form == nil {
		return ""
	}
	if values := form.Value[key]; len(values) > 0 {
		return values[0]
	}
	return ""
}

func stringPtrFormValue(form *multipart.Form, key string) *string {
	value := firstFormValue(form, key)
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return &value
}

func intFormValue(form *multipart.Form, key string) (*int, error) {
	value := strings.TrimSpace(firstFormValue(form, key))
	if value == "" {
		return nil, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return nil, fmt.Errorf("%s must be an integer", key)
	}
	return &parsed, nil
}

func imageEditImageFileHeaders(form *multipart.Form) []*multipart.FileHeader {
	if form == nil {
		return nil
	}
	headers := append([]*multipart.FileHeader{}, form.File["image"]...)
	headers = append(headers, form.File["image[]"]...)
	return headers
}

func readImageEditFile(header *multipart.FileHeader) (model.ImageEditFile, error) {
	file, err := header.Open()
	if err != nil {
		return model.ImageEditFile{}, err
	}
	defer func() { _ = file.Close() }()
	data, err := io.ReadAll(file)
	if err != nil {
		return model.ImageEditFile{}, err
	}
	contentType := header.Header.Get("Content-Type")
	if strings.TrimSpace(contentType) == "" {
		contentType = http.DetectContentType(data)
	}
	return model.ImageEditFile{
		Filename:    header.Filename,
		ContentType: contentType,
		Data:        data,
	}, nil
}

func unknownImageEditValues(values map[string][]string) map[string][]string {
	known := map[string]bool{
		"model": true, "prompt": true, "n": true, "size": true,
		"quality": true, "output_format": true, "background": true,
		"output_compression": true, "response_format": true, "user": true,
	}
	var extra map[string][]string
	for key, value := range values {
		if known[key] {
			continue
		}
		if extra == nil {
			extra = make(map[string][]string)
		}
		extra[key] = append([]string(nil), value...)
	}
	return extra
}
