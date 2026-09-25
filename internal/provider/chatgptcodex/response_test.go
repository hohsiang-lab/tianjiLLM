package chatgptcodex

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTransformResponse_MapsCompletedResponsesOutputToChatCompletion(t *testing.T) {
	transport := Transport{}
	resp := codexHTTPResponse(http.StatusOK, `{
		"id": "resp_123",
		"object": "response",
		"created_at": 1710000000,
		"model": "gpt-5.5-codex",
		"status": "completed",
		"output": [{
			"type": "message",
			"role": "assistant",
			"content": [
				{"type": "output_text", "text": "hello "},
				{"type": "output_text", "text": "world"}
			]
		}],
		"usage": {"input_tokens": 11, "output_tokens": 7, "total_tokens": 18}
	}`)

	got, err := transport.TransformResponse(resp, "chatgpt/gpt-5.5-codex")
	require.NoError(t, err)

	assert.Equal(t, "resp_123", got.ID)
	assert.Equal(t, "chat.completion", got.Object)
	assert.Equal(t, int64(1710000000), got.Created)
	assert.Equal(t, "gpt-5.5-codex", got.Model)
	require.Len(t, got.Choices, 1)
	assert.Equal(t, 0, got.Choices[0].Index)
	require.NotNil(t, got.Choices[0].Message)
	assert.Equal(t, "assistant", got.Choices[0].Message.Role)
	assert.Equal(t, "hello world", got.Choices[0].Message.Content)
	require.NotNil(t, got.Choices[0].FinishReason)
	assert.Equal(t, "stop", *got.Choices[0].FinishReason)
	assert.Equal(t, 11, got.Usage.PromptTokens)
	assert.Equal(t, 7, got.Usage.CompletionTokens)
	assert.Equal(t, 18, got.Usage.TotalTokens)
}

func TestTransformResponse_PreservesToolCallOnlyOutput(t *testing.T) {
	transport := Transport{}
	resp := codexHTTPResponse(http.StatusOK, `{
		"id": "resp_tool",
		"object": "response",
		"created_at": 1710000000,
		"model": "gpt-5.6-terra",
		"status": "completed",
		"output": [{
			"type": "function_call",
			"id": "fc_1",
			"call_id": "call_1",
			"name": "lookup",
			"arguments": "{\"id\":\"123\"}"
		}],
		"usage": {"input_tokens": 11, "output_tokens": 7, "total_tokens": 18}
	}`)

	got, err := transport.TransformResponse(resp, "chatgpt/gpt-5.6-terra")

	require.NoError(t, err)
	require.Len(t, got.Choices, 1)
	require.NotNil(t, got.Choices[0].Message)
	assert.Nil(t, got.Choices[0].Message.Content)
	require.Len(t, got.Choices[0].Message.ToolCalls, 1)
	assert.Equal(t, "call_1", got.Choices[0].Message.ToolCalls[0].ID)
	assert.Equal(t, "function", got.Choices[0].Message.ToolCalls[0].Type)
	assert.Equal(t, "lookup", got.Choices[0].Message.ToolCalls[0].Function.Name)
	assert.Equal(t, `{"id":"123"}`, got.Choices[0].Message.ToolCalls[0].Function.Arguments)
	require.NotNil(t, got.Choices[0].FinishReason)
	assert.Equal(t, "tool_calls", *got.Choices[0].FinishReason)
}

func TestTransformResponse_NoOutputTextReturnsSafeTransformError(t *testing.T) {
	transport := Transport{}
	resp := codexHTTPResponse(http.StatusOK, `{
		"id": "resp_empty",
		"object": "response",
		"status": "completed",
		"output": [{"type": "message", "role": "assistant", "content": []}]
	}`)

	_, err := transport.TransformResponse(resp, "chatgpt/gpt-5.5-codex")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no assistant output_text")
}

func TestTransformResponse_PreservesActionableUpstreamErrors(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		wantType   string
		wantCode   string
		wantErr    error
	}{
		{
			name:       "auth",
			statusCode: http.StatusUnauthorized,
			body:       `{"error":{"message":"missing model.request scope","type":"authentication_error","code":"invalid_token"}}`,
			wantType:   "authentication_error",
			wantCode:   "invalid_token",
			wantErr:    model.ErrAuthentication,
		},
		{
			name:       "missing scope",
			statusCode: http.StatusForbidden,
			body:       `{"error":{"message":"missing api.responses.write scope","type":"permission_error","code":"missing_scope"}}`,
			wantType:   "permission_error",
			wantCode:   "missing_scope",
			wantErr:    model.ErrPermission,
		},
		{
			name:       "quota",
			statusCode: http.StatusTooManyRequests,
			body:       `{"error":{"message":"You exceeded your quota","type":"insufficient_quota","code":"insufficient_quota"}}`,
			wantType:   "insufficient_quota",
			wantCode:   "insufficient_quota",
			wantErr:    model.ErrRateLimit,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transport := Transport{}
			_, err := transport.TransformResponse(codexHTTPResponse(tt.statusCode, tt.body), "chatgpt/gpt-5.5-codex")
			require.Error(t, err)

			var tianjiErr *model.TianjiError
			require.ErrorAs(t, err, &tianjiErr)
			assert.Equal(t, tt.statusCode, tianjiErr.StatusCode)
			assert.Equal(t, tt.wantType, tianjiErr.Type)
			assert.Equal(t, tt.wantCode, tianjiErr.Code)
			assert.True(t, errors.Is(tianjiErr, tt.wantErr))
		})
	}
}

func TestTransformResponse_RedactsTokenMaterialFromUpstreamError(t *testing.T) {
	transport := Transport{}
	_, err := transport.TransformResponse(codexHTTPResponse(http.StatusForbidden, `{
		"error": {
			"message": "Authorization: Bearer sk-secret access_token=access-secret refresh_token=refresh-secret {\"access_token\":\"json-access\",\"refresh_token\":\"json-refresh\"}",
			"type": "permission_error",
			"code": "missing_scope"
		}
	}`), "chatgpt/gpt-5.5-codex")
	require.Error(t, err)

	var tianjiErr *model.TianjiError
	require.ErrorAs(t, err, &tianjiErr)
	assert.NotContains(t, tianjiErr.Message, "sk-secret")
	assert.NotContains(t, tianjiErr.Message, "access-secret")
	assert.NotContains(t, tianjiErr.Message, "refresh-secret")
	assert.NotContains(t, tianjiErr.Message, "json-access")
	assert.NotContains(t, tianjiErr.Message, "json-refresh")
}

func codexHTTPResponse(statusCode int, body string) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Body:       io.NopCloser(bytes.NewBufferString(body)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}
}
