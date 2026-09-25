package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/praxisllmlab/tianjiLLM/internal/callback"
	"github.com/praxisllmlab/tianjiLLM/internal/config"
	handler "github.com/praxisllmlab/tianjiLLM/internal/proxy/handler"
)

func TestNativeProxyStreamingSpendLog(t *testing.T) {
	// 1. Fake Anthropic upstream that returns SSE
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
		w.WriteHeader(200)
		flusher, _ := w.(http.Flusher)

		events := []string{
			"event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"model\":\"claude-opus-4-6\",\"id\":\"msg_test\",\"type\":\"message\",\"role\":\"assistant\",\"content\":[],\"stop_reason\":null,\"usage\":{\"input_tokens\":25,\"output_tokens\":1}}}\n\n",
			"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"Hello\"}}\n\n",
			"event: message_delta\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"output_tokens\":10}}\n\n",
			"event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n",
		}

		for _, e := range events {
			fmt.Fprint(w, e)
			flusher.Flush()
			time.Sleep(5 * time.Millisecond)
		}
	}))
	defer upstream.Close()

	// 2. Capture what LogSuccess receives
	var gotLogData callback.LogData
	done := make(chan struct{})

	registry := callback.NewRegistry()
	registry.Register(&captureLogger{
		onSuccess: func(data callback.LogData) {
			gotLogData = data
			close(done)
		},
	})

	// 3. Create handler with upstream pointing to fake Anthropic
	apiKey := "test-key"
	cfg := &config.ProxyConfig{
		ModelList: []config.ModelConfig{
			{
				ModelName: "claude-*",
				TianjiParams: config.TianjiParams{
					Model:   "anthropic/claude-*",
					APIKey:  &apiKey,
					APIBase: strPtr(upstream.URL),
				},
			},
		},
	}

	h := &handler.Handlers{
		Config:    cfg,
		Callbacks: registry,
	}

	// 4. Send streaming request
	payload := map[string]any{
		"model":      "claude-opus-4-6",
		"max_tokens": 10,
		"stream":     true,
		"messages":   []map[string]string{{"role": "user", "content": "test"}},
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/v1/messages", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("anthropic-version", "2023-06-01")

	w := httptest.NewRecorder()
	h.AnthropicMessages(w, req)

	// 5. Wait for async callback
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for LogSuccess callback")
	}

	// 6. Verify
	t.Logf("Response status: %d", w.Code)
	t.Logf("Response body len: %d", w.Body.Len())
	t.Logf("LogData: model=%s prompt=%d completion=%d total=%d cost=%.4f",
		gotLogData.Model, gotLogData.PromptTokens, gotLogData.CompletionTokens,
		gotLogData.TotalTokens, gotLogData.Cost)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	if !strings.Contains(w.Header().Get("Content-Type"), "text/event-stream") {
		t.Fatalf("expected SSE content type, got %q", w.Header().Get("Content-Type"))
	}

	if gotLogData.PromptTokens != 25 {
		t.Errorf("expected prompt_tokens=25, got %d", gotLogData.PromptTokens)
	}
	if gotLogData.CompletionTokens != 10 {
		t.Errorf("expected completion_tokens=10, got %d", gotLogData.CompletionTokens)
	}
	if gotLogData.Model != "claude-opus-4-6" {
		t.Errorf("expected model=claude-opus-4-6, got %q", gotLogData.Model)
	}
}

type captureLogger struct {
	onSuccess func(callback.LogData)
}

func (c *captureLogger) Name() string                     { return "capture" }
func (c *captureLogger) LogSuccess(data callback.LogData) { c.onSuccess(data) }
func (c *captureLogger) LogFailure(data callback.LogData) {}
