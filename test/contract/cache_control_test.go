package contract

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/proxy/middleware"
	"github.com/stretchr/testify/assert"
)

func TestCacheControlMiddleware_NoAllowedControls(t *testing.T) {
	mw := middleware.NewCacheControlMiddleware()

	called := false
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	// When no allowed controls are in context, all requests pass through
	body := `{"model":"gpt-4","cache":{"type":"ephemeral"}}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.True(t, called, "should pass through when no allowed controls configured")
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCacheControlMiddleware_GetRequestPassThrough(t *testing.T) {
	mw := middleware.NewCacheControlMiddleware()

	called := false
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.True(t, called, "GET requests should always pass through")
}

func TestCacheControlMiddleware_NoCacheFieldInBody(t *testing.T) {
	mw := middleware.NewCacheControlMiddleware()

	called := false
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	body := `{"model":"gpt-4","messages":[{"role":"user","content":"hello"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.True(t, called, "should pass through when no cache field in request body")
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCacheControlMiddleware_InvalidJSON(t *testing.T) {
	mw := middleware.NewCacheControlMiddleware()

	called := false
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.True(t, called, "should pass through on JSON decode error")
}

func TestCacheControlMiddleware_NilBody(t *testing.T) {
	mw := middleware.NewCacheControlMiddleware()

	called := false
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.True(t, called, "should pass through when body is nil")
}
