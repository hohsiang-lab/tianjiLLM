package callback

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDiscordUsageReportSender_UsesWaitBlocksMentionsChunksAndRedactsFailure(t *testing.T) {
	type request struct {
		wait     string
		payload  map[string]any
		content  string
		contentT string
	}
	var mu sync.Mutex
	var requests []request
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		var payload map[string]any
		require.NoError(t, json.Unmarshal(body, &payload))
		content, _ := payload["content"].(string)
		mu.Lock()
		requests = append(requests, request{
			wait:     r.URL.Query().Get("wait"),
			payload:  payload,
			content:  content,
			contentT: r.Header.Get("Content-Type"),
		})
		mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	sender := NewDiscordUsageReportSender(server.URL)
	sender.client = server.Client()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	parts := []string{
		"Tianji OpenAI credential usage operational snapshot (not billing) | part 1/2\n" + strings.Repeat("a", 1900),
		"Tianji OpenAI credential usage operational snapshot (not billing) | part 2/2\n" + strings.Repeat("b", 1900),
	}

	outcome := sender.Send(ctx, parts)

	require.Empty(t, outcome.FailureReason)
	assert.Equal(t, 2, outcome.AttemptedParts)
	assert.Equal(t, 2, outcome.DeliveredParts)
	mu.Lock()
	defer mu.Unlock()
	require.Len(t, requests, 2)
	for i, got := range requests {
		assert.Equal(t, "true", got.wait)
		assert.Contains(t, got.contentT, "application/json")
		assert.Equal(t, parts[i], got.content)
		assert.LessOrEqual(t, utf8.RuneCountInString(got.content), 2000)
		mentions, ok := got.payload["allowed_mentions"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, []any{}, mentions["parse"])
	}

	logs, restoreLogs := captureLog(t)
	defer restoreLogs()
	failure := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"Bearer secret.jwt.fixture","access_token":"access-secret"}`))
	}))
	defer failure.Close()
	failedSender := NewDiscordUsageReportSender(failure.URL)
	failedSender.client = failure.Client()

	failed := failedSender.Send(ctx, []string{"part 1/1"})

	assert.Equal(t, "http_status_500", failed.FailureReason)
	assert.NotContains(t, failed.FailureReason, "access-secret")
	assert.NotContains(t, logs.String(), "access-secret")
	assert.NotContains(t, logs.String(), "secret.jwt.fixture")
}

func TestDiscordUsageReportSender_RetriesOnlyWithinBoundedPolicy(t *testing.T) {
	t.Run("missing webhook skips", func(t *testing.T) {
		outcome := NewDiscordUsageReportSender("").Send(context.Background(), []string{"part 1/1"})
		assert.True(t, outcome.Skipped)
		assert.Zero(t, outcome.AttemptedParts)
	})

	t.Run("confirmed rate limit retries once when retry_after fits", func(t *testing.T) {
		var calls atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			if calls.Add(1) == 1 {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = w.Write([]byte(`{"retry_after":0}`))
				return
			}
			w.WriteHeader(http.StatusNoContent)
		}))
		defer server.Close()
		sender := NewDiscordUsageReportSender(server.URL)
		sender.client = server.Client()
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		outcome := sender.Send(ctx, []string{"part 1/1"})

		assert.Equal(t, int32(2), calls.Load())
		assert.Equal(t, 1, outcome.AttemptedParts)
		assert.Equal(t, 1, outcome.DeliveredParts)
		assert.Empty(t, outcome.FailureReason)
	})

	t.Run("rate limit outside the deadline does not retry", func(t *testing.T) {
		var calls atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			calls.Add(1)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"retry_after":1}`))
		}))
		defer server.Close()
		sender := NewDiscordUsageReportSender(server.URL)
		sender.client = server.Client()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		defer cancel()

		outcome := sender.Send(ctx, []string{"part 1/1"})

		assert.Equal(t, int32(1), calls.Load())
		assert.Equal(t, "rate_limited", outcome.FailureReason)
	})

	for _, status := range []int{http.StatusBadRequest, http.StatusInternalServerError} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls.Add(1)
				w.WriteHeader(status)
			}))
			defer server.Close()
			sender := NewDiscordUsageReportSender(server.URL)
			sender.client = server.Client()

			outcome := sender.Send(context.Background(), []string{"part 1/1"})

			assert.Equal(t, int32(1), calls.Load())
			assert.Equal(t, "http_status_"+strconv.Itoa(status), outcome.FailureReason)
		})
	}

	logs, restoreLogs := captureLog(t)
	defer restoreLogs()
	var transportCalls atomic.Int32
	sender := NewDiscordUsageReportSender("https://discord.example.invalid/webhook")
	sender.client = &http.Client{Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
		transportCalls.Add(1)
		return nil, errors.New("Bearer transport-secret")
	})}

	outcome := sender.Send(context.Background(), []string{"part 1/1"})

	assert.Equal(t, int32(1), transportCalls.Load())
	assert.Equal(t, "transport_failed", outcome.FailureReason)
	assert.NotContains(t, logs.String(), "transport-secret")
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}
