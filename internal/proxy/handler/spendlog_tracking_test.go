package handler

// Tests for HO-78: SpendLogs tracking for embedding, rerank, completion endpoints.

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/praxisllmlab/tianjiLLM/internal/callback"
	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "github.com/praxisllmlab/tianjiLLM/internal/provider/openai"
)

func newEmbeddingTestHandlers(upstreamURL string) *Handlers {
	apiKey := "test-key"
	apiBase := upstreamURL + "/v1"
	return &Handlers{
		Config: &config.ProxyConfig{
			ModelList: []config.ModelConfig{
				{
					ModelName: "text-embedding-ada-002",
					TianjiParams: config.TianjiParams{
						Model:   "openai/text-embedding-ada-002",
						APIKey:  &apiKey,
						APIBase: &apiBase,
					},
				},
			},
		},
	}
}

func newRerankTestHandlers(upstreamURL string) *Handlers {
	apiKey := "test-key"
	// resolveProviderBaseURL looks for model name match or first OpenAI provider
	return &Handlers{
		Config: &config.ProxyConfig{
			ModelList: []config.ModelConfig{
				{
					ModelName: "rerank-english-v3.0",
					TianjiParams: config.TianjiParams{
						Model:   "openai/rerank-english-v3.0",
						APIKey:  &apiKey,
						APIBase: &upstreamURL,
					},
				},
			},
		},
	}
}

func newCompletionTestHandlers(upstreamURL string) *Handlers {
	apiKey := "test-key"
	apiBase := upstreamURL + "/v1"
	return &Handlers{
		Config: &config.ProxyConfig{
			ModelList: []config.ModelConfig{
				{
					ModelName: "gpt-3.5-turbo-instruct",
					TianjiParams: config.TianjiParams{
						Model:   "openai/gpt-3.5-turbo-instruct",
						APIKey:  &apiKey,
						APIBase: &apiBase,
					},
				},
			},
		},
	}
}

// TestEmbedding_LogSuccess_CallType verifies that Embedding fires LogSuccess
// with CallType "embedding" and correct token counts.
func TestEmbedding_LogSuccess_CallType(t *testing.T) {
	t.Parallel()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"object":"list","data":[{"object":"embedding","index":0,"embedding":[0.1,0.2]}],"model":"text-embedding-ada-002","usage":{"prompt_tokens":8,"total_tokens":8}}`))
	}))
	defer upstream.Close()

	spy := newSpyLogger()
	reg := callback.NewRegistry()
	reg.Register(spy)

	h := newEmbeddingTestHandlers(upstream.URL)
	h.Callbacks = reg

	req := httptest.NewRequest("POST", "/v1/embeddings", strings.NewReader(`{"model":"text-embedding-ada-002","input":"hello world"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Embedding(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	spy.waitCalled(t, 2*time.Second)

	spy.mu.Lock()
	data := spy.calls[0]
	spy.mu.Unlock()

	assert.Equal(t, "embedding", data.CallType)
	assert.Equal(t, 8, data.PromptTokens)
	assert.Equal(t, 8, data.TotalTokens)
}

// TestRerank_LogSuccess_CallType verifies that Rerank fires LogSuccess
// with CallType "rerank" and total_tokens from the response.
func TestRerank_LogSuccess_CallType(t *testing.T) {
	t.Parallel()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"results":[{"index":0,"relevance_score":0.9}],"usage":{"total_tokens":42}}`))
	}))
	defer upstream.Close()

	spy := newSpyLogger()
	reg := callback.NewRegistry()
	reg.Register(spy)

	h := newRerankTestHandlers(upstream.URL)
	h.Callbacks = reg

	req := httptest.NewRequest("POST", "/v1/rerank", strings.NewReader(`{"model":"rerank-english-v3.0","query":"test","documents":["doc1","doc2"]}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Rerank(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	spy.waitCalled(t, 2*time.Second)

	spy.mu.Lock()
	data := spy.calls[0]
	spy.mu.Unlock()

	assert.Equal(t, "rerank", data.CallType)
	assert.Equal(t, 42, data.TotalTokens)
}

// TestCompletion_LogSuccess_CallType verifies that Completion fires LogSuccess
// with CallType "completion" and correct prompt/completion token counts.
func TestCompletion_LogSuccess_CallType(t *testing.T) {
	t.Parallel()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"cmpl-1","object":"text_completion","choices":[{"text":"hello","finish_reason":"stop","index":0}],"usage":{"prompt_tokens":5,"completion_tokens":1,"total_tokens":6}}`))
	}))
	defer upstream.Close()

	spy := newSpyLogger()
	reg := callback.NewRegistry()
	reg.Register(spy)

	h := newCompletionTestHandlers(upstream.URL)
	h.Callbacks = reg

	req := httptest.NewRequest("POST", "/v1/completions", strings.NewReader(`{"model":"gpt-3.5-turbo-instruct","prompt":"Say hi"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Completion(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	spy.waitCalled(t, 2*time.Second)

	spy.mu.Lock()
	data := spy.calls[0]
	spy.mu.Unlock()

	assert.Equal(t, "completion", data.CallType)
	assert.Equal(t, 5, data.PromptTokens)
	assert.Equal(t, 1, data.CompletionTokens)
	assert.Equal(t, 6, data.TotalTokens)
}
