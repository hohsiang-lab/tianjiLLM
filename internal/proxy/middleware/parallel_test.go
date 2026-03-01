package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParallelRequestLimiter_NilRedis(t *testing.T) {
	p := NewParallelRequestLimiter(nil)

	// Check: nil redis → always allowed
	ok, err := p.Check(context.Background(), "k1", 5)
	require.NoError(t, err)
	assert.True(t, ok)

	// Check: limit ≤ 0 → always allowed
	ok, err = p.Check(context.Background(), "k1", 0)
	require.NoError(t, err)
	assert.True(t, ok)

	// Release: nil redis → no panic
	p.Release(context.Background(), "k1")
}

func TestNewParallelRequestMiddleware_NilLimiter(t *testing.T) {
	mw := NewParallelRequestMiddleware(nil)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	mw(next).ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestNewParallelRequestMiddleware_NoTokenOrLimit(t *testing.T) {
	p := NewParallelRequestLimiter(nil) // nil redis so Check always passes
	mw := NewParallelRequestMiddleware(p)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	// No tokenHash or limit in context → passthrough
	w := httptest.NewRecorder()
	mw(next).ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestNewParallelRequestMiddleware_WithTokenAndLimit(t *testing.T) {
	p := NewParallelRequestLimiter(nil) // nil redis → always allowed
	mw := NewParallelRequestMiddleware(p)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	ctx := context.WithValue(context.Background(), tokenHashKey, "hash-abc")
	ctx = context.WithValue(ctx, maxParallelLimitKey, 10)
	req := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx)
	w := httptest.NewRecorder()
	mw(next).ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}
