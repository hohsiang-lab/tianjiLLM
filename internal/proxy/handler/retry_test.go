package handler

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// mockUpstream returns an httptest.Server that replies with the given status
// codes in order: first call → codes[0], second → codes[1], …
// After all codes are exhausted it keeps returning the last one.
// body is the response body for the final successful response.
func mockUpstream(codes []int, body string) *httptest.Server {
	var call int64
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		idx := int(atomic.AddInt64(&call, 1)) - 1
		if idx >= len(codes) {
			idx = len(codes) - 1
		}
		code := codes[idx]
		if code == http.StatusOK {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(code)
			fmt.Fprint(w, body)
		} else {
			w.WriteHeader(code)
			fmt.Fprintf(w, `{"error":"status %d"}`, code)
		}
	}))
}

// mockUpstreamWithRetryAfter is like mockUpstream but sets Retry-After header
// on 429 responses.
func mockUpstreamWithRetryAfter(codes []int, retryAfterSecs string, body string) *httptest.Server {
	var call int64
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		idx := int(atomic.AddInt64(&call, 1)) - 1
		if idx >= len(codes) {
			idx = len(codes) - 1
		}
		code := codes[idx]
		if code == http.StatusTooManyRequests {
			w.Header().Set("Retry-After", retryAfterSecs)
		}
		if code == http.StatusOK {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(code)
			fmt.Fprint(w, body)
		} else {
			w.WriteHeader(code)
			fmt.Fprintf(w, `{"error":"status %d"}`, code)
		}
	}))
}

// buildReqTo returns a buildReq function targeting the given URL.
func buildReqTo(url string) func() (*http.Request, error) {
	return func() (*http.Request, error) {
		return http.NewRequest(http.MethodPost, url, bytes.NewReader([]byte(`{"prompt":"test"}`)))
	}
}

// ---------------------------------------------------------------------------
// AC-1: retryable status codes
// ---------------------------------------------------------------------------

func TestDoUpstreamWithRetry_RetryOn429(t *testing.T) {
	srv := mockUpstream([]int{429, 200}, `{"ok":true}`)
	defer srv.Close()

	resp, err := doUpstreamWithRetry(context.Background(), http.DefaultClient, buildReqTo(srv.URL), 2)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestDoUpstreamWithRetry_RetryOn500(t *testing.T) {
	srv := mockUpstream([]int{500, 200}, `{"ok":true}`)
	defer srv.Close()

	resp, err := doUpstreamWithRetry(context.Background(), http.DefaultClient, buildReqTo(srv.URL), 2)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestDoUpstreamWithRetry_RetryOn502(t *testing.T) {
	srv := mockUpstream([]int{502, 200}, `{"ok":true}`)
	defer srv.Close()

	resp, err := doUpstreamWithRetry(context.Background(), http.DefaultClient, buildReqTo(srv.URL), 2)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestDoUpstreamWithRetry_RetryOn503(t *testing.T) {
	srv := mockUpstream([]int{503, 200}, `{"ok":true}`)
	defer srv.Close()

	resp, err := doUpstreamWithRetry(context.Background(), http.DefaultClient, buildReqTo(srv.URL), 2)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestDoUpstreamWithRetry_RetryOn504(t *testing.T) {
	srv := mockUpstream([]int{504, 200}, `{"ok":true}`)
	defer srv.Close()

	resp, err := doUpstreamWithRetry(context.Background(), http.DefaultClient, buildReqTo(srv.URL), 2)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestDoUpstreamWithRetry_NoRetryOn200(t *testing.T) {
	var calls int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&calls, 1)
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"ok":true}`)
	}))
	defer srv.Close()

	resp, err := doUpstreamWithRetry(context.Background(), http.DefaultClient, buildReqTo(srv.URL), 2)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, int64(1), atomic.LoadInt64(&calls), "200 should not trigger retry")
}

func TestDoUpstreamWithRetry_NoRetryOn400(t *testing.T) {
	var calls int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&calls, 1)
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"error":"bad request"}`)
	}))
	defer srv.Close()

	resp, err := doUpstreamWithRetry(context.Background(), http.DefaultClient, buildReqTo(srv.URL), 2)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Equal(t, int64(1), atomic.LoadInt64(&calls), "400 should not trigger retry")
}

func TestDoUpstreamWithRetry_NoRetryOn401(t *testing.T) {
	var calls int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&calls, 1)
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"error":"unauthorized"}`)
	}))
	defer srv.Close()

	resp, err := doUpstreamWithRetry(context.Background(), http.DefaultClient, buildReqTo(srv.URL), 2)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	assert.Equal(t, int64(1), atomic.LoadInt64(&calls), "401 should not trigger retry")
}

// ---------------------------------------------------------------------------
// AC-2: exponential backoff
// ---------------------------------------------------------------------------

func TestDoUpstreamWithRetry_ExponentialBackoff(t *testing.T) {
	// Override the package-level base delay to keep the test fast.
	origDelay := baseRetryDelay
	baseRetryDelay = 10 * time.Millisecond
	defer func() { baseRetryDelay = origDelay }()

	var timestamps []time.Time
	var call int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		timestamps = append(timestamps, time.Now())
		idx := int(atomic.AddInt64(&call, 1)) - 1
		if idx < 2 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"ok":true}`)
	}))
	defer srv.Close()

	resp, err := doUpstreamWithRetry(context.Background(), http.DefaultClient, buildReqTo(srv.URL), 3)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// We expect 3 calls: initial + 2 retries.
	require.Len(t, timestamps, 3)

	// Gap between call 1 and 2 ≈ baseDelay (10ms).
	// Gap between call 2 and 3 ≈ 2*baseDelay (20ms).
	gap1 := timestamps[1].Sub(timestamps[0])
	gap2 := timestamps[2].Sub(timestamps[1])
	assert.True(t, gap2 > gap1, "second backoff (%v) should be longer than first (%v)", gap2, gap1)
}

func TestDoUpstreamWithRetry_RetryAfterHeader(t *testing.T) {
	// Override base delay.
	origDelay := baseRetryDelay
	baseRetryDelay = 10 * time.Millisecond
	defer func() { baseRetryDelay = origDelay }()

	var timestamps []time.Time
	var call int64
	// First response: 429 with Retry-After: 1 (in seconds, but we check
	// the function respects it rather than using exponential backoff).
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		timestamps = append(timestamps, time.Now())
		idx := int(atomic.AddInt64(&call, 1)) - 1
		if idx == 0 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"ok":true}`)
	}))
	defer srv.Close()

	resp, err := doUpstreamWithRetry(context.Background(), http.DefaultClient, buildReqTo(srv.URL), 2)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	require.Len(t, timestamps, 2)
	gap := timestamps[1].Sub(timestamps[0])
	// Retry-After says 1 second — the gap should be at least ~900ms
	// (allowing some tolerance) and definitely more than the base 10ms.
	assert.True(t, gap >= 900*time.Millisecond, "Retry-After:1 should wait ~1s, got %v", gap)
}

// ---------------------------------------------------------------------------
// AC-3: configuration
// ---------------------------------------------------------------------------

func TestDoUpstreamWithRetry_DefaultMaxRetries(t *testing.T) {
	// Verify that a Handlers struct without explicit MaxUpstreamRetries
	// defaults to 2.
	h := &Handlers{Config: nil}
	assert.Equal(t, 2, h.maxRetries(), "default MaxUpstreamRetries should be 2")
}

func TestDoUpstreamWithRetry_ZeroRetries_NoRetry(t *testing.T) {
	var calls int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&calls, 1)
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	// maxRetries=0 means no retry at all — just the initial attempt.
	resp, err := doUpstreamWithRetry(context.Background(), http.DefaultClient, buildReqTo(srv.URL), 0)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusTooManyRequests, resp.StatusCode)
	assert.Equal(t, int64(1), atomic.LoadInt64(&calls), "zero retries means exactly 1 attempt")
}

// ---------------------------------------------------------------------------
// AC-4: log format
// ---------------------------------------------------------------------------

func TestDoUpstreamWithRetry_LogFormat(t *testing.T) {
	origDelay := baseRetryDelay
	baseRetryDelay = time.Millisecond
	defer func() { baseRetryDelay = origDelay }()

	srv := mockUpstream([]int{429, 200}, `{"ok":true}`)
	defer srv.Close()

	var buf bytes.Buffer
	oldLogger := log.Default()
	log.SetOutput(&buf)
	defer log.SetOutput(oldLogger.Writer())

	resp, err := doUpstreamWithRetry(context.Background(), http.DefaultClient, buildReqTo(srv.URL), 2)
	require.NoError(t, err)
	defer resp.Body.Close()

	logOutput := buf.String()
	// Expect log line containing: [retry] attempt 1/2 status=429 waiting=...
	assert.True(t, strings.Contains(logOutput, "[retry]"), "log should contain [retry] prefix, got: %s", logOutput)
	assert.True(t, strings.Contains(logOutput, "attempt 1/2"), "log should contain attempt number, got: %s", logOutput)
	assert.True(t, strings.Contains(logOutput, "status=429"), "log should contain status code, got: %s", logOutput)
	assert.True(t, strings.Contains(logOutput, "waiting="), "log should contain waiting duration, got: %s", logOutput)
}
