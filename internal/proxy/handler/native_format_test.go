package handler

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/praxisllmlab/tianjiLLM/internal/callback"
	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/praxisllmlab/tianjiLLM/internal/db"
	"github.com/praxisllmlab/tianjiLLM/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// spyDBTX implements db.DBTX, capturing all Exec calls for assertion.
type spyDBTX struct {
	mu     sync.Mutex
	calls  [][]interface{} // each element is the args slice from one Exec call
	execCh chan struct{}   // buffered; signaled on each Exec call
}

func newSpyDBTX() *spyDBTX {
	return &spyDBTX{execCh: make(chan struct{}, 8)}
}

func (s *spyDBTX) Exec(_ context.Context, _ string, args ...interface{}) (pgconn.CommandTag, error) {
	s.mu.Lock()
	s.calls = append(s.calls, args)
	s.mu.Unlock()
	select {
	case s.execCh <- struct{}{}:
	default:
	}
	return pgconn.CommandTag{}, nil
}

func (s *spyDBTX) Query(_ context.Context, _ string, _ ...interface{}) (pgx.Rows, error) {
	return nil, nil
}

func (s *spyDBTX) QueryRow(_ context.Context, _ string, _ ...interface{}) pgx.Row {
	return nil
}

// waitExec waits up to timeout for at least one Exec call; fails the test on timeout.
func (s *spyDBTX) waitExec(t *testing.T, timeout time.Duration) {
	t.Helper()
	select {
	case <-s.execCh:
	case <-time.After(timeout):
		t.Fatal("timed out waiting for InsertErrorLog Exec call")
	}
}

// callCount returns how many Exec calls have been recorded.
func (s *spyDBTX) callCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.calls)
}

// firstArgs returns the args from the first captured Exec call.
func (s *spyDBTX) firstArgs(t *testing.T) []interface{} {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	require.NotEmpty(t, s.calls, "expected at least one Exec call")
	return s.calls[0]
}

// nativeTestHandlers creates Handlers with the upstream URL wired into provider config.
// Pass dbtx=nil to leave h.DB as nil.
func nativeTestHandlers(upstreamURL, providerName string, dbtx *spyDBTX) *Handlers {
	apiKey := "test-key"
	cfg := &config.ProxyConfig{
		ModelList: []config.ModelConfig{
			{
				ModelName: providerName + "-test",
				TianjiParams: config.TianjiParams{
					Model:   providerName + "/some-model",
					APIKey:  &apiKey,
					APIBase: &upstreamURL,
				},
			},
		},
	}
	h := &Handlers{Config: cfg}
	if dbtx != nil {
		h.DB = db.New(dbtx)
	}
	return h
}

// ──────────────────────────────────────────────────────────────
// nativeProxy tests
// ──────────────────────────────────────────────────────────────

// TestNativeProxy_ErrorLog_Written verifies that InsertErrorLog is called when
// upstream responds with a non-200 status code.
func TestNativeProxy_ErrorLog_Written(t *testing.T) {
	t.Parallel()

	const errBody = `{"type":"error","error":{"type":"rate_limit_error","message":"too many requests"}}`
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(errBody))
	}))
	defer upstream.Close()

	spy := newSpyDBTX()
	h := nativeTestHandlers(upstream.URL, "anthropic", spy)

	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{}`))
	w := httptest.NewRecorder()
	h.nativeProxy(w, req, "anthropic")

	// InsertErrorLog is called in a goroutine; wait for it.
	spy.waitExec(t, 2*time.Second)

	// InsertErrorLog args order: requestID, apiKeyHash, model, provider,
	// statusCode, errorType, errorMessage, traceback  (8 total).
	args := spy.firstArgs(t)
	require.Len(t, args, 8, "InsertErrorLog must receive exactly 8 args")

	// Compare as interface{} — avoids errcheck on unchecked type assertions.
	assert.Equal(t, "anthropic", args[3], "provider")
	assert.Equal(t, int32(http.StatusTooManyRequests), args[4], "status_code")
	assert.Equal(t, "upstream_error", args[5], "error_type")
	assert.Contains(t, args[6], "too many requests", "error_message should contain upstream body")
}

// TestNativeProxy_ErrorLog_BodyRestored verifies that the response body is
// fully available to the client even after ReadAll was called in ModifyResponse.
func TestNativeProxy_ErrorLog_BodyRestored(t *testing.T) {
	t.Parallel()

	const errBody = `{"error":"invalid_api_key"}`
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(errBody))
	}))
	defer upstream.Close()

	spy := newSpyDBTX()
	h := nativeTestHandlers(upstream.URL, "anthropic", spy)

	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{}`))
	w := httptest.NewRecorder()
	h.nativeProxy(w, req, "anthropic")

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Equal(t, errBody, w.Body.String(),
		"response body must be fully restored after ReadAll in ModifyResponse")
}

// TestNativeProxy_NoErrorLog_On200 verifies that InsertErrorLog is NOT called
// when upstream responds with 200 OK.
func TestNativeProxy_NoErrorLog_On200(t *testing.T) {
	t.Parallel()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"msg_ok","type":"message"}`))
	}))
	defer upstream.Close()

	spy := newSpyDBTX()
	h := nativeTestHandlers(upstream.URL, "anthropic", spy)

	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{}`))
	w := httptest.NewRecorder()
	h.nativeProxy(w, req, "anthropic")

	// Allow any goroutine to run; none should.
	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, 0, spy.callCount(), "InsertErrorLog must not be called on 200 response")
}

// TestNativeProxy_NilDB_NoPanic verifies that nil h.DB does not cause a panic
// when the upstream returns an error response.
func TestNativeProxy_NilDB_NoPanic(t *testing.T) {
	t.Parallel()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"internal_server_error"}`))
	}))
	defer upstream.Close()

	h := nativeTestHandlers(upstream.URL, "anthropic", nil) // DB is nil

	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{}`))
	w := httptest.NewRecorder()

	require.NotPanics(t, func() {
		h.nativeProxy(w, req, "anthropic")
	}, "nativeProxy must not panic when h.DB is nil")
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// ──────────────────────────────────────────────────────────────
// recordErrorLog tests
// ──────────────────────────────────────────────────────────────

// TestRecordErrorLog_UsesChiRequestID verifies that recordErrorLog stores the
// chi RequestID from context — not a pointer address or other fallback value.
func TestRecordErrorLog_UsesChiRequestID(t *testing.T) {
	t.Parallel()

	spy := newSpyDBTX()
	h := &Handlers{
		Config: &config.ProxyConfig{},
		DB:     db.New(spy),
	}

	const wantID = "req-chi-abc123"
	ctx := context.WithValue(context.Background(), chiMiddleware.RequestIDKey, wantID)
	req := &model.ChatCompletionRequest{Model: "gpt-4"}

	h.recordErrorLog(ctx, req, nil, fmt.Errorf("upstream failure"))

	spy.waitExec(t, 2*time.Second)

	args := spy.firstArgs(t)
	require.Len(t, args, 8)
	// args[0] is RequestID; compare as interface{} to avoid errcheck on type assertions.
	assert.Equal(t, wantID, args[0],
		"RequestID must equal chi request ID, not a pointer address")
}

// TestRecordErrorLog_NoRequestID_EmptyString verifies that RequestID is the
// empty string when no chi RequestID is present in the context.
func TestRecordErrorLog_NoRequestID_EmptyString(t *testing.T) {
	t.Parallel()

	spy := newSpyDBTX()
	h := &Handlers{
		Config: &config.ProxyConfig{},
		DB:     db.New(spy),
	}

	ctx := context.Background() // no chi request ID
	req := &model.ChatCompletionRequest{Model: "gpt-4"}

	h.recordErrorLog(ctx, req, nil, fmt.Errorf("some error"))

	spy.waitExec(t, 2*time.Second)

	args := spy.firstArgs(t)
	require.Len(t, args, 8)
	assert.Equal(t, "", args[0],
		"RequestID must be empty string when chi request ID is absent from context")
}

// spyLogger implements callback.CustomLogger, recording LogSuccess calls.
type spyLogger struct {
	mu      sync.Mutex
	calls   []callback.LogData
	calledC chan struct{}
}

func newSpyLogger() *spyLogger {
	return &spyLogger{calledC: make(chan struct{}, 8)}
}

func (s *spyLogger) LogSuccess(data callback.LogData) {
	s.mu.Lock()
	s.calls = append(s.calls, data)
	s.mu.Unlock()
	select {
	case s.calledC <- struct{}{}:
	default:
	}
}

func (s *spyLogger) LogFailure(data callback.LogData) {}

func (s *spyLogger) waitCalled(t *testing.T, timeout time.Duration) {
	t.Helper()
	select {
	case <-s.calledC:
	case <-time.After(timeout):
		t.Fatal("timed out waiting for LogSuccess call")
	}
}

func (s *spyLogger) logCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.calls)
}

// TestNativeProxy_ZeroTokens_LogSuccessStillCalled verifies that LogSuccess is
// called even when prompt and completion tokens are both 0.
func TestNativeProxy_ZeroTokens_LogSuccessStillCalled(t *testing.T) {
	t.Parallel()

	// Upstream returns 200 with a body that yields 0 prompt and 0 completion tokens.
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"msg_ok","type":"message"}`))
	}))
	defer upstream.Close()

	spy := newSpyLogger()
	reg := callback.NewRegistry()
	reg.Register(spy)

	h := nativeTestHandlers(upstream.URL, "anthropic", nil)
	h.Callbacks = reg

	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{}`))
	w := httptest.NewRecorder()
	h.nativeProxy(w, req, "anthropic")

	spy.waitCalled(t, 2*time.Second)
	assert.Equal(t, 1, spy.logCount(), "LogSuccess must be called even with 0 tokens")
}

// TestNativeProxy_ZeroTokens_Streaming_LogSuccessStillCalled verifies that
// LogSuccess is called for streaming responses even when tokens are 0.
func TestNativeProxy_ZeroTokens_Streaming_LogSuccessStillCalled(t *testing.T) {
	t.Parallel()

	// Upstream returns SSE with no usage data → tokens parse as 0.
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("data: {\"type\":\"content_block_delta\",\"delta\":{\"text\":\"hi\"}}\n\n"))
	}))
	defer upstream.Close()

	spy := newSpyLogger()
	reg := callback.NewRegistry()
	reg.Register(spy)

	h := nativeTestHandlers(upstream.URL, "anthropic", nil)
	h.Callbacks = reg

	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{}`))
	w := httptest.NewRecorder()
	h.nativeProxy(w, req, "anthropic")

	// The streaming path calls LogSuccess in sseSpendReader.Close(),
	// which happens when the client reads the body to completion.
	// httptest.ResponseRecorder reads it all, so Close is triggered by the proxy.
	spy.waitCalled(t, 2*time.Second)
	assert.Equal(t, 1, spy.logCount(), "LogSuccess must be called even with 0 tokens in streaming")
}

func TestParseSSEUsage_Anthropic_InputTokens(t *testing.T) {
	// Anthropic message_start carries input_tokens inside message.usage,
	// and message_delta carries output_tokens in root usage.
	raw := []byte(
		`data: {"type":"message_start","message":{"model":"claude-sonnet-4-20250514","usage":{"input_tokens":42,"output_tokens":0}}}` + "\n" +
			`data: {"type":"content_block_delta","delta":{"text":"hi"}}` + "\n" +
			`data: {"type":"message_delta","usage":{"input_tokens":0,"output_tokens":15}}` + "\n",
	)
	prompt, completion, model := parseSSEUsage("anthropic", raw)
	assert.Equal(t, 42, prompt, "input_tokens should be parsed from message_start.message.usage")
	assert.Equal(t, 15, completion, "output_tokens should be parsed from message_delta.usage")
	assert.Equal(t, "claude-sonnet-4-20250514", model)
}

func TestParseSSEUsage_Gemini(t *testing.T) {
	// Gemini streaming: each chunk may have usageMetadata; last one wins.
	raw := []byte(
		`data: {"candidates":[{"content":{"parts":[{"text":"hello"}]}}],"modelVersion":"gemini-2.0-flash","usageMetadata":{"promptTokenCount":10,"candidatesTokenCount":5}}` + "\n" +
			`data: {"candidates":[{"content":{"parts":[{"text":" world"}]}}],"modelVersion":"gemini-2.0-flash","usageMetadata":{"promptTokenCount":10,"candidatesTokenCount":20}}` + "\n",
	)
	prompt, completion, model := parseSSEUsage("gemini", raw)
	assert.Equal(t, 10, prompt, "promptTokenCount from last chunk")
	assert.Equal(t, 20, completion, "candidatesTokenCount from last chunk")
	assert.Equal(t, "gemini-2.0-flash", model)
}

func TestParseSSEUsage_Anthropic_ZeroInputTokens_BeforeFix(t *testing.T) {
	// Ensures that even when root-level usage has 0 input_tokens,
	// we still get the value from message.usage in message_start.
	raw := []byte(
		`data: {"type":"message_start","message":{"model":"claude-sonnet-4-20250514","usage":{"input_tokens":100,"output_tokens":0}}}` + "\n",
	)
	prompt, _, _ := parseSSEUsage("anthropic", raw)
	assert.Equal(t, 100, prompt, "input_tokens must come from nested message.usage")
}

func TestParseSSEUsage_OpenAI(t *testing.T) {
	// OpenAI streaming: last chunk has usage with prompt_tokens/completion_tokens.
	raw := []byte(
		`data: {"id":"chatcmpl-abc","object":"chat.completion.chunk","model":"gpt-4o","choices":[{"delta":{"content":"hi"}}]}` + "\n" +
			`data: {"id":"chatcmpl-abc","object":"chat.completion.chunk","model":"gpt-4o","choices":[],"usage":{"prompt_tokens":10,"completion_tokens":25,"total_tokens":35}}` + "\n",
	)
	prompt, completion, model := parseSSEUsage("openai", raw)
	assert.Equal(t, 10, prompt, "prompt_tokens from usage chunk")
	assert.Equal(t, 25, completion, "completion_tokens from usage chunk")
	assert.Equal(t, "gpt-4o", model)
}

func TestParseSSEUsage_OpenAI_NoUsage(t *testing.T) {
	// When no usage chunk is present, should return zeros without panic.
	raw := []byte(
		`data: {"id":"chatcmpl-abc","object":"chat.completion.chunk","model":"gpt-4o","choices":[{"delta":{"content":"hi"}}]}` + "\n" +
			`data: [DONE]` + "\n",
	)
	prompt, completion, model := parseSSEUsage("openai", raw)
	assert.Equal(t, 0, prompt)
	assert.Equal(t, 0, completion)
	assert.Equal(t, "gpt-4o", model)
}

func TestParseUsage_OpenAI(t *testing.T) {
	body := []byte(`{"id":"chatcmpl-abc","model":"gpt-4o","usage":{"prompt_tokens":100,"completion_tokens":50,"total_tokens":150}}`)
	prompt, completion, model := parseUsage("openai", body)
	assert.Equal(t, 100, prompt)
	assert.Equal(t, 50, completion)
	assert.Equal(t, "gpt-4o", model)
}

func TestParseUsage_OpenRouter(t *testing.T) {
	body := []byte(`{"model":"meta-llama/llama-3-70b","usage":{"prompt_tokens":200,"completion_tokens":80}}`)
	prompt, completion, model := parseUsage("openrouter", body)
	assert.Equal(t, 200, prompt)
	assert.Equal(t, 80, completion)
	assert.Equal(t, "meta-llama/llama-3-70b", model)
}

func TestParseUsage_DeepSeek(t *testing.T) {
	body := []byte(`{"model":"deepseek-chat","usage":{"prompt_tokens":50,"completion_tokens":30}}`)
	prompt, completion, model := parseUsage("deepseek", body)
	assert.Equal(t, 50, prompt)
	assert.Equal(t, 30, completion)
	assert.Equal(t, "deepseek-chat", model)
}

func TestParseUsage_DefaultFallback(t *testing.T) {
	body := []byte(`{"model":"some-model","usage":{"prompt_tokens":10,"completion_tokens":5}}`)
	prompt, completion, model := parseUsage("unknown-provider", body)
	assert.Equal(t, 10, prompt)
	assert.Equal(t, 5, completion)
	assert.Equal(t, "some-model", model)
}

func TestParseUsage_DefaultFallback_ZeroTokens(t *testing.T) {
	// Default fallback requires at least one non-zero token count
	body := []byte(`{"model":"some-model","usage":{"prompt_tokens":0,"completion_tokens":0}}`)
	prompt, completion, model := parseUsage("unknown-provider", body)
	assert.Equal(t, 0, prompt)
	assert.Equal(t, 0, completion)
	assert.Equal(t, "", model)
}

func TestParseUsage_Anthropic(t *testing.T) {
	body := []byte(`{"model":"claude-sonnet-4-20250514","usage":{"input_tokens":42,"output_tokens":15}}`)
	prompt, completion, model := parseUsage("anthropic", body)
	assert.Equal(t, 42, prompt)
	assert.Equal(t, 15, completion)
	assert.Equal(t, "claude-sonnet-4-20250514", model)
}

func TestParseUsage_InvalidJSON(t *testing.T) {
	body := []byte(`not json`)
	prompt, completion, model := parseUsage("openai", body)
	assert.Equal(t, 0, prompt)
	assert.Equal(t, 0, completion)
	assert.Equal(t, "", model)
}
