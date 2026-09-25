package passthrough

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/praxisllmlab/tianjiLLM/internal/callback"
	"github.com/praxisllmlab/tianjiLLM/internal/proxy/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// captureLogger captures LogSuccess calls for test assertions.
type captureLogger struct {
	mu   sync.Mutex
	logs []callback.LogData
}

func (c *captureLogger) LogSuccess(data callback.LogData) {
	c.mu.Lock()
	c.logs = append(c.logs, data)
	c.mu.Unlock()
}
func (c *captureLogger) LogFailure(callback.LogData) {}

func (c *captureLogger) snapshot() []callback.LogData {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]callback.LogData, len(c.logs))
	copy(out, c.logs)
	return out
}

// waitForLog blocks until at least one log is captured or timeout.
func waitForLog(c *captureLogger, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if len(c.snapshot()) > 0 {
			return true
		}
		time.Sleep(5 * time.Millisecond)
	}
	return false
}

func newTestRegistry(spy *captureLogger) *callback.Registry {
	r := callback.NewRegistry()
	r.Register(spy)
	return r
}

// T012: non-streaming response triggers LogSuccess with correct tokens
func TestRouter_NonStreaming_CallsLogSuccess(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"model":"claude-sonnet-4-6","usage":{"input_tokens":100,"output_tokens":50}}`)
	}))
	defer upstream.Close()

	spy := &captureLogger{}
	reg := newTestRegistry(spy)

	router := NewRouter([]Endpoint{{Path: "/v1/anthropic", Target: upstream.URL, Provider: "anthropic", APIKey: "[REDACTED]"}}, nil, reg)
	router.RegisterLogger("anthropic", &AnthropicLoggingHandler{})

	req := httptest.NewRequest(http.MethodPost, "/v1/anthropic/v1/messages", strings.NewReader(`{"model":"claude-sonnet-4-6","messages":[]}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.Handler().ServeHTTP(w, req)

	require.True(t, waitForLog(spy, 500*time.Millisecond), "LogSuccess not called")
	logs := spy.snapshot()
	require.Len(t, logs, 1)
	assert.Equal(t, "claude-sonnet-4-6", logs[0].Model)
	assert.Equal(t, 100, logs[0].PromptTokens)
	assert.Equal(t, 50, logs[0].CompletionTokens)
	assert.Equal(t, "anthropic", logs[0].Provider)
}

// T012: streaming response triggers LogSuccess after stream ends
func TestRouter_Streaming_CallsLogSuccessAfterStreamEnd(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "event: message_start\n")
		fmt.Fprintf(w, "data: %s\n\n", `{"type":"message_start","message":{"model":"claude-sonnet-4-6","usage":{"input_tokens":80,"cache_read_input_tokens":0,"cache_creation_input_tokens":0}}}`)
		fmt.Fprint(w, "event: message_delta\n")
		fmt.Fprintf(w, "data: %s\n\n", `{"type":"message_delta","usage":{"output_tokens":30}}`)
	}))
	defer upstream.Close()

	spy := &captureLogger{}
	reg := newTestRegistry(spy)

	router := NewRouter([]Endpoint{{Path: "/v1/anthropic", Target: upstream.URL, Provider: "anthropic", APIKey: "[REDACTED]"}}, nil, reg)
	router.RegisterLogger("anthropic", &AnthropicLoggingHandler{})

	req := httptest.NewRequest(http.MethodPost, "/v1/anthropic/v1/messages", strings.NewReader(`{"model":"claude-sonnet-4-6","messages":[],"stream":true}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.Handler().ServeHTTP(w, req)
	// Drain body to ensure stream Close() fires
	_, _ = io.ReadAll(w.Body)

	require.True(t, waitForLog(spy, 500*time.Millisecond), "LogSuccess not called after stream end")
	logs := spy.snapshot()
	require.Len(t, logs, 1)
	assert.Equal(t, 80, logs[0].PromptTokens)
	assert.Equal(t, 30, logs[0].CompletionTokens)
}

// T012: cache tokens flow through to LogData
func TestRouter_NonStreaming_CacheTokensLogged(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"model":"claude-sonnet-4-6","usage":{"input_tokens":60,"output_tokens":20,"cache_read_input_tokens":40,"cache_creation_input_tokens":10}}`)
	}))
	defer upstream.Close()

	spy := &captureLogger{}
	router := NewRouter([]Endpoint{{Path: "/v1/anthropic", Target: upstream.URL, Provider: "anthropic", APIKey: "[REDACTED]"}}, nil, newTestRegistry(spy))
	router.RegisterLogger("anthropic", &AnthropicLoggingHandler{})

	req := httptest.NewRequest(http.MethodPost, "/v1/anthropic/v1/messages", strings.NewReader(`{}`))
	w := httptest.NewRecorder()
	router.Handler().ServeHTTP(w, req)

	require.True(t, waitForLog(spy, 500*time.Millisecond))
	logs := spy.snapshot()
	// prompt = 60 + 40 + 10 = 110
	assert.Equal(t, 110, logs[0].PromptTokens)
	assert.Equal(t, 40, logs[0].CacheReadInputTokens)
	assert.Equal(t, 10, logs[0].CacheCreationInputTokens)
}

// T012: nil callbacks — no panic
func TestRouter_NoCallbacks_DoesNotPanic(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"usage":{"input_tokens":10,"output_tokens":5}}`)
	}))
	defer upstream.Close()

	router := NewRouter([]Endpoint{{Path: "/v1/anthropic", Target: upstream.URL, Provider: "anthropic", APIKey: "[REDACTED]"}}, nil, nil)
	router.RegisterLogger("anthropic", &AnthropicLoggingHandler{})

	req := httptest.NewRequest(http.MethodPost, "/v1/anthropic/v1/messages", strings.NewReader(`{}`))
	w := httptest.NewRecorder()
	assert.NotPanics(t, func() { router.Handler().ServeHTTP(w, req) })
}

// T012: non-200 upstream — LogSuccess NOT called
func TestRouter_NonOK_DoesNotCallLogSuccess(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		fmt.Fprint(w, `{"error":"rate limited"}`)
	}))
	defer upstream.Close()

	spy := &captureLogger{}
	router := NewRouter([]Endpoint{{Path: "/v1/anthropic", Target: upstream.URL, Provider: "anthropic", APIKey: "[REDACTED]"}}, nil, newTestRegistry(spy))
	router.RegisterLogger("anthropic", &AnthropicLoggingHandler{})

	req := httptest.NewRequest(http.MethodPost, "/v1/anthropic/v1/messages", strings.NewReader(`{}`))
	w := httptest.NewRecorder()
	router.Handler().ServeHTTP(w, req)

	time.Sleep(50 * time.Millisecond)
	assert.Empty(t, spy.snapshot(), "LogSuccess must not be called on non-200")
}

// T012: context values flow through to LogData
func TestRouter_LogData_HasContextValues(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"model":"claude-sonnet-4-6","usage":{"input_tokens":10,"output_tokens":5}}`)
	}))
	defer upstream.Close()

	spy := &captureLogger{}
	router := NewRouter([]Endpoint{{Path: "/v1/anthropic", Target: upstream.URL, Provider: "anthropic", APIKey: "[REDACTED]"}}, nil, newTestRegistry(spy))
	router.RegisterLogger("anthropic", &AnthropicLoggingHandler{})

	req := httptest.NewRequest(http.MethodPost, "/v1/anthropic/v1/messages", strings.NewReader(`{}`))
	ctx := context.WithValue(req.Context(), middleware.ContextKeyTokenHash, "abc123hash")
	ctx = context.WithValue(ctx, middleware.ContextKeyTeamID, "team-xyz")
	ctx = context.WithValue(ctx, middleware.ContextKeyRequesterIP, "1.2.3.4")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	router.Handler().ServeHTTP(w, req)

	require.True(t, waitForLog(spy, 500*time.Millisecond))
	logs := spy.snapshot()
	assert.Equal(t, "abc123hash", logs[0].APIKey)
	assert.Equal(t, "team-xyz", logs[0].TeamID)
	assert.Equal(t, "1.2.3.4", logs[0].RequesterIPAddress)
}

// T012: EndTime is after StartTime
func TestRouter_LogData_HasDuration(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"model":"claude-sonnet-4-6","usage":{"input_tokens":10,"output_tokens":5}}`)
	}))
	defer upstream.Close()

	spy := &captureLogger{}
	router := NewRouter([]Endpoint{{Path: "/v1/anthropic", Target: upstream.URL, Provider: "anthropic", APIKey: "[REDACTED]"}}, nil, newTestRegistry(spy))
	router.RegisterLogger("anthropic", &AnthropicLoggingHandler{})

	req := httptest.NewRequest(http.MethodPost, "/v1/anthropic/v1/messages", strings.NewReader(`{}`))
	w := httptest.NewRecorder()
	router.Handler().ServeHTTP(w, req)

	require.True(t, waitForLog(spy, 500*time.Millisecond))
	logs := spy.snapshot()
	assert.True(t, logs[0].EndTime.After(logs[0].StartTime), "EndTime should be after StartTime")
}

// TestRouter_ProviderAuthIsSanitized ensures caller credentials never cross the proxy boundary.
func TestRouter_ProviderAuthIsSanitized(t *testing.T) {
	headersCh := make(chan http.Header, 1)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		headersCh <- r.Header.Clone()
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	router := NewRouter([]Endpoint{{
		Path:     "/v1/anthropic",
		Target:   upstream.URL,
		Provider: "anthropic",
		APIKey:   "[REDACTED]",
	}}, nil, nil)
	req := httptest.NewRequest(http.MethodPost, "/v1/anthropic/v1/messages", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer [REDACTED]")
	req.Header.Set("api-key", "[REDACTED]")
	req.Header.Set("x-api-key", "[REDACTED]")
	req.Header.Set("Proxy-Authorization", "Basic [REDACTED]")

	w := httptest.NewRecorder()
	router.Handler().ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	got := <-headersCh
	require.Equal(t, "[REDACTED]", got.Get("x-api-key"))
	assert.Empty(t, got.Get("Authorization"))
	assert.Empty(t, got.Get("api-key"))
	assert.Empty(t, got.Get("Proxy-Authorization"))
}

func TestRouter_RejectsMissingProviderCredential(t *testing.T) {
	var upstreamCalled atomic.Bool
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalled.Store(true)
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	router := NewRouter([]Endpoint{{
		Path:     "/v1/anthropic",
		Target:   upstream.URL,
		Provider: "anthropic",
	}}, nil, nil)
	req := httptest.NewRequest(http.MethodPost, "/v1/anthropic/v1/messages", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer [REDACTED]")

	w := httptest.NewRecorder()
	router.Handler().ServeHTTP(w, req)

	require.Equal(t, http.StatusUnauthorized, w.Code)
	require.False(t, upstreamCalled.Load())
}

func TestHandler_ProviderAuthIsSanitized(t *testing.T) {
	headersCh := make(chan http.Header, 1)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		headersCh <- r.Header.Clone()
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	handler := Handler(Config{
		ProviderEndpoints: map[string]string{"/anthropic": upstream.URL},
		APIKeys:           map[string]string{"anthropic": "[REDACTED]"},
	})
	req := httptest.NewRequest(http.MethodPost, "/anthropic/v1/messages", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer [REDACTED]")
	req.Header.Set("api-key", "[REDACTED]")
	req.Header.Set("x-api-key", "[REDACTED]")
	req.Header.Set("Proxy-Authorization", "Basic [REDACTED]")

	w := httptest.NewRecorder()
	handler(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	got := <-headersCh
	require.Equal(t, "[REDACTED]", got.Get("x-api-key"))
	assert.Empty(t, got.Get("Authorization"))
	assert.Empty(t, got.Get("api-key"))
	assert.Empty(t, got.Get("Proxy-Authorization"))
}

func TestHandler_RejectsMissingProviderCredential(t *testing.T) {
	var upstreamCalled atomic.Bool
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalled.Store(true)
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	handler := Handler(Config{ProviderEndpoints: map[string]string{"/anthropic": upstream.URL}})
	req := httptest.NewRequest(http.MethodPost, "/anthropic/v1/messages", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer [REDACTED]")

	w := httptest.NewRecorder()
	handler(w, req)

	require.Equal(t, http.StatusUnauthorized, w.Code)
	require.False(t, upstreamCalled.Load())
}
