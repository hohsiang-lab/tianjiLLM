package handler

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/praxisllmlab/tianjiLLM/internal/model"
)

// assistantsProxy creates a reverse proxy for Assistants API endpoints.
// It resolves the upstream from config and forwards the request as-is.
func (h *Handlers) assistantsProxy(w http.ResponseWriter, r *http.Request) {
	upstream, apiKey := h.resolveAssistantsUpstream()
	if upstream == "" {
		writeJSON(w, http.StatusNotImplemented, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "assistants API not configured", Type: "not_supported"},
		})
		return
	}

	target, err := url.Parse(upstream)
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
			if apiKey != "" {
				req.Header.Set("Authorization", "Bearer "+apiKey)
			}
			req.Header.Set("OpenAI-Beta", "assistants=v2")
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			log.Printf("assistants proxy error: %v", err)
			http.Error(w, `{"error":"upstream request failed"}`, http.StatusBadGateway)
		},
	}

	proxy.ServeHTTP(w, r)
}

// resolveAssistantsUpstream returns the upstream base URL and API key
// for the Assistants API from config.
func (h *Handlers) resolveAssistantsUpstream() (string, string) {
	if h.Config.AssistantSettings != nil {
		return h.Config.AssistantSettings.APIBase, h.Config.AssistantSettings.APIKey
	}

	// Fallback: look for an openai model in the config
	for _, m := range h.Config.ModelList {
		if m.TianjiParams.Model == "openai/gpt-4o" || m.TianjiParams.Model == "gpt-4o" {
			apiKey := ""
			if m.TianjiParams.APIKey != nil {
				apiKey = *m.TianjiParams.APIKey
			}
			base := "https://api.openai.com"
			if m.TianjiParams.APIBase != nil {
				base = *m.TianjiParams.APIBase
			}
			return base, apiKey
		}
	}

	return "", ""
}

// AssistantCreate handles POST /v1/assistants.
func (h *Handlers) AssistantCreate(w http.ResponseWriter, r *http.Request) {
	h.assistantsProxy(w, r)
}

// AssistantGet handles GET /v1/assistants/{assistant_id}.
func (h *Handlers) AssistantGet(w http.ResponseWriter, r *http.Request) {
	h.assistantsProxy(w, r)
}

// AssistantList handles GET /v1/assistants.
func (h *Handlers) AssistantList(w http.ResponseWriter, r *http.Request) {
	h.assistantsProxy(w, r)
}

// AssistantModify handles POST /v1/assistants/{assistant_id}.
func (h *Handlers) AssistantModify(w http.ResponseWriter, r *http.Request) {
	h.assistantsProxy(w, r)
}

// AssistantDelete handles DELETE /v1/assistants/{assistant_id}.
func (h *Handlers) AssistantDelete(w http.ResponseWriter, r *http.Request) {
	h.assistantsProxy(w, r)
}

// ThreadCreate handles POST /v1/threads.
func (h *Handlers) ThreadCreate(w http.ResponseWriter, r *http.Request) {
	h.assistantsProxy(w, r)
}

// ThreadGet handles GET /v1/threads/{thread_id}.
func (h *Handlers) ThreadGet(w http.ResponseWriter, r *http.Request) {
	h.assistantsProxy(w, r)
}

// ThreadModify handles POST /v1/threads/{thread_id}.
func (h *Handlers) ThreadModify(w http.ResponseWriter, r *http.Request) {
	h.assistantsProxy(w, r)
}

// ThreadDelete handles DELETE /v1/threads/{thread_id}.
func (h *Handlers) ThreadDelete(w http.ResponseWriter, r *http.Request) {
	h.assistantsProxy(w, r)
}

// MessageCreate handles POST /v1/threads/{thread_id}/messages.
func (h *Handlers) MessageCreate(w http.ResponseWriter, r *http.Request) {
	h.assistantsProxy(w, r)
}

// MessageList handles GET /v1/threads/{thread_id}/messages.
func (h *Handlers) MessageList(w http.ResponseWriter, r *http.Request) {
	h.assistantsProxy(w, r)
}

// MessageGet handles GET /v1/threads/{thread_id}/messages/{message_id}.
func (h *Handlers) MessageGet(w http.ResponseWriter, r *http.Request) {
	h.assistantsProxy(w, r)
}

// RunCreate handles POST /v1/threads/{thread_id}/runs.
// Supports stream=true via SSE passthrough.
func (h *Handlers) RunCreate(w http.ResponseWriter, r *http.Request) {
	h.assistantsProxy(w, r)
}

// RunGet handles GET /v1/threads/{thread_id}/runs/{run_id}.
func (h *Handlers) RunGet(w http.ResponseWriter, r *http.Request) {
	h.assistantsProxy(w, r)
}

// RunList handles GET /v1/threads/{thread_id}/runs.
func (h *Handlers) RunList(w http.ResponseWriter, r *http.Request) {
	h.assistantsProxy(w, r)
}

// RunCancel handles POST /v1/threads/{thread_id}/runs/{run_id}/cancel.
func (h *Handlers) RunCancel(w http.ResponseWriter, r *http.Request) {
	h.assistantsProxy(w, r)
}

// RunStepsList handles GET /v1/threads/{thread_id}/runs/{run_id}/steps.
func (h *Handlers) RunStepsList(w http.ResponseWriter, r *http.Request) {
	h.assistantsProxy(w, r)
}

// RunStepGet handles GET /v1/threads/{thread_id}/runs/{run_id}/steps/{step_id}.
func (h *Handlers) RunStepGet(w http.ResponseWriter, r *http.Request) {
	h.assistantsProxy(w, r)
}
