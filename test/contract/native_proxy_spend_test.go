package contract

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/praxisllmlab/tianjiLLM/internal/callback"
	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/praxisllmlab/tianjiLLM/internal/proxy"
	"github.com/praxisllmlab/tianjiLLM/internal/proxy/handler"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// spyLogger captures LogData from callback invocations.
type spyLogger struct {
	mu   sync.Mutex
	logs []callback.LogData
	done chan struct{}
}

func newSpyLogger() *spyLogger {
	return &spyLogger{done: make(chan struct{}, 1)}
}

func (s *spyLogger) LogSuccess(data callback.LogData) {
	s.mu.Lock()
	s.logs = append(s.logs, data)
	s.mu.Unlock()
	select {
	case s.done <- struct{}{}:
	default:
	}
}

func (s *spyLogger) LogFailure(data callback.LogData) {}

func (s *spyLogger) wait(t *testing.T, timeout time.Duration) callback.LogData {
	t.Helper()
	select {
	case <-s.done:
	case <-time.After(timeout):
		t.Fatal("timed out waiting for LogSuccess callback")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	require.Len(t, s.logs, 1)
	return s.logs[0]
}

func TestNativeProxy_AnthropicSpendLog(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Contains(t, r.URL.Path, "/v1/messages")
		assert.Equal(t, "test-api-key", r.Header.Get("x-api-key"))

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":    "msg_test123",
			"type":  "message",
			"role":  "assistant",
			"model": "claude-sonnet-4-20250514",
			"content": []map[string]any{
				{"type": "text", "text": "Hello!"},
			},
			"stop_reason": "end_turn",
			"usage": map[string]any{
				"input_tokens":  42,
				"output_tokens": 17,
			},
		})
	}))
	defer upstream.Close()

	spy := newSpyLogger()
	registry := callback.NewRegistry()
	registry.Register(spy)

	apiKey := "test-api-key"
	cfg := &config.ProxyConfig{
		ModelList: []config.ModelConfig{
			{
				ModelName: "claude-sonnet",
				TianjiParams: config.TianjiParams{
					Model:   "anthropic/claude-sonnet-4-20250514",
					APIKey:  &apiKey,
					APIBase: &upstream.URL,
				},
			},
		},
		GeneralSettings: config.GeneralSettings{
			MasterKey: "sk-master",
		},
	}

	handlers := &handler.Handlers{
		Config:    cfg,
		Callbacks: registry,
	}

	srv := proxy.NewServer(proxy.ServerConfig{
		Handlers:  handlers,
		MasterKey: cfg.GeneralSettings.MasterKey,
	})

	body := `{
		"model": "claude-sonnet-4-20250514",
		"max_tokens": 100,
		"messages": [{"role": "user", "content": "Hi"}]
	}`
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer sk-master")

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	// Verify the response body is still intact (not consumed by ModifyResponse)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "msg_test123", resp["id"])

	// Wait for async callback
	data := spy.wait(t, 2*time.Second)

	assert.Equal(t, "anthropic", data.Provider)
	assert.Equal(t, "claude-sonnet-4-20250514", data.Model)
	assert.Equal(t, 42, data.PromptTokens)
	assert.Equal(t, 17, data.CompletionTokens)
	assert.Equal(t, 59, data.TotalTokens)
	assert.True(t, data.Latency > 0, "latency should be positive")
}

func TestNativeProxy_StreamingSpendLog(t *testing.T) {
	// Simulate a real Anthropic streaming response with usage in message_delta.
	ssePayload := strings.Join([]string{
		"event: message_start",
		`data: {"type":"message_start","message":{"id":"msg_stream1","model":"claude-sonnet-4-20250514","role":"assistant","usage":{"input_tokens":25,"output_tokens":0}}}`,
		"",
		"event: content_block_start",
		`data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
		"",
		"event: content_block_delta",
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hi!"}}`,
		"",
		"event: content_block_stop",
		`data: {"type":"content_block_stop","index":0}`,
		"",
		"event: message_delta",
		`data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":8}}`,
		"",
		"event: message_stop",
		`data: {"type":"message_stop"}`,
		"",
	}, "\n")

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(ssePayload))
	}))
	defer upstream.Close()

	spy := newSpyLogger()
	registry := callback.NewRegistry()
	registry.Register(spy)

	apiKey := "test-api-key"
	cfg := &config.ProxyConfig{
		ModelList: []config.ModelConfig{
			{
				ModelName: "claude-sonnet",
				TianjiParams: config.TianjiParams{
					Model:   "anthropic/claude-sonnet-4-20250514",
					APIKey:  &apiKey,
					APIBase: &upstream.URL,
				},
			},
		},
		GeneralSettings: config.GeneralSettings{
			MasterKey: "sk-master",
		},
	}

	handlers := &handler.Handlers{
		Config:    cfg,
		Callbacks: registry,
	}

	srv := proxy.NewServer(proxy.ServerConfig{
		Handlers:  handlers,
		MasterKey: cfg.GeneralSettings.MasterKey,
	})

	body := `{
		"model": "claude-sonnet-4-20250514",
		"max_tokens": 100,
		"stream": true,
		"messages": [{"role": "user", "content": "Hi"}]
	}`
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer sk-master")

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	// Verify SSE body is passed through intact
	assert.Contains(t, w.Body.String(), "message_start")
	assert.Contains(t, w.Body.String(), "Hi!")

	// Wait for async callback from sseSpendReader.Close()
	data := spy.wait(t, 2*time.Second)

	assert.Equal(t, "anthropic", data.Provider)
	assert.Equal(t, "claude-sonnet-4-20250514", data.Model)
	assert.Equal(t, 25, data.PromptTokens)
	assert.Equal(t, 8, data.CompletionTokens)
	assert.Equal(t, 33, data.TotalTokens)
	assert.True(t, data.Latency > 0)
}

func TestNativeProxy_StreamingNoUsage_StillLogsZeroTokens(t *testing.T) {
	// Stream with no usage in any event — callback should still fire with zero tokens.
	ssePayload := "event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_x\",\"model\":\"claude-sonnet-4-20250514\"}}\n\n"

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(ssePayload))
	}))
	defer upstream.Close()

	spy := newSpyLogger()
	registry := callback.NewRegistry()
	registry.Register(spy)

	apiKey := "test-api-key"
	cfg := &config.ProxyConfig{
		ModelList: []config.ModelConfig{
			{
				ModelName: "claude-sonnet",
				TianjiParams: config.TianjiParams{
					Model:   "anthropic/claude-sonnet-4-20250514",
					APIKey:  &apiKey,
					APIBase: &upstream.URL,
				},
			},
		},
		GeneralSettings: config.GeneralSettings{
			MasterKey: "sk-master",
		},
	}

	handlers := &handler.Handlers{
		Config:    cfg,
		Callbacks: registry,
	}

	srv := proxy.NewServer(proxy.ServerConfig{
		Handlers:  handlers,
		MasterKey: cfg.GeneralSettings.MasterKey,
	})

	body := `{
		"model": "claude-sonnet-4-20250514",
		"max_tokens": 100,
		"stream": true,
		"messages": [{"role": "user", "content": "Hi"}]
	}`
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer sk-master")

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	// Wait for async callback — even zero tokens should trigger a log
	data := spy.wait(t, 2*time.Second)

	assert.Equal(t, "anthropic", data.Provider)
	assert.Equal(t, "claude-sonnet-4-20250514", data.Model)
	assert.Equal(t, 0, data.PromptTokens)
	assert.Equal(t, 0, data.CompletionTokens)
	assert.Equal(t, 0, data.TotalTokens)
}

func TestNativeProxy_ErrorResponseSkipsCallback(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"type":"error","error":{"type":"invalid_request_error","message":"bad request"}}`))
	}))
	defer upstream.Close()

	spy := newSpyLogger()
	registry := callback.NewRegistry()
	registry.Register(spy)

	apiKey := "test-api-key"
	cfg := &config.ProxyConfig{
		ModelList: []config.ModelConfig{
			{
				ModelName: "claude-sonnet",
				TianjiParams: config.TianjiParams{
					Model:   "anthropic/claude-sonnet-4-20250514",
					APIKey:  &apiKey,
					APIBase: &upstream.URL,
				},
			},
		},
		GeneralSettings: config.GeneralSettings{
			MasterKey: "sk-master",
		},
	}

	handlers := &handler.Handlers{
		Config:    cfg,
		Callbacks: registry,
	}

	srv := proxy.NewServer(proxy.ServerConfig{
		Handlers:  handlers,
		MasterKey: cfg.GeneralSettings.MasterKey,
	})

	body := `{
		"model": "claude-sonnet-4-20250514",
		"max_tokens": 100,
		"messages": [{"role": "user", "content": "Hi"}]
	}`
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer sk-master")

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	time.Sleep(100 * time.Millisecond)

	spy.mu.Lock()
	count := len(spy.logs)
	spy.mu.Unlock()
	assert.Equal(t, 0, count, "error response should not trigger spend callback")
}
