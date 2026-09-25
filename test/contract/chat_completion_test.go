package contract

import (
	"bufio"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/praxisllmlab/tianjiLLM/internal/model"
	_ "github.com/praxisllmlab/tianjiLLM/internal/provider/openai" // register provider
	"github.com/praxisllmlab/tianjiLLM/internal/proxy"
	"github.com/praxisllmlab/tianjiLLM/internal/proxy/handler"
	"github.com/praxisllmlab/tianjiLLM/internal/testutil/openaitest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestServer(t *testing.T, upstreamURL string) *proxy.Server {
	t.Helper()
	apiKey := "sk-test-key"
	cfg := &config.ProxyConfig{
		ModelList: []config.ModelConfig{
			{
				ModelName: "gpt-4o",
				TianjiParams: config.TianjiParams{
					Model:  "openai/gpt-4o",
					APIKey: &apiKey,
				},
			},
		},
		GeneralSettings: config.GeneralSettings{
			MasterKey: "sk-master",
		},
	}

	handlers := &handler.Handlers{
		Config: cfg,
	}

	return proxy.NewServer(proxy.ServerConfig{
		Handlers:  handlers,
		MasterKey: cfg.GeneralSettings.MasterKey,
	})
}

func TestChatCompletion_NonStreamingJSON(t *testing.T) {
	srv, upstream := newDirectOpenAIContractServer(t, openaitest.UpstreamServerOptions{
		ChatCompletionBody: map[string]any{
			"id":      "chatcmpl-direct",
			"object":  "chat.completion",
			"created": 1700000000,
			"model":   "gpt-4o",
			"choices": []any{map[string]any{
				"index": 0,
				"message": map[string]any{
					"role":    "assistant",
					"content": "hello",
				},
				"finish_reason": "stop",
			}},
			"usage": map[string]any{
				"prompt_tokens":     7,
				"completion_tokens": 3,
				"total_tokens":      10,
			},
		},
	})

	recorder := httptest.NewRecorder()
	srv.ServeHTTP(recorder, newAuthenticatedContractRequest(
		http.MethodPost,
		"/v1/chat/completions",
		`{"model":"contract-model","messages":[{"role":"user","content":"hi"}],"stream":false,"max_completion_tokens":321}`,
	))

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	assert.Equal(t, "application/json", recorder.Header().Get("Content-Type"))
	var response model.ModelResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	assert.Equal(t, "chatcmpl-direct", response.ID)
	assert.Equal(t, "chat.completion", response.Object)
	require.Len(t, response.Choices, 1)
	assert.Equal(t, "hello", response.Choices[0].Message.Content)
	assert.Equal(t, "stop", *response.Choices[0].FinishReason)
	assert.Equal(t, 10, response.Usage.TotalTokens)

	requests := upstream.Requests()
	require.Len(t, requests, 1)
	var upstreamBody map[string]any
	require.NoError(t, json.Unmarshal(requests[0].Body, &upstreamBody))
	assert.Equal(t, false, upstreamBody["stream"])
	assert.Equal(t, float64(321), upstreamBody["max_completion_tokens"])
}

func TestChatCompletion_StreamingSSE(t *testing.T) {
	srv, upstream := newDirectOpenAIContractServer(t, openaitest.UpstreamServerOptions{
		ChatCompletionSSE: standardChatCompletionSSE(),
	})

	recorder := httptest.NewRecorder()
	srv.ServeHTTP(recorder, newAuthenticatedContractRequest(
		http.MethodPost,
		"/v1/chat/completions",
		`{"model":"contract-model","messages":[{"role":"user","content":"hi"}],"stream":true,"stream_options":{"include_usage":true}}`,
	))

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	assert.Contains(t, recorder.Header().Get("Content-Type"), "text/event-stream")
	events := decodeSSEData(t, recorder.Body)
	require.Len(t, events, 5)
	assert.Equal(t, "[DONE]", events[4])

	var first, second, finish, usage model.StreamChunk
	require.NoError(t, json.Unmarshal([]byte(events[0]), &first))
	require.NoError(t, json.Unmarshal([]byte(events[1]), &second))
	require.NoError(t, json.Unmarshal([]byte(events[2]), &finish))
	require.NoError(t, json.Unmarshal([]byte(events[3]), &usage))
	assert.Equal(t, "hel", *first.Choices[0].Delta.Content)
	assert.Equal(t, "lo", *second.Choices[0].Delta.Content)
	assert.Equal(t, "stop", *finish.Choices[0].FinishReason)
	assert.Empty(t, usage.Choices)
	require.NotNil(t, usage.Usage)
	assert.Equal(t, 10, usage.Usage.TotalTokens)

	requests := upstream.Requests()
	require.Len(t, requests, 1)
	var upstreamBody map[string]any
	require.NoError(t, json.Unmarshal(requests[0].Body, &upstreamBody))
	assert.Equal(t, true, upstreamBody["stream"])
	assert.Equal(t, map[string]any{"include_usage": true}, upstreamBody["stream_options"])
}

func TestChatCompletion_StreamOnlyBackendAggregatesToJSON(t *testing.T) {
	srv, upstream := newStreamOnlyContractServer(t, openaitest.UpstreamServerOptions{
		ChatCompletionSSE: standardChatCompletionSSE(),
	})

	recorder := httptest.NewRecorder()
	srv.ServeHTTP(recorder, newAuthenticatedContractRequest(
		http.MethodPost,
		"/v1/chat/completions",
		`{"model":"contract-model","messages":[{"role":"user","content":"hi"}],"stream":false}`,
	))

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	assert.Equal(t, "application/json", recorder.Header().Get("Content-Type"))
	var response model.ModelResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Len(t, response.Choices, 1)
	assert.Equal(t, "hello", response.Choices[0].Message.Content)
	assert.Equal(t, "stop", *response.Choices[0].FinishReason)
	assert.Equal(t, 10, response.Usage.TotalTokens)

	requests := upstream.Requests()
	require.Len(t, requests, 1)
	var upstreamBody map[string]any
	require.NoError(t, json.Unmarshal(requests[0].Body, &upstreamBody))
	assert.Equal(t, true, upstreamBody["stream"])
	assert.Equal(t, map[string]any{"include_usage": true}, upstreamBody["stream_options"])
}

func TestChatCompletion_StreamOnlyBackendDoesNotInventUnsupportedUsageOption(t *testing.T) {
	srv, upstream := newOpenAIContractServer(t, openaitest.UpstreamServerOptions{
		ChatCompletionSSE: standardChatCompletionSSE(),
	}, model.CapabilityRecord{SupportsStream: true})

	recorder := httptest.NewRecorder()
	srv.ServeHTTP(recorder, newAuthenticatedContractRequest(
		http.MethodPost,
		"/v1/chat/completions",
		`{"model":"contract-model","messages":[{"role":"user","content":"hi"}],"stream":false,"stream_options":{"include_usage":false}}`,
	))

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	requests := upstream.Requests()
	require.Len(t, requests, 1)
	var upstreamBody map[string]any
	require.NoError(t, json.Unmarshal(requests[0].Body, &upstreamBody))
	assert.NotContains(t, upstreamBody, "stream_options")
}

func TestChatCompletion_StreamOnlyBackendAggregatesWithoutUnsupportedUsageOption(t *testing.T) {
	srv, _ := newOpenAIContractServer(t, openaitest.UpstreamServerOptions{
		ChatCompletionSSE: []string{
			`{"id":"chatcmpl-stream","object":"chat.completion.chunk","model":"gpt-4o","choices":[{"index":0,"delta":{"content":"hello"},"finish_reason":null}]}`,
			`{"id":"chatcmpl-stream","object":"chat.completion.chunk","model":"gpt-4o","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`,
			`[DONE]`,
		},
	}, model.CapabilityRecord{SupportsStream: true})

	recorder := httptest.NewRecorder()
	srv.ServeHTTP(recorder, newAuthenticatedContractRequest(
		http.MethodPost,
		"/v1/chat/completions",
		`{"model":"contract-model","messages":[{"role":"user","content":"hi"}],"stream":false}`,
	))

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	var response model.ModelResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Len(t, response.Choices, 1)
	assert.Equal(t, "hello", response.Choices[0].Message.Content)
	assert.Equal(t, "stop", *response.Choices[0].FinishReason)
	assert.Equal(t, model.Usage{}, response.Usage)
}

func TestChatCompletion_StreamOnlyBackendForwardsChunks(t *testing.T) {
	release := make(chan struct{})
	released := false
	defer func() {
		if !released {
			close(release)
		}
	}()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unavailable", http.StatusInternalServerError)
			return
		}
		_, _ = io.WriteString(w, "data: "+standardChatCompletionSSE()[0]+"\n\n")
		flusher.Flush()
		<-release
		for _, event := range standardChatCompletionSSE()[1:] {
			_, _ = io.WriteString(w, "data: "+event+"\n\n")
		}
		flusher.Flush()
	}))
	defer upstream.Close()

	gateway := httptest.NewServer(newOpenAIContractProxy(upstream.URL+"/v1", model.CapabilityRecord{
		SupportsStream:                    true,
		SupportsStreamOptionsIncludeUsage: true,
	}))
	defer gateway.Close()
	client := gateway.Client()
	client.Timeout = 2 * time.Second
	req, err := http.NewRequest(
		http.MethodPost,
		gateway.URL+"/v1/chat/completions",
		strings.NewReader(`{"model":"contract-model","messages":[{"role":"user","content":"hi"}],"stream":true}`),
	)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+contractMasterKey)
	req.Header.Set("Content-Type", "application/json")

	response, err := client.Do(req)
	require.NoError(t, err)
	defer response.Body.Close()
	reader := bufio.NewReader(response.Body)
	firstLine, err := reader.ReadString('\n')
	require.NoError(t, err)
	assert.Contains(t, firstLine, `"content":"hel"`)

	close(release)
	released = true
	rest, err := io.ReadAll(reader)
	require.NoError(t, err)
	assert.Contains(t, string(rest), `"finish_reason":"stop"`)
	assert.Contains(t, string(rest), "data: [DONE]")
}

func TestChatCompletion_StreamOnlyMalformedOrIncompleteStreamFails(t *testing.T) {
	tests := []struct {
		name   string
		events []string
	}{
		{name: "malformed", events: []string{`{"id":"chatcmpl-stream","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":"partial"},"finish_reason":null}]}`, `{not-json}`}},
		{name: "incomplete", events: []string{`{"id":"chatcmpl-stream","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":"partial"},"finish_reason":null}]}`}},
		{name: "done without finish reason", events: []string{`{"id":"chatcmpl-stream","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":"partial"},"finish_reason":null}]}`, `[DONE]`}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv, _ := newStreamOnlyContractServer(t, openaitest.UpstreamServerOptions{ChatCompletionSSE: tt.events})
			recorder := httptest.NewRecorder()
			srv.ServeHTTP(recorder, newAuthenticatedContractRequest(
				http.MethodPost,
				"/v1/chat/completions",
				`{"model":"contract-model","messages":[{"role":"user","content":"hi"}]}`,
			))

			require.Equal(t, http.StatusBadGateway, recorder.Code, recorder.Body.String())
			var response model.ErrorResponse
			require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
			assert.Equal(t, "api_error", response.Error.Type)
			assert.Equal(t, "upstream_stream_error", response.Error.Code)
			assert.NotContains(t, recorder.Body.String(), "partial")
		})
	}
}

func TestChatCompletion_StreamOnlyDoneWithoutFinishReasonFails(t *testing.T) {
	srv, _ := newOpenAIContractServer(t, openaitest.UpstreamServerOptions{
		ChatCompletionSSE: []string{
			`{"id":"chatcmpl-stream","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":"partial"},"finish_reason":null}]}`,
			`[DONE]`,
		},
	}, model.CapabilityRecord{SupportsStream: true})
	recorder := httptest.NewRecorder()

	srv.ServeHTTP(recorder, newAuthenticatedContractRequest(
		http.MethodPost,
		"/v1/chat/completions",
		`{"model":"contract-model","messages":[{"role":"user","content":"hi"}]}`,
	))

	require.Equal(t, http.StatusBadGateway, recorder.Code, recorder.Body.String())
	var response model.ErrorResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	assert.Equal(t, "upstream_stream_error", response.Error.Code)
}

func TestChatCompletion_StreamOnlyUpstreamStatusPreservesSanitizedError(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = io.WriteString(w, `{"error":{"message":"Bearer sk-secret rate limited","type":"rate_limit_error","code":"rate_limit_exceeded"}}`)
	}))
	defer upstream.Close()

	srv := newOpenAIContractProxy(upstream.URL, model.CapabilityRecord{SupportsStream: true})
	recorder := httptest.NewRecorder()
	srv.ServeHTTP(recorder, newAuthenticatedContractRequest(
		http.MethodPost,
		"/v1/chat/completions",
		`{"model":"contract-model","messages":[{"role":"user","content":"hi"}],"stream":false}`,
	))

	require.Equal(t, http.StatusTooManyRequests, recorder.Code, recorder.Body.String())
	var response model.ErrorResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	assert.Equal(t, "rate_limit_error", response.Error.Type)
	assert.Equal(t, "rate_limit_exceeded", response.Error.Code)
	assert.Contains(t, response.Error.Message, "rate limited")
	assert.NotContains(t, recorder.Body.String(), "sk-secret")
}

func TestChatCompletion_RejectsAmbiguousTokenLimitsBeforeUpstream(t *testing.T) {
	srv, upstream := newDirectOpenAIContractServer(t, openaitest.UpstreamServerOptions{
		ChatCompletionBody: map[string]any{"id": "must-not-run"},
	})
	recorder := httptest.NewRecorder()
	srv.ServeHTTP(recorder, newAuthenticatedContractRequest(
		http.MethodPost,
		"/v1/chat/completions",
		`{"model":"contract-model","messages":[{"role":"user","content":"hi"}],"max_tokens":1,"max_completion_tokens":2}`,
	))

	require.Equal(t, http.StatusBadRequest, recorder.Code, recorder.Body.String())
	var response model.ErrorResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	assert.Equal(t, "invalid_request_error", response.Error.Type)
	assert.Equal(t, "max_tokens", response.Error.Param)
	assert.Equal(t, "invalid_request", response.Error.Code)
	assert.Empty(t, upstream.Requests())
}

func TestChatCompletion_RejectsNonStreamingUsageOptionsBeforeUpstream(t *testing.T) {
	srv, upstream := newDirectOpenAIContractServer(t, openaitest.UpstreamServerOptions{
		ChatCompletionBody: map[string]any{"id": "must-not-run"},
	})
	recorder := httptest.NewRecorder()
	srv.ServeHTTP(recorder, newAuthenticatedContractRequest(
		http.MethodPost,
		"/v1/chat/completions",
		`{"model":"contract-model","messages":[{"role":"user","content":"hi"}],"stream":false,"stream_options":{"include_usage":true}}`,
	))

	require.Equal(t, http.StatusBadRequest, recorder.Code, recorder.Body.String())
	var response model.ErrorResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	assert.Equal(t, "invalid_request_error", response.Error.Type)
	assert.Equal(t, "stream_options", response.Error.Param)
	assert.Equal(t, "invalid_request", response.Error.Code)
	assert.Empty(t, upstream.Requests())
}

func TestChatCompletion_ExplicitTextResponseFormatIsOmitted(t *testing.T) {
	srv, upstream := newOpenAIContractServer(t, openaitest.UpstreamServerOptions{
		ChatCompletionBody: map[string]any{
			"id":      "chatcmpl-text",
			"object":  "chat.completion",
			"model":   "gpt-4o",
			"choices": []any{map[string]any{"index": 0, "message": map[string]any{"role": "assistant", "content": "ok"}, "finish_reason": "stop"}},
			"usage":   map[string]any{"prompt_tokens": 1, "completion_tokens": 1, "total_tokens": 2},
		},
	}, model.CapabilityRecord{SupportsNonStream: true})

	recorder := httptest.NewRecorder()
	srv.ServeHTTP(recorder, newAuthenticatedContractRequest(
		http.MethodPost,
		"/v1/chat/completions",
		`{"model":"contract-model","messages":[{"role":"user","content":"hi"}],"response_format":{"type":"text"}}`,
	))

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	requests := upstream.Requests()
	require.Len(t, requests, 1)
	var upstreamBody map[string]any
	require.NoError(t, json.Unmarshal(requests[0].Body, &upstreamBody))
	assert.NotContains(t, upstreamBody, "response_format")
}

func standardChatCompletionSSE() []string {
	return []string{
		`{"id":"chatcmpl-stream","object":"chat.completion.chunk","created":1700000000,"model":"gpt-4o","choices":[{"index":0,"delta":{"content":"hel"},"finish_reason":null}]}`,
		`{"id":"chatcmpl-stream","object":"chat.completion.chunk","created":1700000000,"model":"gpt-4o","choices":[{"index":0,"delta":{"content":"lo"},"finish_reason":null}]}`,
		`{"id":"chatcmpl-stream","object":"chat.completion.chunk","created":1700000000,"model":"gpt-4o","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`,
		`{"id":"chatcmpl-stream","object":"chat.completion.chunk","created":1700000000,"model":"gpt-4o","choices":[],"usage":{"prompt_tokens":7,"completion_tokens":3,"total_tokens":10}}`,
		`[DONE]`,
	}
}

func TestChatCompletion_InvalidModel(t *testing.T) {
	srv := newTestServer(t, "")

	body := `{
		"model": "nonexistent-model",
		"messages": [{"role": "user", "content": "Hello"}]
	}`

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer sk-master")

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var errResp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &errResp))
	assert.Contains(t, errResp, "error")
}

func TestChatCompletion_InvalidJSON(t *testing.T) {
	srv := newTestServer(t, "")

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader("{invalid}"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer sk-master")

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestChatCompletion_NoAuth(t *testing.T) {
	srv := newTestServer(t, "")

	body := `{"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}`

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestHealthEndpoints(t *testing.T) {
	srv := newTestServer(t, "")

	tests := []struct {
		path string
		code int
	}{
		{"/health", http.StatusOK},
		{"/health/liveness", http.StatusOK},
		{"/health/readiness", http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			w := httptest.NewRecorder()
			srv.ServeHTTP(w, req)
			assert.Equal(t, tt.code, w.Code)
		})
	}
}

func TestListModels(t *testing.T) {
	srv := newTestServer(t, "")

	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	req.Header.Set("Authorization", "Bearer sk-master")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "list", resp["object"])

	data, ok := resp["data"].([]any)
	require.True(t, ok)
	assert.Len(t, data, 1)
}
