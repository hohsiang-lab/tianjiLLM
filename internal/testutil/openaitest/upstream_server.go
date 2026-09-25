package openaitest

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
)

type UpstreamServerOptions struct {
	Models             []ModelFixture
	ChatCompletionBody map[string]any
	ChatCompletionSSE  []string
	EmbeddingBody      map[string]any
	ResponseHeaders    http.Header
}

type ModelFixture struct {
	ID      string
	OwnedBy string
}

type UpstreamServer struct {
	server *httptest.Server
	opts   UpstreamServerOptions

	mu       sync.Mutex
	requests []RecordedRequest
}

func NewUpstreamServer(t testing.TB, opts UpstreamServerOptions) *UpstreamServer {
	t.Helper()
	s := &UpstreamServer{opts: opts}
	s.server = httptest.NewServer(http.HandlerFunc(s.handle))
	t.Cleanup(s.server.Close)
	return s
}

func (s *UpstreamServer) URL() string { return s.server.URL }

func (s *UpstreamServer) BaseURL() string { return s.server.URL + "/v1" }

func (s *UpstreamServer) Host() string {
	parsed, err := url.Parse(s.server.URL)
	if err != nil {
		return ""
	}
	return parsed.Host
}

func (s *UpstreamServer) Client() *http.Client { return s.server.Client() }

func (s *UpstreamServer) Requests() []RecordedRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]RecordedRequest, len(s.requests))
	copy(out, s.requests)
	return out
}

func (s *UpstreamServer) handle(w http.ResponseWriter, r *http.Request) {
	recorded := recordRequest(r)
	s.mu.Lock()
	s.requests = append(s.requests, recorded)
	s.mu.Unlock()

	for name, values := range s.opts.ResponseHeaders {
		for _, value := range values {
			w.Header().Add(name, value)
		}
	}

	switch r.URL.Path {
	case "/v1/models":
		if r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method_not_allowed"})
			return
		}
		writeJSON(w, http.StatusOK, s.modelsResponse())
	case "/v1/chat/completions":
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method_not_allowed"})
			return
		}
		if s.opts.ChatCompletionSSE != nil {
			w.Header().Set("Content-Type", "text/event-stream")
			for _, data := range s.opts.ChatCompletionSSE {
				fmt.Fprintf(w, "data: %s\n\n", data)
			}
			return
		}
		writeJSON(w, http.StatusOK, s.chatCompletionResponse())
	case "/v1/embeddings":
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method_not_allowed"})
			return
		}
		writeJSON(w, http.StatusOK, s.embeddingResponse())
	default:
		writeJSON(w, http.StatusNotFound, map[string]any{
			"error": fmt.Sprintf("unexpected OpenAI upstream mock path: %s", r.URL.Path),
		})
	}
}

func (s *UpstreamServer) modelsResponse() map[string]any {
	models := s.opts.Models
	if len(models) == 0 {
		models = []ModelFixture{{ID: "gpt-4o", OwnedBy: "openai"}}
	}
	data := make([]any, 0, len(models))
	for _, model := range models {
		ownedBy := model.OwnedBy
		if ownedBy == "" {
			ownedBy = "openai"
		}
		data = append(data, map[string]any{
			"id":       model.ID,
			"object":   "model",
			"owned_by": ownedBy,
		})
	}
	return map[string]any{"object": "list", "data": data}
}

func (s *UpstreamServer) chatCompletionResponse() map[string]any {
	if s.opts.ChatCompletionBody != nil {
		return s.opts.ChatCompletionBody
	}
	return map[string]any{
		"id":      "chatcmpl_mock",
		"object":  "chat.completion",
		"created": float64(1700000000),
		"model":   "gpt-4o",
		"choices": []any{map[string]any{
			"index": float64(0),
			"message": map[string]any{
				"role":    "assistant",
				"content": "mock response",
			},
			"finish_reason": "stop",
		}},
		"usage": map[string]any{"prompt_tokens": float64(1), "completion_tokens": float64(1), "total_tokens": float64(2)},
	}
}

func (s *UpstreamServer) embeddingResponse() map[string]any {
	if s.opts.EmbeddingBody != nil {
		return s.opts.EmbeddingBody
	}
	return map[string]any{
		"object": "list",
		"data": []any{map[string]any{
			"object":    "embedding",
			"index":     float64(0),
			"embedding": []any{float64(0.1), float64(0.2), float64(0.3)},
		}},
		"model": "text-embedding-3-small",
		"usage": map[string]any{"prompt_tokens": float64(1), "total_tokens": float64(1)},
	}
}
