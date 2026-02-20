package contract

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/praxisllmlab/tianjiLLM/internal/proxy"
	"github.com/praxisllmlab/tianjiLLM/internal/proxy/handler"
	"github.com/praxisllmlab/tianjiLLM/internal/router"
	"github.com/praxisllmlab/tianjiLLM/internal/router/strategy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFallback_ModelSpecific(t *testing.T) {
	apiKey := "sk-test"
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "chatcmpl-fb", "object": "chat.completion", "model": "claude-3",
			"choices": []map[string]any{{"index": 0, "message": map[string]any{"role": "assistant", "content": "fallback"}}},
			"usage":   map[string]any{"prompt_tokens": 5, "completion_tokens": 3, "total_tokens": 8},
		})
	}))
	defer upstream.Close()

	cfg := &config.ProxyConfig{
		ModelList: []config.ModelConfig{
			// Only the fallback model has deployments; primary model "missing-model" has none
			{
				ModelName:    "claude-3",
				TianjiParams: config.TianjiParams{Model: "openai/claude-3", APIKey: &apiKey, APIBase: &upstream.URL},
			},
		},
		GeneralSettings: config.GeneralSettings{MasterKey: "sk-master"},
	}

	rtr := router.New(cfg.ModelList, strategy.NewShuffle(), router.RouterSettings{
		NumRetries: 0,
		Fallbacks: map[string][]string{
			"missing-model": {"claude-3"},
		},
	})

	handlers := &handler.Handlers{Config: cfg, Router: rtr}
	srv := proxy.NewServer(proxy.ServerConfig{
		Handlers:  handlers,
		MasterKey: "sk-master",
	})

	// Request "missing-model" which has no deployments → Route fails → fallback to claude-3
	body := `{"model":"missing-model","messages":[{"role":"user","content":"test"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer sk-master")

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "chatcmpl-fb", resp["id"])
}

func TestFallback_DefaultFallbacks(t *testing.T) {
	apiKey := "sk-test"
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "chatcmpl-default", "object": "chat.completion", "model": "default-model",
			"choices": []map[string]any{{"index": 0, "message": map[string]any{"role": "assistant", "content": "default"}}},
			"usage":   map[string]any{"prompt_tokens": 5, "completion_tokens": 3, "total_tokens": 8},
		})
	}))
	defer upstream.Close()

	cfg := &config.ProxyConfig{
		ModelList: []config.ModelConfig{
			// Only fallback has deployments
			{
				ModelName:    "fallback-model",
				TianjiParams: config.TianjiParams{Model: "openai/fallback-model", APIKey: &apiKey, APIBase: &upstream.URL},
			},
		},
		GeneralSettings: config.GeneralSettings{MasterKey: "sk-master"},
	}

	rtr := router.New(cfg.ModelList, strategy.NewShuffle(), router.RouterSettings{
		NumRetries:       0,
		DefaultFallbacks: []string{"fallback-model"},
	})

	handlers := &handler.Handlers{Config: cfg, Router: rtr}
	srv := proxy.NewServer(proxy.ServerConfig{
		Handlers:  handlers,
		MasterKey: "sk-master",
	})

	// "nonexistent" has no deployments → Route fails → default fallback → fallback-model
	body := `{"model":"nonexistent","messages":[{"role":"user","content":"test"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer sk-master")

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestFallback_AllFail(t *testing.T) {
	apiKey := "sk-test"
	cfg := &config.ProxyConfig{
		ModelList: []config.ModelConfig{
			{
				ModelName:    "main",
				TianjiParams: config.TianjiParams{Model: "openai/main", APIKey: &apiKey},
			},
		},
		GeneralSettings: config.GeneralSettings{MasterKey: "sk-master"},
	}

	rtr := router.New(cfg.ModelList, strategy.NewShuffle(), router.RouterSettings{
		NumRetries: 0,
		Fallbacks: map[string][]string{
			"main": {"nonexistent"},
		},
	})

	handlers := &handler.Handlers{Config: cfg, Router: rtr}
	srv := proxy.NewServer(proxy.ServerConfig{
		Handlers:  handlers,
		MasterKey: "sk-master",
	})

	body := `{"model":"main","messages":[{"role":"user","content":"test"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer sk-master")

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	// Should fail since both main and fallback are unavailable
	assert.NotEqual(t, http.StatusOK, w.Code)
}
