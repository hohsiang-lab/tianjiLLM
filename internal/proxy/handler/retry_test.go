package handler

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRetry_429_ThenSuccess: upstream 回 429 兩次後 200，client 拿到 200。
func TestRetry_429_ThenSuccess(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount < 3 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"choices":[{"message":{"content":"ok"}}]}`)
	}))
	defer server.Close()

	resp, err := doUpstreamWithRetry(t.Context(), server.Client(), func() (*http.Request, error) {
		return http.NewRequestWithContext(t.Context(), http.MethodPost, server.URL, strings.NewReader(`{}`))
	}, 2)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, 3, callCount, "expected 3 calls (2 retries + 1 success)")
}

// TestRetry_503_ThenSuccess: upstream 回 503 一次後成功。
func TestRetry_503_ThenSuccess(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	resp, err := doUpstreamWithRetry(t.Context(), server.Client(), func() (*http.Request, error) {
		return http.NewRequestWithContext(t.Context(), http.MethodPost, server.URL, strings.NewReader(`{}`))
	}, 2)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, 2, callCount)
}

// TestRetry_ExceedMaxRetries: 超過 maxRetries，回傳最後的錯誤 response。
func TestRetry_ExceedMaxRetries(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusTooManyRequests)
		fmt.Fprint(w, `{"error":"rate limited"}`)
	}))
	defer server.Close()

	resp, err := doUpstreamWithRetry(t.Context(), server.Client(), func() (*http.Request, error) {
		return http.NewRequestWithContext(t.Context(), http.MethodPost, server.URL, strings.NewReader(`{}`))
	}, 2)
	require.NoError(t, err)
	assert.Equal(t, http.StatusTooManyRequests, resp.StatusCode)
	assert.Equal(t, 3, callCount, "original + 2 retries = 3 calls")
	body, _ := io.ReadAll(resp.Body)
	assert.Contains(t, string(body), "rate limited")
}

// TestRetry_NoRetryOn400: 400 不 retry。
func TestRetry_NoRetryOn400(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	resp, err := doUpstreamWithRetry(t.Context(), server.Client(), func() (*http.Request, error) {
		return http.NewRequestWithContext(t.Context(), http.MethodPost, server.URL, strings.NewReader(`{}`))
	}, 2)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Equal(t, 1, callCount, "400 should not be retried")
}

// TestRetry_NoRetryOn401: 401 不 retry。
func TestRetry_NoRetryOn401(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	resp, err := doUpstreamWithRetry(t.Context(), server.Client(), func() (*http.Request, error) {
		return http.NewRequestWithContext(t.Context(), http.MethodPost, server.URL, strings.NewReader(`{}`))
	}, 2)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	assert.Equal(t, 1, callCount, "401 should not be retried")
}

// TestRetry_RetryAfterHeader: Retry-After header 覆蓋 backoff。
func TestRetry_RetryAfterHeader(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	start := time.Now()
	resp, err := doUpstreamWithRetry(t.Context(), server.Client(), func() (*http.Request, error) {
		return http.NewRequestWithContext(t.Context(), http.MethodPost, server.URL, strings.NewReader(`{}`))
	}, 2)
	elapsed := time.Since(start)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.GreaterOrEqual(t, elapsed, time.Second, "should respect Retry-After: 1")
}

// TestRetry_ZeroRetries: maxRetries=0 不 retry，只打一次。
func TestRetry_ZeroRetries(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	resp, err := doUpstreamWithRetry(t.Context(), server.Client(), func() (*http.Request, error) {
		return http.NewRequestWithContext(t.Context(), http.MethodPost, server.URL, strings.NewReader(`{}`))
	}, 0)
	require.NoError(t, err)
	assert.Equal(t, http.StatusTooManyRequests, resp.StatusCode)
	assert.Equal(t, 1, callCount, "maxRetries=0 = single attempt")
}
