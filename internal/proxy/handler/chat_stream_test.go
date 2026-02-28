package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/praxisllmlab/tianjiLLM/internal/callback"
	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/praxisllmlab/tianjiLLM/internal/model"
	anthropicprovider "github.com/praxisllmlab/tianjiLLM/internal/provider/anthropic"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// logCapture is a minimal callback.CustomLogger for handler-package tests.
type logCapture struct {
	mu   sync.Mutex
	logs []callback.LogData
	done chan struct{}
}

func newLogCapture() *logCapture {
	return &logCapture{done: make(chan struct{}, 1)}
}

func (l *logCapture) LogSuccess(data callback.LogData) {
	l.mu.Lock()
	l.logs = append(l.logs, data)
	l.mu.Unlock()
	select {
	case l.done <- struct{}{}:
	default:
	}
}

func (l *logCapture) LogFailure(_ callback.LogData) {}

func (l *logCapture) wait(t *testing.T, timeout time.Duration) callback.LogData {
	t.Helper()
	select {
	case <-l.done:
	case <-time.After(timeout):
		t.Fatal("timed out waiting for LogSuccess callback")
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	require.Len(t, l.logs, 1)
	return l.logs[0]
}

// TestLogStreamSuccess_UsesAccumulatedUsage verifies that accUsage (prompt from
// message_start, completion from message_delta) is preferred over lastChunk.Usage.
func TestLogStreamSuccess_UsesAccumulatedUsage(t *testing.T) {
	t.Parallel()

	cap := newLogCapture()
	reg := callback.NewRegistry()
	reg.Register(cap)

	h := &Handlers{
		Config:    &config.ProxyConfig{},
		Callbacks: reg,
	}

	req := &model.ChatCompletionRequest{Model: "anthropic/claude-sonnet-4-5-20250929"}
	// lastChunk has zero usage (message_start role chunk — no usage before fix)
	lastChunk := &model.StreamChunk{Model: "claude-sonnet-4-5-20250929"}
	accUsage := model.Usage{PromptTokens: 30, CompletionTokens: 12}

	start := time.Now()
	end := start.Add(100 * time.Millisecond)
	h.logStreamSuccess(context.Background(), req, lastChunk, accUsage, nil, start, end, 50*time.Millisecond)

	data := cap.wait(t, 2*time.Second)
	assert.Equal(t, 30, data.PromptTokens)
	assert.Equal(t, 12, data.CompletionTokens)
	assert.Equal(t, 42, data.TotalTokens)
}

// TestLogStreamSuccess_FallsBackToLastChunk verifies that when accUsage is zero
// (providers that don't split usage across events), lastChunk.Usage is used.
func TestLogStreamSuccess_FallsBackToLastChunk(t *testing.T) {
	t.Parallel()

	cap := newLogCapture()
	reg := callback.NewRegistry()
	reg.Register(cap)

	h := &Handlers{
		Config:    &config.ProxyConfig{},
		Callbacks: reg,
	}

	req := &model.ChatCompletionRequest{Model: "openai/gpt-4o"}
	lastChunk := &model.StreamChunk{
		Usage: &model.Usage{PromptTokens: 20, CompletionTokens: 10, TotalTokens: 30},
	}
	accUsage := model.Usage{} // zero — nothing accumulated

	start := time.Now()
	end := start.Add(100 * time.Millisecond)
	h.logStreamSuccess(context.Background(), req, lastChunk, accUsage, nil, start, end, 50*time.Millisecond)

	data := cap.wait(t, 2*time.Second)
	assert.Equal(t, 20, data.PromptTokens)
	assert.Equal(t, 10, data.CompletionTokens)
	assert.Equal(t, 30, data.TotalTokens)
}

// TestHandleStreamingCompletion_AnthropicUsageAccumulated exercises the full
// streaming path with real Anthropic SSE format: input_tokens in message_start,
// output_tokens in message_delta. Verifies the callback receives both.
func TestHandleStreamingCompletion_AnthropicUsageAccumulated(t *testing.T) {
	t.Parallel()

	ssePayload := strings.Join([]string{
		`data: {"type":"message_start","message":{"id":"msg_t1","model":"claude-sonnet-4-5-20250929","usage":{"input_tokens":30}}}`,
		``,
		`data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
		``,
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello!"}}`,
		``,
		`data: {"type":"content_block_stop","index":0}`,
		``,
		`data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":12}}`,
		``,
		`data: {"type":"message_stop"}`,
		``,
	}, "\n")

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(ssePayload))
	}))
	defer upstream.Close()

	cap := newLogCapture()
	reg := callback.NewRegistry()
	reg.Register(cap)

	apiKey := "test-key"
	cfg := &config.ProxyConfig{
		ModelList: []config.ModelConfig{
			{
				ModelName: "claude-sonnet",
				TianjiParams: config.TianjiParams{
					Model:  "anthropic/claude-sonnet-4-5-20250929",
					APIKey: &apiKey,
				},
			},
		},
	}
	h := &Handlers{Config: cfg, Callbacks: reg}

	p := anthropicprovider.NewWithBaseURL(upstream.URL)
	req := &model.ChatCompletionRequest{
		Model:    "claude-sonnet-4-5-20250929",
		Messages: []model.Message{{Role: "user", Content: "hi"}},
		Stream:   boolPtr(true),
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	h.handleStreamingCompletion(w, r, p, req, apiKey)

	data := cap.wait(t, 2*time.Second)
	assert.Equal(t, 30, data.PromptTokens, "prompt tokens from message_start.message.usage")
	assert.Equal(t, 12, data.CompletionTokens, "completion tokens from message_delta.usage")
	assert.Equal(t, 42, data.TotalTokens)

	// Verify SSE content was forwarded to client
	assert.Contains(t, w.Body.String(), "Hello!")
}

func boolPtr(b bool) *bool { return &b }
