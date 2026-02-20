package handler

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/praxisllmlab/tianjiLLM/internal/model"
)

// nativeProxy creates a reverse proxy to a specific provider's base URL.
func (h *Handlers) nativeProxy(w http.ResponseWriter, r *http.Request, providerName string) {
	baseURL, apiKey := h.resolveNativeUpstream(providerName)
	if baseURL == "" {
		writeJSON(w, http.StatusNotImplemented, model.ErrorResponse{
			Error: model.ErrorDetail{Message: providerName + " not configured", Type: "not_supported"},
		})
		return
	}

	target, err := url.Parse(baseURL)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "invalid upstream URL", Type: "internal_error"},
		})
		return
	}

	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = target.Scheme
			req.URL.Host = target.Host
			req.Host = target.Host

			switch providerName {
			case "anthropic":
				req.Header.Set("x-api-key", apiKey)
				req.Header.Set("anthropic-version", "2023-06-01")
			default:
				if apiKey != "" {
					req.Header.Set("Authorization", "Bearer "+apiKey)
				}
			}
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			log.Printf("native proxy error (%s): %v", providerName, err)
			http.Error(w, `{"error":"upstream request failed"}`, http.StatusBadGateway)
		},
	}

	proxy.ServeHTTP(w, r)
}

// resolveNativeUpstream finds the base URL and API key for a native provider.
func (h *Handlers) resolveNativeUpstream(providerName string) (string, string) {
	for _, m := range h.Config.ModelList {
		parts := strings.SplitN(m.TianjiParams.Model, "/", 2)
		if len(parts) >= 1 && parts[0] == providerName {
			apiKey := ""
			if m.TianjiParams.APIKey != nil {
				apiKey = *m.TianjiParams.APIKey
			}
			base := ""
			if m.TianjiParams.APIBase != nil {
				base = *m.TianjiParams.APIBase
			}
			if base == "" {
				base = defaultBaseURL(providerName)
			}
			return base, apiKey
		}
	}
	return "", ""
}

func defaultBaseURL(provider string) string {
	switch provider {
	case "openai":
		return "https://api.openai.com"
	case "anthropic":
		return "https://api.anthropic.com"
	case "gemini":
		return "https://generativelanguage.googleapis.com"
	case "cohere":
		return "https://api.cohere.ai"
	case "mistral":
		return "https://api.mistral.ai"
	default:
		return ""
	}
}

// AnthropicMessages handles POST /v1/messages (Anthropic native format).
func (h *Handlers) AnthropicMessages(w http.ResponseWriter, r *http.Request) {
	h.nativeProxy(w, r, "anthropic")
}

// AnthropicCountTokens handles POST /v1/messages/count_tokens.
func (h *Handlers) AnthropicCountTokens(w http.ResponseWriter, r *http.Request) {
	h.nativeProxy(w, r, "anthropic")
}

// GeminiGenerateContent handles POST /v1beta/models/{name}:generateContent.
func (h *Handlers) GeminiGenerateContent(w http.ResponseWriter, r *http.Request) {
	h.nativeProxy(w, r, "gemini")
}

// GeminiStreamGenerateContent handles POST /v1beta/models/{name}:streamGenerateContent.
func (h *Handlers) GeminiStreamGenerateContent(w http.ResponseWriter, r *http.Request) {
	h.nativeProxy(w, r, "gemini")
}

// GeminiCountTokens handles POST /v1beta/models/{name}:countTokens.
func (h *Handlers) GeminiCountTokens(w http.ResponseWriter, r *http.Request) {
	h.nativeProxy(w, r, "gemini")
}

// ImagesEdit handles POST /v1/images/edits.
func (h *Handlers) ImagesEdit(w http.ResponseWriter, r *http.Request) {
	h.assistantsProxy(w, r)
}

// ImageVariation handles POST /v1/images/variations.
func (h *Handlers) ImageVariation(w http.ResponseWriter, r *http.Request) {
	h.assistantsProxy(w, r)
}
