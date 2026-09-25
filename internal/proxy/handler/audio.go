package handler

import (
	"bytes"
	"encoding/json"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/praxisllmlab/tianjiLLM/internal/model"
	"github.com/praxisllmlab/tianjiLLM/internal/proxy/middleware"
)

// AudioTranscription handles POST /v1/audio/transcriptions.
// This is a multipart form upload — we proxy the raw request body.
func (h *Handlers) AudioTranscription(w http.ResponseWriter, r *http.Request) {
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

	modelName := audioTranscriptionModelName(r.Header.Get("Content-Type"), body)
	if modelName == "" {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{
				Message: "model is required",
				Type:    "invalid_request_error",
			},
		})
		return
	}

	if h.requestUsesOfficialOpenAIModel(modelName) {
		writeUnsupportedChatGPTCodexEndpoint(w, "audio transcription")
		return
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
		writeUnsupportedChatGPTCodexEndpoint(w, "audio transcription")
		return
	}

	url := route.Provider.GetRequestURL(modelName)
	url = url[:len(url)-len("/chat/completions")] + "/audio/transcriptions"

	middleware.LogProviderResolved(r.Context(), h.lookupProviderName(modelName), url, "audio_transcription", modelName)

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
		data.CallType = "audio_transcription"
		data.EndTime = endTime
		data.Latency = endTime.Sub(startTime)
		applyOpenAISubscriptionLogAttribution(&data, openAISubscriptionAttributionForAction(subscriptionAttribution, "audio_transcription"))
		go h.Callbacks.LogSuccess(data)
	}
}

func audioTranscriptionModelName(contentType string, body []byte) string {
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		return ""
	}
	switch mediaType {
	case "application/x-www-form-urlencoded":
		values, err := url.ParseQuery(string(body))
		if err != nil {
			return ""
		}
		return values.Get("model")
	case "multipart/form-data":
		boundary := params["boundary"]
		if strings.TrimSpace(boundary) == "" {
			return ""
		}
		reader := multipart.NewReader(bytes.NewReader(body), boundary)
		form, err := reader.ReadForm(32 << 20)
		if err != nil {
			return ""
		}
		defer func() {
			_ = form.RemoveAll()
		}()
		if values := form.Value["model"]; len(values) > 0 {
			return values[0]
		}
		return ""
	default:
		return ""
	}
}

// AudioSpeech handles POST /v1/audio/speech.
func (h *Handlers) AudioSpeech(w http.ResponseWriter, r *http.Request) {
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
	var req model.AudioSpeechRequest
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
		writeUnsupportedChatGPTCodexEndpoint(w, "audio speech")
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
		writeUnsupportedChatGPTCodexEndpoint(w, "audio speech")
		return
	}

	url := route.Provider.GetRequestURL(req.Model)
	url = url[:len(url)-len("/chat/completions")] + "/audio/speech"

	middleware.LogProviderResolved(r.Context(), h.lookupProviderName(req.Model), url, "audio_speech", req.Model)

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

	if resp.StatusCode >= 200 && resp.StatusCode < 300 && h.Callbacks != nil {
		endTime := time.Now()
		data := buildBaseLogData(r.Context(), startTime)
		data.Model = req.Model
		data.Provider = h.lookupProviderName(req.Model)
		data.CallType = "audio_speech"
		data.EndTime = endTime
		data.Latency = endTime.Sub(startTime)
		applyOpenAISubscriptionLogAttribution(&data, openAISubscriptionAttributionForAction(subscriptionAttribution, "audio_speech"))
		go h.Callbacks.LogSuccess(data)
	}
}
