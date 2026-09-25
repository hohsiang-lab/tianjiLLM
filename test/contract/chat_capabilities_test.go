package contract

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/model"
	"github.com/praxisllmlab/tianjiLLM/internal/testutil/openaitest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChatCapabilities_PreserveSupportedAndRejectUnsupportedParameters(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		field      string
		want       any
		capability model.CapabilityRecord
		stream     bool
	}{
		{
			name:  "json object",
			body:  `{"model":"contract-model","messages":[{"role":"user","content":"hi"}],"response_format":{"type":"json_object"}}`,
			field: "response_format",
			want:  map[string]any{"type": "json_object"},
			capability: model.CapabilityRecord{
				SupportsNonStream:      true,
				SupportsResponseFormat: true,
				SupportsJSONObject:     true,
			},
		},
		{
			name:  "json schema",
			body:  `{"model":"contract-model","messages":[{"role":"user","content":"hi"}],"response_format":{"type":"json_schema","json_schema":{"name":"answer","schema":{"type":"object"},"strict":true}}}`,
			field: "response_format",
			want: map[string]any{
				"type": "json_schema",
				"json_schema": map[string]any{
					"name":   "answer",
					"schema": map[string]any{"type": "object"},
					"strict": true,
				},
			},
			capability: model.CapabilityRecord{
				SupportsNonStream:      true,
				SupportsResponseFormat: true,
				SupportsJSONSchema:     true,
			},
		},
		{
			name:       "max tokens",
			body:       `{"model":"contract-model","messages":[{"role":"user","content":"hi"}],"max_tokens":100}`,
			field:      "max_tokens",
			want:       float64(100),
			capability: model.CapabilityRecord{SupportsNonStream: true, SupportsMaxTokens: true},
		},
		{
			name:       "max completion tokens",
			body:       `{"model":"contract-model","messages":[{"role":"user","content":"hi"}],"max_completion_tokens":100}`,
			field:      "max_completion_tokens",
			want:       float64(100),
			capability: model.CapabilityRecord{SupportsNonStream: true, SupportsMaxCompletionTokens: true},
		},
		{
			name:       "temperature",
			body:       `{"model":"contract-model","messages":[{"role":"user","content":"hi"}],"temperature":0.2}`,
			field:      "temperature",
			want:       0.2,
			capability: model.CapabilityRecord{SupportsNonStream: true, SupportsTemperature: true},
		},
		{
			name:       "top p",
			body:       `{"model":"contract-model","messages":[{"role":"user","content":"hi"}],"top_p":0.9}`,
			field:      "top_p",
			want:       0.9,
			capability: model.CapabilityRecord{SupportsNonStream: true, SupportsTopP: true},
		},
		{
			name:  "tools",
			body:  `{"model":"contract-model","messages":[{"role":"user","content":"hi"}],"tools":[{"type":"function","function":{"name":"lookup","parameters":{"type":"object"}}}]}`,
			field: "tools",
			want: []any{map[string]any{
				"type": "function",
				"function": map[string]any{
					"name":       "lookup",
					"parameters": map[string]any{"type": "object"},
				},
			}},
			capability: model.CapabilityRecord{SupportsNonStream: true, SupportsTools: true},
		},
		{
			name:       "tool choice",
			body:       `{"model":"contract-model","messages":[{"role":"user","content":"hi"}],"tool_choice":"auto"}`,
			field:      "tool_choice",
			want:       "auto",
			capability: model.CapabilityRecord{SupportsNonStream: true, SupportsToolChoice: true},
		},
		{
			name:       "stream usage",
			body:       `{"model":"contract-model","messages":[{"role":"user","content":"hi"}],"stream":true,"stream_options":{"include_usage":true}}`,
			field:      "stream_options",
			want:       map[string]any{"include_usage": true},
			capability: model.CapabilityRecord{SupportsStream: true, SupportsStreamOptionsIncludeUsage: true},
			stream:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			options := openaitest.UpstreamServerOptions{}
			if tt.stream {
				options.ChatCompletionSSE = standardChatCompletionSSE()
			}
			srv, upstream := newOpenAIContractServer(t, options, tt.capability)
			recorder := httptest.NewRecorder()
			srv.ServeHTTP(recorder, newAuthenticatedContractRequest(http.MethodPost, "/v1/chat/completions", tt.body))

			require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
			requests := upstream.Requests()
			require.Len(t, requests, 1)
			var body map[string]any
			require.NoError(t, json.Unmarshal(requests[0].Body, &body))
			assert.Equal(t, tt.want, body[tt.field])

			unsupported := model.CapabilityRecord{SupportsNonStream: true}
			if tt.stream {
				unsupported = model.CapabilityRecord{SupportsStream: true}
			}
			srv, upstream = newOpenAIContractServer(t, options, unsupported)
			recorder = httptest.NewRecorder()
			srv.ServeHTTP(recorder, newAuthenticatedContractRequest(http.MethodPost, "/v1/chat/completions", tt.body))

			require.Equal(t, http.StatusBadRequest, recorder.Code, recorder.Body.String())
			param := tt.field
			if tt.field == "stream_options" {
				param = "stream_options.include_usage"
			}
			requireStandardError(t, recorder.Body.Bytes(), param, "unsupported_parameter")
			assert.Empty(t, upstream.Requests())
		})
	}
}

func TestChatCapabilities_ReturnStandardValidationErrorsBeforeUpstream(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		capability model.CapabilityRecord
		param      string
		code       string
	}{
		{
			name:       "unsupported parameter",
			body:       `{"model":"contract-model","messages":[{"role":"user","content":"hi"}],"temperature":0.2}`,
			capability: model.CapabilityRecord{SupportsNonStream: true},
			param:      "temperature",
			code:       "unsupported_parameter",
		},
		{
			name:       "invalid value",
			body:       `{"model":"contract-model","messages":[{"role":"user","content":"hi"}],"temperature":2.1}`,
			capability: model.CapabilityRecord{SupportsNonStream: true, SupportsTemperature: true},
			param:      "temperature",
			code:       "invalid_value",
		},
		{
			name:       "ambiguous token fields",
			body:       `{"model":"contract-model","messages":[{"role":"user","content":"hi"}],"max_tokens":1,"max_completion_tokens":2}`,
			capability: model.CapabilityRecord{SupportsNonStream: true, SupportsMaxTokens: true, SupportsMaxCompletionTokens: true},
			param:      "max_tokens",
			code:       "invalid_request",
		},
		{
			name:       "usage option requires stream",
			body:       `{"model":"contract-model","messages":[{"role":"user","content":"hi"}],"stream":false,"stream_options":{"include_usage":true}}`,
			capability: model.CapabilityRecord{SupportsNonStream: true, SupportsStreamOptionsIncludeUsage: true},
			param:      "stream_options",
			code:       "invalid_request",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv, upstream := newOpenAIContractServer(t, openaitest.UpstreamServerOptions{}, tt.capability)
			recorder := httptest.NewRecorder()
			srv.ServeHTTP(recorder, newAuthenticatedContractRequest(http.MethodPost, "/v1/chat/completions", tt.body))

			require.Equal(t, http.StatusBadRequest, recorder.Code, recorder.Body.String())
			requireStandardError(t, recorder.Body.Bytes(), tt.param, tt.code)
			assert.Empty(t, upstream.Requests())
		})
	}
}
