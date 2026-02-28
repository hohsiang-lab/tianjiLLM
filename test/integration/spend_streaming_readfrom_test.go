package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/praxisllmlab/tianjiLLM/internal/callback"
	"github.com/praxisllmlab/tianjiLLM/internal/config"
	handler "github.com/praxisllmlab/tianjiLLM/internal/proxy/handler"
)

// TestNativeProxyStreamingSpendLog_ReadFromBypass is a regression test for HO-60.
//
// Root cause: chi's WrapResponseWriter implements io.ReaderFrom. When
// httputil.ReverseProxy streams via io.Copy, it calls dst.ReadFrom(src)
// instead of src.Read()/dst.Write(), bypassing sseSpendReader.Read()
// entirely and leaving the tee buffer empty → tokens=0.
//
// This test reproduces the bug by using a real TCP server with chi
// middleware (which wraps ResponseWriter with io.ReaderFrom support),
// unlike httptest.NewRecorder which does NOT implement io.ReaderFrom.
func TestNativeProxyStreamingSpendLog_ReadFromBypass(t *testing.T) {
	// 1. Fake Anthropic upstream returning SSE
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

	// 2. Capture callback
	var gotLogData callback.LogData
	done := make(chan struct{})

	registry := callback.NewRegistry()
	registry.Register(&captureLogger{
		onSuccess: func(data callback.LogData) {
			gotLogData = data
			close(done)
		},
	})

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

	// 3. Real TCP server with chi middleware (the key difference from NewRecorder)
	//    chi's Logger middleware wraps ResponseWriter with WrapResponseWriter,
	//    which implements io.ReaderFrom → triggers the ReadFrom bypass in io.Copy.
	r := chi.NewRouter()
	r.Use(chiMiddleware.Recoverer)
	r.Use(chiMiddleware.Logger) // This wraps ResponseWriter with io.ReaderFrom!
	r.Post("/v1/messages", h.AnthropicMessages)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := &http.Server{Handler: r}
	go func() { _ = server.Serve(listener) }()
	defer server.Close()

	addr := listener.Addr().String()

	// 4. Real HTTP client request (not httptest.NewRequest)
	payload := map[string]any{
		"model":      "claude-opus-4-6",
		"max_tokens": 10,
		"stream":     true,
		"messages":   []map[string]string{{"role": "user", "content": "test"}},
	}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", "http://"+addr+"/v1/messages", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	respBody, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	// 5. Wait for async callback
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for LogSuccess callback")
	}

	// 6. Verify — without the readCloserOnly fix, tokens would be 0
	t.Logf("Response status: %d", resp.StatusCode)
	t.Logf("Response body len: %d", len(respBody))
	t.Logf("LogData: model=%s prompt=%d completion=%d total=%d cost=%.4f",
		gotLogData.Model, gotLogData.PromptTokens, gotLogData.CompletionTokens,
		gotLogData.TotalTokens, gotLogData.Cost)

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, string(respBody))
	}

	if !strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream") {
		t.Fatalf("expected SSE content type, got %q", resp.Header.Get("Content-Type"))
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
