package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/praxisllmlab/tianjiLLM/internal/callback"
	"github.com/praxisllmlab/tianjiLLM/internal/config"
	_ "github.com/praxisllmlab/tianjiLLM/internal/provider/openai" // register openai provider
)

func embeddingTestHandlers(upstreamURL string) *Handlers {
	apiKey := "test-key"
	cfg := &config.ProxyConfig{
		ModelList: []config.ModelConfig{
			{
				ModelName: "text-embedding-ada-002",
				TianjiParams: config.TianjiParams{
					Model:   "openai/text-embedding-ada-002",
					APIKey:  &apiKey,
					APIBase: &upstreamURL,
				},
			},
		},
	}
	return &Handlers{Config: cfg, Callbacks: callback.NewRegistry()}
}

func rerankTestHandlers(upstreamURL string) *Handlers {
	apiKey := "test-key"
	cfg := &config.ProxyConfig{
		ModelList: []config.ModelConfig{
			{
				ModelName: "jina-reranker-v2-base-multilingual",
				TianjiParams: config.TianjiParams{
					Model:   "openai/jina-reranker-v2-base-multilingual",
					APIKey:  &apiKey,
					APIBase: &upstreamURL,
				},
			},
		},
	}
	return &Handlers{Config: cfg, Callbacks: callback.NewRegistry()}
}

func completionLegacyTestHandlers(upstreamURL string) *Handlers {
	apiKey := "test-key"
	cfg := &config.ProxyConfig{
		ModelList: []config.ModelConfig{
			{
				ModelName: "gpt-3.5-turbo-instruct",
				TianjiParams: config.TianjiParams{
					Model:   "openai/gpt-3.5-turbo-instruct",
					APIKey:  &apiKey,
					APIBase: &upstreamURL,
				},
			},
		},
	}
	return &Handlers{Config: cfg, Callbacks: callback.NewRegistry()}
}

// lastCall returns the most recent LogData recorded by the spy.
func (s *spyLogger) lastCall(t *testing.T) callback.LogData {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.calls) == 0 {
		t.Fatal("no LogSuccess calls recorded")
	}
	return s.calls[len(s.calls)-1]
}

// ── Embedding ──────────────────────────────────────────────────────────────────

func TestEmbedding_SpendLog_Called(t *testing.T) {
	t.Parallel()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		resp := map[string]any{
			"object": "list",
			"data":   []any{},
			"model":  "text-embedding-ada-002",
			"usage":  map[string]any{"prompt_tokens": 8, "total_tokens": 8},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer upstream.Close()

	spy := newSpyLogger()
	h := embeddingTestHandlers(upstream.URL)
	h.Callbacks.Register(spy)

	body := `{"model":"text-embedding-ada-002","input":"hello world"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/embeddings", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Embedding(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	spy.waitCalled(t, 2*time.Second)

	data := spy.lastCall(t)
	if data.PromptTokens != 8 {
		t.Errorf("expected PromptTokens=8, got %d", data.PromptTokens)
	}
	if data.TotalTokens != 8 {
		t.Errorf("expected TotalTokens=8, got %d", data.TotalTokens)
	}
	if data.CallType != "embedding" {
		t.Errorf("expected CallType=embedding, got %q", data.CallType)
	}
}

func TestEmbedding_SpendLog_NotCalledOnError(t *testing.T) {
	t.Parallel()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"provider error"}`))
	}))
	defer upstream.Close()

	spy := newSpyLogger()
	h := embeddingTestHandlers(upstream.URL)
	h.Callbacks.Register(spy)

	body := `{"model":"text-embedding-ada-002","input":"hello"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/embeddings", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Embedding(w, req)

	time.Sleep(100 * time.Millisecond)
	if spy.logCount() != 0 {
		t.Errorf("expected 0 LogSuccess calls on error, got %d", spy.logCount())
	}
}

func TestEmbedding_SpendLog_MissingUsage(t *testing.T) {
	t.Parallel()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"object":"list","data":[],"model":"text-embedding-ada-002"}`))
	}))
	defer upstream.Close()

	spy := newSpyLogger()
	h := embeddingTestHandlers(upstream.URL)
	h.Callbacks.Register(spy)

	body := `{"model":"text-embedding-ada-002","input":"hello"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/embeddings", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Embedding(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	spy.waitCalled(t, 2*time.Second)
	data := spy.lastCall(t)
	if data.PromptTokens != 0 || data.TotalTokens != 0 {
		t.Errorf("expected zero tokens for missing usage, got prompt=%d total=%d", data.PromptTokens, data.TotalTokens)
	}
}

// ── Rerank ────────────────────────────────────────────────────────────────────

func TestRerank_SpendLog_Called(t *testing.T) {
	t.Parallel()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		resp := map[string]any{
			"results": []any{},
			"model":   "jina-reranker-v2-base-multilingual",
			"usage":   map[string]any{"total_tokens": 42},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer upstream.Close()

	spy := newSpyLogger()
	h := rerankTestHandlers(upstream.URL)
	h.Callbacks.Register(spy)

	body := `{"model":"jina-reranker-v2-base-multilingual","query":"test","documents":["doc1","doc2"]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/rerank", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Rerank(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	spy.waitCalled(t, 2*time.Second)

	data := spy.lastCall(t)
	if data.TotalTokens != 42 {
		t.Errorf("expected TotalTokens=42, got %d", data.TotalTokens)
	}
	if data.CallType != "rerank" {
		t.Errorf("expected CallType=rerank, got %q", data.CallType)
	}
}

func TestRerank_ResponseBodyForwarded(t *testing.T) {
	t.Parallel()

	expectedBody := `{"results":[{"index":0,"relevance_score":0.9}],"model":"jina-reranker-v2-base-multilingual","usage":{"total_tokens":10}}`
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(expectedBody))
	}))
	defer upstream.Close()

	h := rerankTestHandlers(upstream.URL)

	body := `{"model":"jina-reranker-v2-base-multilingual","query":"test","documents":["doc1"]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/rerank", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Rerank(w, req)

	if w.Body.String() != expectedBody {
		t.Errorf("response body mismatch\nwant: %s\ngot:  %s", expectedBody, w.Body.String())
	}
}

func TestRerank_SpendLog_NotCalledOnError(t *testing.T) {
	t.Parallel()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer upstream.Close()

	spy := newSpyLogger()
	h := rerankTestHandlers(upstream.URL)
	h.Callbacks.Register(spy)

	body := `{"model":"jina-reranker-v2-base-multilingual","query":"test","documents":["doc1"]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/rerank", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Rerank(w, req)

	time.Sleep(100 * time.Millisecond)
	if spy.logCount() != 0 {
		t.Errorf("expected 0 LogSuccess calls on error, got %d", spy.logCount())
	}
}

// ── Legacy Completion ─────────────────────────────────────────────────────────

func TestLegacyCompletion_SpendLog_Called(t *testing.T) {
	t.Parallel()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		resp := map[string]any{
			"id":     "cmpl-abc",
			"object": "text_completion",
			"model":  "gpt-3.5-turbo-instruct",
			"choices": []any{
				map[string]any{"text": "Hello!", "index": 0, "finish_reason": "stop"},
			},
			"usage": map[string]any{
				"prompt_tokens":     5,
				"completion_tokens": 3,
				"total_tokens":      8,
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer upstream.Close()

	spy := newSpyLogger()
	h := completionLegacyTestHandlers(upstream.URL)
	h.Callbacks.Register(spy)

	body := `{"model":"gpt-3.5-turbo-instruct","prompt":"Hello"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Completion(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	spy.waitCalled(t, 2*time.Second)

	data := spy.lastCall(t)
	if data.PromptTokens != 5 {
		t.Errorf("expected PromptTokens=5, got %d", data.PromptTokens)
	}
	if data.CompletionTokens != 3 {
		t.Errorf("expected CompletionTokens=3, got %d", data.CompletionTokens)
	}
	if data.TotalTokens != 8 {
		t.Errorf("expected TotalTokens=8, got %d", data.TotalTokens)
	}
	if data.CallType != "completion" {
		t.Errorf("expected CallType=completion, got %q", data.CallType)
	}
}

func TestLegacyCompletion_ResponseBodyForwarded(t *testing.T) {
	t.Parallel()

	expectedBody := `{"id":"cmpl-abc","choices":[{"text":"Hello!","index":0,"finish_reason":"stop"}],"usage":{"prompt_tokens":5,"completion_tokens":3,"total_tokens":8}}`
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(expectedBody))
	}))
	defer upstream.Close()

	h := completionLegacyTestHandlers(upstream.URL)

	body := `{"model":"gpt-3.5-turbo-instruct","prompt":"Hello"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Completion(w, req)

	if w.Body.String() != expectedBody {
		t.Errorf("response body mismatch\nwant: %s\ngot:  %s", expectedBody, w.Body.String())
	}
}

func TestLegacyCompletion_SpendLog_NotCalledOnError(t *testing.T) {
	t.Parallel()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer upstream.Close()

	spy := newSpyLogger()
	h := completionLegacyTestHandlers(upstream.URL)
	h.Callbacks.Register(spy)

	body := `{"model":"gpt-3.5-turbo-instruct","prompt":"Hello"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Completion(w, req)

	time.Sleep(100 * time.Millisecond)
	if spy.logCount() != 0 {
		t.Errorf("expected 0 LogSuccess calls on error, got %d", spy.logCount())
	}
}
