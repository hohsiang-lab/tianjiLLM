package integration

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/praxisllmlab/tianjiLLM/internal/callback"
	"github.com/praxisllmlab/tianjiLLM/internal/config"
	_ "github.com/praxisllmlab/tianjiLLM/internal/provider/openai"
	"github.com/praxisllmlab/tianjiLLM/internal/proxy"
	"github.com/praxisllmlab/tianjiLLM/internal/proxy/handler"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCallback_WebhookFiresOnSuccess(t *testing.T) {
	var mu sync.Mutex
	var received []map[string]any

	webhookSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var payload map[string]any
		_ = json.Unmarshal(body, &payload)
		mu.Lock()
		received = append(received, payload)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer webhookSrv.Close()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":      "chatcmpl-test",
			"object":  "chat.completion",
			"created": 1234567890,
			"model":   "gpt-4o-mini",
			"choices": []map[string]any{
				{
					"index":         0,
					"message":       map[string]string{"role": "assistant", "content": "Hello!"},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]int{
				"prompt_tokens":     10,
				"completion_tokens": 5,
				"total_tokens":      15,
			},
		})
	}))
	defer upstream.Close()

	registry := callback.NewRegistry()
	registry.Register(callback.NewWebhookCallback(webhookSrv.URL, nil))

	apiKey := "sk-test"
	cfg := &config.ProxyConfig{
		ModelList: []config.ModelConfig{
			{
				ModelName: "gpt-4o-mini",
				TianjiParams: config.TianjiParams{
					Model:   "openai/gpt-4o-mini",
					APIKey:  &apiKey,
					APIBase: strPtr(upstream.URL),
				},
			},
		},
		GeneralSettings: config.GeneralSettings{
			MasterKey: "sk-master",
		},
	}

	srv := proxy.NewServer(proxy.ServerConfig{
		Handlers: &handler.Handlers{
			Config:    cfg,
			Callbacks: registry,
		},
		MasterKey: "sk-master",
	})

	reqBody := `{"model":"gpt-4o-mini","messages":[{"role":"user","content":"hi"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer sk-master")
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Wait for async callback
	time.Sleep(300 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	require.Len(t, received, 1, "webhook should have received 1 event")
	assert.Equal(t, "llm.success", received[0]["event"])
	assert.Equal(t, "gpt-4o-mini", received[0]["model"])
}

func TestCallback_RegistryNames(t *testing.T) {
	registry := callback.NewRegistry()
	registry.Register(callback.NewWebhookCallback("http://example.com", nil))
	registry.Register(callback.NewSlackCallback("http://example.com/slack", 0.8))

	names := registry.Names()
	assert.Len(t, names, 2)
	assert.Contains(t, names, "WebhookCallback")
	assert.Contains(t, names, "SlackCallback")
}

func strPtr(s string) *string { return &s }
