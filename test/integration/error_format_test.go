package integration

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/praxisllmlab/tianjiLLM/internal/model"
	"github.com/praxisllmlab/tianjiLLM/internal/proxy"
	"github.com/praxisllmlab/tianjiLLM/internal/proxy/handler"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestErrorFormat_MissingAuth(t *testing.T) {
	handlers := &handler.Handlers{
		Config: &config.ProxyConfig{},
	}
	srv := proxy.NewServer(proxy.ServerConfig{
		Handlers:  handlers,
		MasterKey: "sk-master",
	})

	req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(`{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}]}`))
	// No Authorization header

	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)

	var errResp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &errResp))
	// Should have error field with message
	errField, ok := errResp["error"]
	assert.True(t, ok, "response should have 'error' field")
	assert.NotNil(t, errField)
}

func TestErrorFormat_ModelNotFound(t *testing.T) {
	handlers := &handler.Handlers{
		Config: &config.ProxyConfig{},
	}
	srv := proxy.NewServer(proxy.ServerConfig{
		Handlers:  handlers,
		MasterKey: "sk-master",
	})

	req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(`{"model":"nonexistent-model","messages":[{"role":"user","content":"hi"}]}`))
	req.Header.Set("Authorization", "Bearer sk-master")

	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)

	var errResp model.ErrorResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &errResp))
	assert.Contains(t, errResp.Error.Message, "not found")
	assert.Equal(t, "invalid_request_error", errResp.Error.Type)
	assert.Equal(t, "model_not_found", errResp.Error.Code)
}

func TestErrorFormat_InvalidJSON(t *testing.T) {
	handlers := &handler.Handlers{
		Config: &config.ProxyConfig{},
	}
	srv := proxy.NewServer(proxy.ServerConfig{
		Handlers:  handlers,
		MasterKey: "sk-master",
	})

	req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(`not json`))
	req.Header.Set("Authorization", "Bearer sk-master")

	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)

	var errResp model.ErrorResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &errResp))
	assert.Contains(t, errResp.Error.Message, "invalid request body")
	assert.Equal(t, "invalid_request_error", errResp.Error.Type)
}

func TestErrorFormat_NotImplemented(t *testing.T) {
	handlers := &handler.Handlers{
		Config: &config.ProxyConfig{},
	}
	srv := proxy.NewServer(proxy.ServerConfig{
		Handlers:  handlers,
		MasterKey: "sk-master",
	})

	req := httptest.NewRequest("POST", "/v1/someprovider/endpoint", nil)
	req.Header.Set("Authorization", "Bearer sk-master")

	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotImplemented, rec.Code)

	var errResp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &errResp))
	errField, ok := errResp["error"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "not_implemented", errField["type"])
}

func TestErrorFormat_TianjiError_MatchesPython(t *testing.T) {
	// Verify TianjiError structure matches Python LiteLLM's error format
	err := &model.TianjiError{
		StatusCode: 429,
		Message:    "Rate limit exceeded",
		Type:       "RateLimitError",
		Provider:   "openai",
		Model:      "gpt-4o",
		Err:        model.ErrRateLimit,
	}

	assert.Equal(t, 429, err.StatusCode)
	assert.Equal(t, "openai", err.Provider)
	assert.Contains(t, err.Error(), "[openai]")
	assert.Contains(t, err.Error(), "RateLimitError")
	assert.ErrorIs(t, err, model.ErrRateLimit)

	// JSON serialization
	data, jsonErr := json.Marshal(err)
	require.NoError(t, jsonErr)

	var parsed map[string]any
	require.NoError(t, json.Unmarshal(data, &parsed))
	assert.Equal(t, float64(429), parsed["status_code"])
	assert.Equal(t, "openai", parsed["llm_provider"])
	assert.Equal(t, "gpt-4o", parsed["model"])
}

func TestMapHTTPStatusToError(t *testing.T) {
	tests := []struct {
		status int
		want   error
	}{
		{401, model.ErrAuthentication},
		{403, model.ErrPermission},
		{404, model.ErrNotFound},
		{429, model.ErrRateLimit},
		{400, model.ErrInvalidRequest},
		{408, model.ErrTimeout},
		{500, model.ErrServiceUnavailable},
		{503, model.ErrServiceUnavailable},
	}

	for _, tt := range tests {
		err := model.MapHTTPStatusToError(tt.status)
		assert.ErrorIs(t, err, tt.want, "status %d", tt.status)
	}
}
