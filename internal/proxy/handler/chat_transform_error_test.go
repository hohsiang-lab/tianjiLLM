package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteTransformError_PreservesActionableUpstreamStatusAndCode(t *testing.T) {
	w := httptest.NewRecorder()
	err := &model.TianjiError{
		StatusCode: http.StatusTooManyRequests,
		Message:    "You exceeded your quota",
		Type:       "insufficient_quota",
		Code:       "insufficient_quota",
		Provider:   "chatgpt_codex_backend",
		Model:      "gpt-5.5-codex",
		Err:        model.ErrRateLimit,
	}

	writeTransformError(w, err, "fallback-model")

	assert.Equal(t, http.StatusTooManyRequests, w.Code)
	var body model.ErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "You exceeded your quota", body.Error.Message)
	assert.Equal(t, "insufficient_quota", body.Error.Type)
	assert.Equal(t, "insufficient_quota", body.Error.Code)
	assert.Equal(t, "chatgpt_codex_backend", body.Error.Provider)
	assert.Equal(t, "gpt-5.5-codex", body.Error.Model)
}

func TestWriteTransformError_RedactsActionableUpstreamMessage(t *testing.T) {
	w := httptest.NewRecorder()
	err := &model.TianjiError{
		StatusCode: http.StatusForbidden,
		Message:    "Authorization: Bearer sk-secret access_token=access-secret refresh_token=refresh-secret",
		Type:       "permission_error",
		Code:       "missing_scope",
		Provider:   "chatgpt_codex_backend",
		Model:      "gpt-5.5-codex",
		Err:        model.ErrPermission,
	}

	writeTransformError(w, err, "fallback-model")

	assert.Equal(t, http.StatusForbidden, w.Code)
	body := w.Body.String()
	assert.NotContains(t, body, "sk-secret")
	assert.NotContains(t, body, "access-secret")
	assert.NotContains(t, body, "refresh-secret")
	assert.Contains(t, body, "missing_scope")
}

func TestWriteTransformError_GenericTransformErrorStaysBadGateway(t *testing.T) {
	w := httptest.NewRecorder()

	writeTransformError(w, assert.AnError, "gpt-4o")

	assert.Equal(t, http.StatusBadGateway, w.Code)
	var body model.ErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "internal_error", body.Error.Type)
	assert.Contains(t, body.Error.Message, "transform response")
}

func TestWriteTransformError_OpenAIPlatformErrorStaysBadGateway(t *testing.T) {
	w := httptest.NewRecorder()
	err := &model.TianjiError{
		StatusCode: http.StatusUnauthorized,
		Message:    "Incorrect API key provided",
		Type:       "invalid_request_error",
		Code:       "invalid_api_key",
		Provider:   "openai",
		Err:        model.ErrAuthentication,
	}

	writeTransformError(w, err, "gpt-4o")

	assert.Equal(t, http.StatusBadGateway, w.Code)
	var body model.ErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "internal_error", body.Error.Type)
	assert.Empty(t, body.Error.Code)
}
