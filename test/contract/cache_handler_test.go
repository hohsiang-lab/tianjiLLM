package contract

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/cache"
	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/praxisllmlab/tianjiLLM/internal/proxy"
	"github.com/praxisllmlab/tianjiLLM/internal/proxy/handler"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCacheHandler_MissAndStore(t *testing.T) {
	// Mock upstream that returns a fixed response
	callCount := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":     "chatcmpl-123",
			"object": "chat.completion",
			"model":  "gpt-4o-mini",
			"choices": []map[string]any{
				{
					"index":         0,
					"message":       map[string]any{"role": "assistant", "content": "Hello!"},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]any{
				"prompt_tokens":     5,
				"completion_tokens": 1,
				"total_tokens":      6,
			},
		})
	}))
	defer upstream.Close()

	apiBase := upstream.URL
	memCache := cache.NewMemoryCache()
	h := &handler.Handlers{
		Config: &config.ProxyConfig{
			ModelList: []config.ModelConfig{
				{
					ModelName: "gpt-4o-mini",
					TianjiParams: config.TianjiParams{
						Model:   "openai/gpt-4o-mini",
						APIKey:  strPtr("test-key"),
						APIBase: &apiBase,
					},
				},
			},
		},
		Cache: memCache,
	}

	srv := proxy.NewServer(proxy.ServerConfig{
		Handlers:  h,
		MasterKey: "sk-master",
	})

	body := `{"model":"gpt-4o-mini","messages":[{"role":"user","content":"Hello"}]}`

	// First request — cache MISS → call upstream
	req1 := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(body))
	req1.Header.Set("Content-Type", "application/json")
	req1.Header.Set("Authorization", "Bearer sk-master")
	w1 := httptest.NewRecorder()
	srv.ServeHTTP(w1, req1)

	require.Equal(t, http.StatusOK, w1.Code)
	assert.Equal(t, 1, callCount)

	// Second request — cache HIT → no upstream call
	req2 := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Authorization", "Bearer sk-master")
	w2 := httptest.NewRecorder()
	srv.ServeHTTP(w2, req2)

	require.Equal(t, http.StatusOK, w2.Code)
	assert.Equal(t, 1, callCount, "second request should be served from cache")
	assert.Equal(t, "HIT", w2.Header().Get("X-Cache"))

	// Verify response content matches
	var resp1, resp2 map[string]any
	_ = json.Unmarshal(w1.Body.Bytes(), &resp1)
	_ = json.Unmarshal(w2.Body.Bytes(), &resp2)
	assert.Equal(t, resp1["id"], resp2["id"])
}

func TestCacheHandler_DifferentMessages(t *testing.T) {
	callCount := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		body, _ := io.ReadAll(r.Body)
		var req map[string]any
		_ = json.Unmarshal(body, &req)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":     "chatcmpl-" + strings.Repeat("x", callCount),
			"object": "chat.completion",
			"model":  "gpt-4o-mini",
			"choices": []map[string]any{
				{
					"index":         0,
					"message":       map[string]any{"role": "assistant", "content": "response-" + strings.Repeat("x", callCount)},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]any{"prompt_tokens": 5, "completion_tokens": 1, "total_tokens": 6},
		})
	}))
	defer upstream.Close()

	apiBase := upstream.URL
	h := &handler.Handlers{
		Config: &config.ProxyConfig{
			ModelList: []config.ModelConfig{
				{
					ModelName: "gpt-4o-mini",
					TianjiParams: config.TianjiParams{
						Model:   "openai/gpt-4o-mini",
						APIKey:  strPtr("test-key"),
						APIBase: &apiBase,
					},
				},
			},
		},
		Cache: cache.NewMemoryCache(),
	}

	srv := proxy.NewServer(proxy.ServerConfig{
		Handlers:  h,
		MasterKey: "sk-master",
	})

	// Request 1
	body1 := `{"model":"gpt-4o-mini","messages":[{"role":"user","content":"Hello"}]}`
	req1 := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(body1))
	req1.Header.Set("Content-Type", "application/json")
	req1.Header.Set("Authorization", "Bearer sk-master")
	w1 := httptest.NewRecorder()
	srv.ServeHTTP(w1, req1)
	require.Equal(t, http.StatusOK, w1.Code)

	// Request 2 — different message, should NOT be cached
	body2 := `{"model":"gpt-4o-mini","messages":[{"role":"user","content":"Goodbye"}]}`
	req2 := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(body2))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Authorization", "Bearer sk-master")
	w2 := httptest.NewRecorder()
	srv.ServeHTTP(w2, req2)
	require.Equal(t, http.StatusOK, w2.Code)

	assert.Equal(t, 2, callCount, "different messages should trigger separate upstream calls")
}

func strPtr(s string) *string { return &s }
