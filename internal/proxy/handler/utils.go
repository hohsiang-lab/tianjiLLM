package handler

import (
	"net/http"

	"github.com/praxisllmlab/tianjiLLM/internal/model"
	"github.com/praxisllmlab/tianjiLLM/internal/provider"
	"github.com/praxisllmlab/tianjiLLM/internal/token"
)

// SupportedOpenAIParams handles GET /utils/supported_openai_params?model=...
// Returns the list of OpenAI-compatible parameters supported by the provider for the given model.
func (h *Handlers) SupportedOpenAIParams(w http.ResponseWriter, r *http.Request) {
	modelName := r.URL.Query().Get("model")
	if modelName == "" {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "model parameter required", Type: "invalid_request_error"},
		})
		return
	}

	providerName, _ := provider.ParseModelName(modelName)
	p, err := provider.Get(providerName)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "unknown provider: " + providerName, Type: "invalid_request_error"},
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"model":            modelName,
		"provider":         providerName,
		"supported_params": p.GetSupportedParams(),
	})
}

// TokenCount handles POST /utils/token_counter.
// Accepts a model and either messages or text, returns the token count.
func (h *Handlers) TokenCount(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Model    string `json:"model"`
		Text     string `json:"text"`
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
			Name    string `json:"name"`
		} `json:"messages"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "invalid request: " + err.Error(), Type: "invalid_request_error"},
		})
		return
	}
	if req.Model == "" {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "model required", Type: "invalid_request_error"},
		})
		return
	}

	tc := h.TokenCounter
	if tc == nil {
		// Fallback: create a temporary counter if none was injected
		tc = token.New()
		h.TokenCounter = tc
	}

	var count int
	if len(req.Messages) > 0 {
		msgs := make([]token.Message, len(req.Messages))
		for i, m := range req.Messages {
			msgs[i] = token.Message{Role: m.Role, Content: m.Content, Name: m.Name}
		}
		count = tc.CountMessages(req.Model, msgs)
	} else {
		count = tc.CountText(req.Model, req.Text)
	}

	if count < 0 {
		writeJSON(w, http.StatusOK, map[string]any{
			"model":       req.Model,
			"token_count": nil,
			"error":       "token counting not supported for this model",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"model":       req.Model,
		"token_count": count,
	})
}
