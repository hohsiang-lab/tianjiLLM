package handler

// Tests for HO-84: rerank/embedding SpendLog token count 為 0
//
// Root cause:
//   - rerank.go: LogData.PromptTokens is NEVER set (handler only sets TotalTokens).
//     RerankUsage has no PromptTokens field. SpendLog always records PromptTokens=0.
//   - rerank.go: If upstream (e.g. Cohere) returns usage under a different key (not
//     "usage"), parsed.Usage is nil and TotalTokens is also 0 → UI shows "(0+0)".
//   - embedding.go: PromptTokens is passed correctly when upstream returns prompt_tokens.
//
// These tests are EXPECTED TO FAIL on the current main branch.

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/praxisllmlab/tianjiLLM/internal/callback"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "github.com/praxisllmlab/tianjiLLM/internal/provider/openai"
)

// TestHO84_Rerank_PromptTokens_IsNonZero verifies that after a rerank call,
// SpendLog.PromptTokens > 0.
//
// CURRENT BEHAVIOR (BUG): rerank.go only sets TotalTokens in LogData.
// PromptTokens is never populated → SpendLog.PromptTokens = 0.
//
// STATUS: FAIL
func TestHO84_Rerank_PromptTokens_IsNonZero(t *testing.T) {
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

	req := httptest.NewRequest("POST", "/v1/rerank",
		strings.NewReader(`{"model":"rerank-english-v3.0","query":"test query","documents":["doc1","doc2"]}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Rerank(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	spy.waitCalled(t, 2*time.Second)

	spy.mu.Lock()
	data := spy.calls[0]
	spy.mu.Unlock()

	// BUG: FAILS — rerank.go never sets LogData.PromptTokens. Always 0.
	assert.Greater(t, data.PromptTokens, 0,
		"SpendLog.PromptTokens should be > 0 after rerank call (got %d); "+
			"rerank.go never sets LogData.PromptTokens — only TotalTokens is set", data.PromptTokens)
}

// TestHO84_Rerank_TotalTokens_WithCohereStyleResponse verifies that rerank
// handles Cohere-style upstream responses where usage is under "meta.tokens"
// instead of "usage.total_tokens".
//
// STATUS: FAIL (TotalTokens = 0 when response uses Cohere meta format)
func TestHO84_Rerank_TotalTokens_WithCohereStyleResponse(t *testing.T) {
	t.Parallel()

	cohereStyleResponse := `{
		"results": [{"index": 0, "relevance_score": 0.95}],
		"meta": {
			"billed_units": {"search_units": 1},
			"tokens": {"input_tokens": 150, "output_tokens": 0}
		}
	}`

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(cohereStyleResponse))
	}))
	defer upstream.Close()

	spy := newSpyLogger()
	reg := callback.NewRegistry()
	reg.Register(spy)

	h := newRerankTestHandlers(upstream.URL)
	h.Callbacks = reg

	req := httptest.NewRequest("POST", "/v1/rerank",
		strings.NewReader(`{"model":"rerank-english-v3.0","query":"test","documents":["doc1","doc2"]}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Rerank(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	spy.waitCalled(t, 2*time.Second)

	spy.mu.Lock()
	data := spy.calls[0]
	spy.mu.Unlock()

	// BUG: FAILS — parsed.Usage is nil (no "usage" key), so totalTokens=0.
	// Cohere returns tokens under "meta.tokens.input_tokens", not "usage.total_tokens".
	assert.Greater(t, data.TotalTokens, 0,
		"SpendLog.TotalTokens should be > 0 for Cohere-style rerank response (got %d); "+
			"RerankUsage struct only handles {\"usage\":{\"total_tokens\":N}}, "+
			"not Cohere's {\"meta\":{\"tokens\":{\"input_tokens\":N}}}", data.TotalTokens)
}

// TestHO84_Embedding_PromptTokens_IsNonZero verifies that after an embedding call,
// SpendLog.PromptTokens > 0 when upstream returns prompt_tokens.
//
// STATUS: Should PASS (embedding.go correctly reads result.Usage.PromptTokens).
// Included to document the contract and catch regression.
func TestHO84_Embedding_PromptTokens_IsNonZero(t *testing.T) {
	t.Parallel()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"object": "list",
			"data": [{"object": "embedding", "index": 0, "embedding": [0.1, 0.2, 0.3]}],
			"model": "text-embedding-ada-002",
			"usage": {"prompt_tokens": 10, "total_tokens": 10}
		}`))
	}))
	defer upstream.Close()

	spy := newSpyLogger()
	reg := callback.NewRegistry()
	reg.Register(spy)

	h := newEmbeddingTestHandlers(upstream.URL)
	h.Callbacks = reg

	req := httptest.NewRequest("POST", "/v1/embeddings",
		strings.NewReader(`{"model":"text-embedding-ada-002","input":"test embedding input"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Embedding(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	spy.waitCalled(t, 2*time.Second)

	spy.mu.Lock()
	data := spy.calls[0]
	spy.mu.Unlock()

	assert.Greater(t, data.PromptTokens, 0,
		"SpendLog.PromptTokens should be > 0 after embedding call (got %d)", data.PromptTokens)
	assert.Greater(t, data.TotalTokens, 0,
		"SpendLog.TotalTokens should be > 0 after embedding call (got %d)", data.TotalTokens)
}

// TestHO84_Rerank_PromptTokens_EqualsTotalTokens verifies the fix contract:
// For rerank calls (no prompt/completion distinction), PromptTokens should
// be set equal to TotalTokens so the UI can display token usage.
//
// STATUS: FAIL (current: PromptTokens=0, TotalTokens=100)
func TestHO84_Rerank_PromptTokens_EqualsTotalTokens(t *testing.T) {
	t.Parallel()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"results":[{"index":0,"relevance_score":0.8}],"usage":{"total_tokens":100}}`))
	}))
	defer upstream.Close()

	spy := newSpyLogger()
	reg := callback.NewRegistry()
	reg.Register(spy)

	h := newRerankTestHandlers(upstream.URL)
	h.Callbacks = reg

	req := httptest.NewRequest("POST", "/v1/rerank",
		strings.NewReader(`{"model":"rerank-english-v3.0","query":"query","documents":["a","b","c"]}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Rerank(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	spy.waitCalled(t, 2*time.Second)

	spy.mu.Lock()
	data := spy.calls[0]
	spy.mu.Unlock()

	assert.Equal(t, 100, data.TotalTokens, "TotalTokens should be 100")
	// BUG: FAILS — rerank.go never sets PromptTokens.
	// Fix: set PromptTokens = TotalTokens for rerank (no breakdown available).
	assert.Equal(t, 100, data.PromptTokens,
		"SpendLog.PromptTokens should equal TotalTokens for rerank; got PromptTokens=%d, TotalTokens=%d",
		data.PromptTokens, data.TotalTokens)
}
