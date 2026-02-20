package integration

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/praxisllmlab/tianjiLLM/internal/proxy"
	"github.com/praxisllmlab/tianjiLLM/internal/proxy/handler"
	"github.com/praxisllmlab/tianjiLLM/internal/proxy/middleware"
	"github.com/praxisllmlab/tianjiLLM/internal/router"
	"github.com/praxisllmlab/tianjiLLM/internal/router/strategy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPhase6_ResponseSecurity_EncryptDecryptRoundTrip(t *testing.T) {
	sec := middleware.NewResponseIDSecurity("test-key-for-phase6")

	// Encrypt for user-1/team-1
	encrypted := sec.EncryptID("resp-abc", "user-1", "team-1")
	assert.Contains(t, encrypted, "resp-abc.", "encrypted ID should contain original ID")

	// Decrypt with same user/team succeeds
	orig, valid := sec.DecryptID(encrypted, "user-1", "team-1")
	assert.True(t, valid)
	assert.Equal(t, "resp-abc", orig)

	// Decrypt with different user fails
	_, valid = sec.DecryptID(encrypted, "user-2", "team-1")
	assert.False(t, valid, "cross-user should be rejected")
}

func TestPhase6_CacheControlMiddleware_PassThrough(t *testing.T) {
	apiKey := "sk-test"
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "chatcmpl-cc", "object": "chat.completion", "model": "gpt-4",
			"choices": []map[string]any{{"index": 0, "message": map[string]any{"role": "assistant", "content": "ok"}}},
			"usage":   map[string]any{"prompt_tokens": 5, "completion_tokens": 3, "total_tokens": 8},
		})
	}))
	defer upstream.Close()

	cfg := &config.ProxyConfig{
		ModelList: []config.ModelConfig{{
			ModelName:    "gpt-4",
			TianjiParams: config.TianjiParams{Model: "openai/gpt-4", APIKey: &apiKey, APIBase: &upstream.URL},
		}},
		GeneralSettings: config.GeneralSettings{MasterKey: "sk-master"},
	}

	rtr := router.New(cfg.ModelList, strategy.NewShuffle(), router.RouterSettings{})
	handlers := &handler.Handlers{Config: cfg, Router: rtr}
	srv := proxy.NewServer(proxy.ServerConfig{
		Handlers:  handlers,
		MasterKey: "sk-master",
	})

	// Request without cache control — should pass through fine
	body := `{"model":"gpt-4","messages":[{"role":"user","content":"test"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer sk-master")

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "chatcmpl-cc", resp["id"])
}

func TestPhase6_ParallelLimiter_NilRedisAllows(t *testing.T) {
	// Without Redis, parallel limiter should be a no-op passthrough
	apiKey := "sk-test"
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "chatcmpl-pl", "object": "chat.completion", "model": "gpt-4",
			"choices": []map[string]any{{"index": 0, "message": map[string]any{"role": "assistant", "content": "parallel ok"}}},
			"usage":   map[string]any{"prompt_tokens": 5, "completion_tokens": 3, "total_tokens": 8},
		})
	}))
	defer upstream.Close()

	cfg := &config.ProxyConfig{
		ModelList: []config.ModelConfig{{
			ModelName:    "gpt-4",
			TianjiParams: config.TianjiParams{Model: "openai/gpt-4", APIKey: &apiKey, APIBase: &upstream.URL},
		}},
		GeneralSettings: config.GeneralSettings{MasterKey: "sk-master"},
	}

	rtr := router.New(cfg.ModelList, strategy.NewShuffle(), router.RouterSettings{})
	handlers := &handler.Handlers{Config: cfg, Router: rtr}
	// No RedisClient → parallel limiter is nil → passthrough
	srv := proxy.NewServer(proxy.ServerConfig{
		Handlers:  handlers,
		MasterKey: "sk-master",
	})

	body := `{"model":"gpt-4","messages":[{"role":"user","content":"test"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer sk-master")

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
