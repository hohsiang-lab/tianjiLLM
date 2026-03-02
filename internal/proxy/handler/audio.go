package handler

import (
	"net/http"

	"github.com/praxisllmlab/tianjiLLM/internal/model"
	"github.com/praxisllmlab/tianjiLLM/internal/proxy/middleware"
)

// AudioTranscription handles POST /v1/audio/transcriptions.
// This is a multipart form upload — we proxy the raw request body.
func (h *Handlers) AudioTranscription(w http.ResponseWriter, r *http.Request) {
	modelName := r.FormValue("model")
	if modelName == "" {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{
				Message: "model is required",
				Type:    "invalid_request_error",
			},
		})
		return
	}

	p, apiKey, _, err := h.resolveProviderFromConfig(modelName)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{
				Message: err.Error(),
				Type:    "invalid_request_error",
			},
		})
		return
	}

	url := p.GetRequestURL(modelName)
	url = url[:len(url)-len("/chat/completions")] + "/audio/transcriptions"

	// Phase 2: provider.resolved
	middleware.LogProviderResolved(r.Context(), h.lookupProviderName(modelName), url, "audio_transcription", modelName)

	proxyUpstream(w, r, url, apiKey, p)
}

// AudioSpeech handles POST /v1/audio/speech.
func (h *Handlers) AudioSpeech(w http.ResponseWriter, r *http.Request) {
	var req model.AudioSpeechRequest
	if err := decodeJSON(r, &req); err != nil {
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

	url := p.GetRequestURL(req.Model)
	url = url[:len(url)-len("/chat/completions")] + "/audio/speech"

	// Phase 2: provider.resolved
	middleware.LogProviderResolved(r.Context(), h.lookupProviderName(req.Model), url, "audio_speech", req.Model)

	proxyUpstream(w, r, url, apiKey, p)
}
