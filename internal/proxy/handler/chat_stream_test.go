package handler

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/praxisllmlab/tianjiLLM/internal/cache"
	"github.com/praxisllmlab/tianjiLLM/internal/callback"
	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/praxisllmlab/tianjiLLM/internal/model"
	anthropicprovider "github.com/praxisllmlab/tianjiLLM/internal/provider/anthropic"
	openaiprovider "github.com/praxisllmlab/tianjiLLM/internal/provider/openai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func stringPtr(value string) *string { return &value }

func intPtr(value int) *int { return &value }

// logCapture is a minimal callback.CustomLogger for handler-package tests.
type logCapture struct {
	mu       sync.Mutex
	logs     []callback.LogData
	failures []callback.LogData
	done     chan struct{}
	failed   chan struct{}
}

func newLogCapture() *logCapture {
	return &logCapture{done: make(chan struct{}, 1), failed: make(chan struct{}, 1)}
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

func (l *logCapture) LogFailure(data callback.LogData) {
	l.mu.Lock()
	l.failures = append(l.failures, data)
	l.mu.Unlock()
	select {
	case l.failed <- struct{}{}:
	default:
	}
}

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

func (l *logCapture) waitFailure(t *testing.T, timeout time.Duration) callback.LogData {
	t.Helper()
	select {
	case <-l.failed:
	case <-time.After(timeout):
		t.Fatal("timed out waiting for LogFailure callback")
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	require.Len(t, l.failures, 1)
	assert.Empty(t, l.logs)
	return l.failures[0]
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
	h.logStreamSuccess(context.Background(), req, lastChunk, accUsage, nil, start, end, 50*time.Millisecond, 0)

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
	h.logStreamSuccess(context.Background(), req, lastChunk, accUsage, nil, start, end, 50*time.Millisecond, 0)

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

func TestStreamAggregatePreservesContentToolsRefusalFinishReasonAndUsage(t *testing.T) {
	var aggregate streamAggregate
	aggregate.Add(&model.StreamChunk{
		ID:      "chatcmpl-1",
		Created: 1700000000,
		Model:   "gpt-test",
		Choices: []model.StreamChoice{{
			Index: 0,
			Delta: model.Delta{
				Content: stringPtr("hel"),
				ToolCalls: []model.ToolCall{{
					Index: intPtr(1),
					ID:    "call-1",
					Type:  "function",
					Function: model.ToolCallFunction{
						Name:      "lookup",
						Arguments: `{"q":"`,
					},
				}},
			},
		}},
	})
	aggregate.Add(&model.StreamChunk{
		Choices: []model.StreamChoice{{
			Index: 0,
			Delta: model.Delta{
				Content: stringPtr("lo"),
				Refusal: stringPtr("no"),
				ToolCalls: []model.ToolCall{{
					Index: intPtr(1),
					Function: model.ToolCallFunction{
						Arguments: `x"}`,
					},
				}},
			},
			FinishReason: stringPtr("tool_calls"),
		}},
	})
	aggregate.Add(&model.StreamChunk{
		Choices: []model.StreamChoice{},
		Usage:   &model.Usage{PromptTokens: 7, CompletionTokens: 3, TotalTokens: 10},
	})

	response, err := aggregate.Response("fallback")
	require.NoError(t, err)
	assert.Equal(t, "hello", response.Choices[0].Message.Content)
	require.NotNil(t, response.Choices[0].Message.Refusal)
	assert.Equal(t, "no", *response.Choices[0].Message.Refusal)
	require.Len(t, response.Choices[0].Message.ToolCalls, 1)
	assert.Equal(t, `{"q":"x"}`, response.Choices[0].Message.ToolCalls[0].Function.Arguments)
	assert.Nil(t, response.Choices[0].Message.ToolCalls[0].Index)
	assert.Equal(t, "tool_calls", *response.Choices[0].FinishReason)
	assert.Equal(t, 10, response.Usage.TotalTokens)
}

func TestStreamAggregatePreservesMultipleChoices(t *testing.T) {
	var aggregate streamAggregate
	aggregate.Add(&model.StreamChunk{
		ID:    "chatcmpl-many",
		Model: "gpt-test",
		Choices: []model.StreamChoice{
			{Index: 1, Delta: model.Delta{Content: stringPtr("second")}},
			{Index: 0, Delta: model.Delta{Content: stringPtr("first")}},
		},
	})
	aggregate.Add(&model.StreamChunk{
		Choices: []model.StreamChoice{
			{Index: 1, FinishReason: stringPtr("length")},
			{Index: 0, FinishReason: stringPtr("stop")},
		},
	})
	aggregate.Complete()

	response, err := aggregate.Response("fallback")
	require.NoError(t, err)
	require.Len(t, response.Choices, 2)
	assert.Equal(t, 0, response.Choices[0].Index)
	assert.Equal(t, "first", response.Choices[0].Message.Content)
	assert.Equal(t, "stop", *response.Choices[0].FinishReason)
	assert.Equal(t, 1, response.Choices[1].Index)
	assert.Equal(t, "second", response.Choices[1].Message.Content)
	assert.Equal(t, "length", *response.Choices[1].FinishReason)
}

func TestStreamAggregateMergesUsageAcrossChunks(t *testing.T) {
	var aggregate streamAggregate
	aggregate.Add(&model.StreamChunk{
		Choices: []model.StreamChoice{{
			Delta: model.Delta{Content: stringPtr("hello")},
		}},
		Usage: &model.Usage{PromptTokens: 7, TotalTokens: 7},
	})
	aggregate.Add(&model.StreamChunk{
		Choices: []model.StreamChoice{{
			FinishReason: stringPtr("stop"),
		}},
		Usage: &model.Usage{CompletionTokens: 3, TotalTokens: 3},
	})
	aggregate.Complete()

	response, err := aggregate.Response("gpt-test")
	require.NoError(t, err)
	assert.Equal(t, 7, response.Usage.PromptTokens)
	assert.Equal(t, 3, response.Usage.CompletionTokens)
	assert.Equal(t, 10, response.Usage.TotalTokens)
}

func TestStreamAggregateLeavesContentNullWithoutContentDelta(t *testing.T) {
	var aggregate streamAggregate
	aggregate.Add(&model.StreamChunk{
		Choices: []model.StreamChoice{{
			Delta: model.Delta{
				ToolCalls: []model.ToolCall{{
					Index: intPtr(0),
					ID:    "call-1",
					Type:  "function",
					Function: model.ToolCallFunction{
						Name:      "lookup",
						Arguments: `{}`,
					},
				}},
			},
			FinishReason: stringPtr("tool_calls"),
		}},
	})

	response, err := aggregate.Response("gpt-test")
	require.NoError(t, err)
	assert.Nil(t, response.Choices[0].Message.Content)
}

func TestStreamAggregateRejectsIncompleteStream(t *testing.T) {
	var aggregate streamAggregate
	aggregate.Add(&model.StreamChunk{
		Choices: []model.StreamChoice{{Delta: model.Delta{Content: stringPtr("partial")}}},
	})

	_, err := aggregate.Response("gpt-test")
	require.ErrorContains(t, err, "before completion")
}

func TestStreamAggregateRejectsDoneWithoutFinishReason(t *testing.T) {
	var aggregate streamAggregate
	aggregate.Add(&model.StreamChunk{
		Choices: []model.StreamChoice{{Delta: model.Delta{Content: stringPtr("partial")}}},
	})
	aggregate.Complete()

	_, err := aggregate.Response("gpt-test")
	require.ErrorContains(t, err, "before finish reason")
}

func TestConsumeCompletionStreamAllowsCleanEOFAfterFinishReason(t *testing.T) {
	var aggregate streamAggregate
	err := consumeCompletionStream(
		strings.NewReader(`data: {"id":"chatcmpl-eof","object":"chat.completion.chunk","model":"gpt-test","choices":[{"index":0,"delta":{"content":"done"},"finish_reason":"stop"}]}`+"\n\n"),
		func(data []byte) (*model.StreamChunk, bool, error) {
			var chunk model.StreamChunk
			return &chunk, false, json.Unmarshal(data, &chunk)
		},
		func(chunk *model.StreamChunk, _ bool) error {
			aggregate.Add(chunk)
			return nil
		},
	)
	require.NoError(t, err)

	response, err := aggregate.Response("fallback")
	require.NoError(t, err)
	assert.Equal(t, "done", response.Choices[0].Message.Content)
	assert.Equal(t, "stop", *response.Choices[0].FinishReason)
}

func TestHandleStreamingCompletion_RequestedUsageMissingLogsFailure(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, `data: {"id":"chatcmpl-no-usage","object":"chat.completion.chunk","model":"gpt-4o","choices":[{"index":0,"delta":{"content":"done"},"finish_reason":"stop"}]}`+"\n\n")
	}))
	defer upstream.Close()

	capture := newLogCapture()
	registry := callback.NewRegistry()
	registry.Register(capture)
	h := &Handlers{Config: &config.ProxyConfig{}, Callbacks: registry}
	req := &model.ChatCompletionRequest{
		Model:         "gpt-4o",
		Messages:      []model.Message{{Role: "user", Content: "hi"}},
		Stream:        boolPtr(true),
		StreamOptions: &model.StreamOptions{IncludeUsage: true},
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	h.handleStreamingCompletion(recorder, request, openaiprovider.NewWithBaseURL(upstream.URL), req, "test-key")

	failure := capture.waitFailure(t, 2*time.Second)
	require.ErrorContains(t, failure.Error, "before usage")
	assert.Contains(t, recorder.Body.String(), `"code":"upstream_stream_error"`)
}

func TestStreamAggregateRejectsEmptyFinishReason(t *testing.T) {
	for _, reason := range []string{"", " \t"} {
		t.Run(reason, func(t *testing.T) {
			var aggregate streamAggregate
			aggregate.Add(&model.StreamChunk{
				Choices: []model.StreamChoice{{
					Delta:        model.Delta{Content: stringPtr("partial")},
					FinishReason: stringPtr(reason),
				}},
			})

			_, err := aggregate.Response("gpt-test")
			require.ErrorContains(t, err, "before finish reason")
		})
	}
}

func TestStreamAggregateRejectsUsageOnlyCompletion(t *testing.T) {
	var aggregate streamAggregate
	aggregate.Add(&model.StreamChunk{
		ID:      "chatcmpl-usage",
		Model:   "gpt-test",
		Choices: []model.StreamChoice{},
		Usage:   &model.Usage{PromptTokens: 5, CompletionTokens: 2, TotalTokens: 7},
	})
	aggregate.Complete()

	_, err := aggregate.Response("fallback")
	require.ErrorContains(t, err, "before completion")
}

func TestHandleStreamingCompletion_MalformedStreamLogsFailure(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: {not-json}\n\n")
	}))
	defer upstream.Close()

	capture := newLogCapture()
	registry := callback.NewRegistry()
	registry.Register(capture)
	h := &Handlers{Config: &config.ProxyConfig{}, Callbacks: registry}
	req := &model.ChatCompletionRequest{
		Model:    "gpt-4o",
		Messages: []model.Message{{Role: "user", Content: "hi"}},
		Stream:   boolPtr(true),
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	h.handleStreamingCompletion(recorder, request, openaiprovider.NewWithBaseURL(upstream.URL), req, "test-key")

	failure := capture.waitFailure(t, 2*time.Second)
	require.Error(t, failure.Error)
	assert.Contains(t, recorder.Body.String(), `"code":"upstream_stream_error"`)
	assert.Contains(t, recorder.Body.String(), "data: [DONE]")
}

func TestHandleStreamingCompletion_UpstreamErrorIsSanitized(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "Bearer stream-secret internal stack", http.StatusTooManyRequests)
	}))
	defer upstream.Close()

	capture := newLogCapture()
	registry := callback.NewRegistry()
	registry.Register(capture)
	h := &Handlers{Config: &config.ProxyConfig{}, Callbacks: registry}
	req := &model.ChatCompletionRequest{
		Model:    "gpt-4o",
		Messages: []model.Message{{Role: "user", Content: "hi"}},
		Stream:   boolPtr(true),
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	h.handleStreamingCompletion(recorder, request, openaiprovider.NewWithBaseURL(upstream.URL), req, "test-key")

	capture.waitFailure(t, 2*time.Second)
	require.Equal(t, http.StatusTooManyRequests, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"code":"upstream_error"`)
	assert.Contains(t, recorder.Body.String(), "Too Many Requests")
	assert.NotContains(t, recorder.Body.String(), "stream-secret")
	assert.NotContains(t, recorder.Body.String(), "internal stack")
}

func TestHandleStreamingCompletion_PreservesSanitizedOpenAIClientError(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, `{"error":{"message":"invalid response format Authorization: Bearer stream-secret","type":"invalid_request_error","code":"unsupported_parameter"}}`)
	}))
	defer upstream.Close()

	h := &Handlers{Config: &config.ProxyConfig{}}
	req := &model.ChatCompletionRequest{
		Model:    "gpt-4o",
		Messages: []model.Message{{Role: "user", Content: "hi"}},
		Stream:   boolPtr(true),
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	h.handleStreamingCompletion(recorder, request, openaiprovider.NewWithBaseURL(upstream.URL), req, "test-key")

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	var response model.ErrorResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	assert.Contains(t, response.Error.Message, "invalid response format")
	assert.NotContains(t, response.Error.Message, "stream-secret")
	assert.Equal(t, "invalid_request_error", response.Error.Type)
	assert.Equal(t, "unsupported_parameter", response.Error.Code)
}

func TestWriteUpstreamHTTPFailure_PreservesSanitizedNonStandardStatusMessage(t *testing.T) {
	recorder := httptest.NewRecorder()
	writeUpstreamHTTPFailure(recorder, &model.TianjiError{
		StatusCode: 529,
		Message:    "capacity exhausted Authorization: Bearer stream-secret",
		Type:       "api_error",
	}, "gpt-test")

	require.Equal(t, 529, recorder.Code)
	var response model.ErrorResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	assert.Contains(t, response.Error.Message, "capacity exhausted")
	assert.NotContains(t, response.Error.Message, "stream-secret")
	assert.Equal(t, "upstream_error", response.Error.Code)
}

func TestHandleStreamingCompletion_PreservesReauthorizationError(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, `{"error":{"message":"OpenAI subscription reauthorization required","type":"authentication_error","code":"openai_subscription_reauthorization_required"}}`)
	}))
	defer upstream.Close()

	h := &Handlers{Config: &config.ProxyConfig{}}
	req := &model.ChatCompletionRequest{
		Model:    "gpt-4o",
		Messages: []model.Message{{Role: "user", Content: "hi"}},
		Stream:   boolPtr(true),
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	h.handleStreamingCompletion(recorder, request, openaiprovider.NewWithBaseURL(upstream.URL), req, "test-key")

	require.Equal(t, http.StatusUnauthorized, recorder.Code)
	var response model.ErrorResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	assert.Equal(t, "openai_subscription_reauthorization_required", response.Error.Code)
}

func TestHandleNonStreamingCompletion_UpstreamRequestErrorIsNotLabeledAsStreamFailure(t *testing.T) {
	h := &Handlers{Config: &config.ProxyConfig{}}
	req := &model.ChatCompletionRequest{
		Model:    "gpt-4o",
		Messages: []model.Message{{Role: "user", Content: "hi"}},
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	h.handleNonStreamingCompletion(recorder, request, openaiprovider.NewWithBaseURL("://bad"), req, "test-key", nil)

	require.Equal(t, http.StatusBadGateway, recorder.Code)
	var response model.ErrorResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	assert.Equal(t, "api_error", response.Error.Type)
	assert.Equal(t, "upstream_error", response.Error.Code)
	assert.Equal(t, "upstream request failed", response.Error.Message)
	assert.NotContains(t, recorder.Body.String(), "missing protocol scheme")
}

func TestHandleAggregatedStreamingCompletion_DoesNotCacheByPromptOnly(t *testing.T) {
	var calls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		content := "first"
		if body["max_tokens"] == float64(2) {
			content = "second"
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w,
			`data: {"id":"chatcmpl-cache","object":"chat.completion.chunk","model":"gpt-4o","choices":[{"index":0,"delta":{"content":"`+content+`"}}]}`+"\n\n"+
				`data: {"id":"chatcmpl-cache","object":"chat.completion.chunk","model":"gpt-4o","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`+"\n\n"+
				`data: {"id":"chatcmpl-cache","object":"chat.completion.chunk","model":"gpt-4o","choices":[],"usage":{"prompt_tokens":7,"completion_tokens":1,"total_tokens":8}}`+"\n\n"+
				"data: [DONE]\n\n",
		)
	}))
	defer upstream.Close()

	h := &Handlers{Config: &config.ProxyConfig{}, Cache: cache.NewMemoryCache()}
	provider := openaiprovider.NewWithBaseURL(upstream.URL)
	firstReq := &model.ChatCompletionRequest{
		Model:     "gpt-4o",
		Messages:  []model.Message{{Role: "user", Content: "same prompt"}},
		MaxTokens: intPtr(1),
	}
	first := httptest.NewRecorder()
	h.handleAggregatedStreamingCompletion(first, httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil), provider, firstReq, "test-key", nil, false)
	require.Equal(t, http.StatusOK, first.Code, first.Body.String())

	secondReq := *firstReq
	secondReq.MaxTokens = intPtr(2)
	second := httptest.NewRecorder()
	h.handleAggregatedStreamingCompletion(second, httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil), provider, &secondReq, "test-key", nil, false)

	require.Equal(t, http.StatusOK, second.Code, second.Body.String())
	assert.Empty(t, second.Header().Get("X-Cache"))
	assert.Equal(t, int32(2), calls.Load())
	var response model.ModelResponse
	require.NoError(t, json.Unmarshal(second.Body.Bytes(), &response))
	assert.Equal(t, "second", response.Choices[0].Message.Content)
}
