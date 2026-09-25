package callback

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// syncBuffer is a bytes.Buffer protected by a mutex, safe for concurrent writes and reads.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (n int, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.String()
}

// captureLog redirects the standard logger to a thread-safe buffer for the duration of the test.
// Returns the buffer and a cleanup function that restores the original output.
func captureLog(t *testing.T) (*syncBuffer, func()) {
	t.Helper()
	buf := &syncBuffer{}
	old := log.Writer()
	log.SetOutput(buf)
	return buf, func() { log.SetOutput(old) }
}

// newMockDiscordServer creates an httptest server that records received payloads.
// Returns the server and a function that returns all received content strings.
func newMockDiscordServer(t *testing.T, statusCode int) (*httptest.Server, func() []string) {
	t.Helper()
	var mu sync.Mutex
	var msgs []string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var payload map[string]string
		if err := json.Unmarshal(body, &payload); err == nil {
			mu.Lock()
			msgs = append(msgs, payload["content"])
			mu.Unlock()
		}
		w.WriteHeader(statusCode)
		if statusCode >= 300 {
			_, _ = w.Write([]byte("webhook error"))
		}
	}))

	return srv, func() []string {
		mu.Lock()
		defer mu.Unlock()
		cp := make([]string, len(msgs))
		copy(cp, msgs)
		return cp
	}
}

// legacyState builds an AnthropicOAuthRateLimitState with only legacy API key fields set
// (no UnifiedStatus). Used to test the legacy alert path.
func legacyState(tokenKey string, requestsLimit, requestsRemaining, tokensLimit, tokensRemaining int) AnthropicOAuthRateLimitState {
	return AnthropicOAuthRateLimitState{
		TokenKey:                   tokenKey,
		RequestsLimit:              requestsLimit,
		RequestsRemaining:          requestsRemaining,
		TokensLimit:                tokensLimit,
		TokensRemaining:            tokensRemaining,
		Unified5hUtilization:       -1,
		Unified7dUtilization:       -1,
		Unified7dSonnetUtilization: -1,
		FallbackPercentage:         -1,
	}
}

// oauthState builds an AnthropicOAuthRateLimitState with unified OAuth fields set.
func oauthState(tokenKey string, status string, util5h, util7d float64) AnthropicOAuthRateLimitState {
	return AnthropicOAuthRateLimitState{
		TokenKey:                   tokenKey,
		UnifiedStatus:              status,
		Unified5hUtilization:       util5h,
		Unified7dUtilization:       util7d,
		Unified7dSonnetUtilization: -1,
		FallbackPercentage:         -1,
	}
}

// --- Legacy API key path ---

func TestCheckAndAlert_Legacy_TokensBelowThreshold_AlertFires(t *testing.T) {
	srv, getMsgs := newMockDiscordServer(t, 200)
	defer srv.Close()

	a := NewDiscordRateLimitAlerter(srv.URL, 0.2)
	require.NotNil(t, a)
	a.cooldown = 0

	// tokens: 1000/10000 = 10% — below 20% threshold
	state := legacyState("testkey", 1000, 900, 10000, 1000)
	a.CheckAndAlert(state)
	time.Sleep(100 * time.Millisecond)

	msgs := getMsgs()
	require.Len(t, msgs, 1, "one alert should fire for tokens below threshold")
	assert.Contains(t, msgs[0], "tokens", "alert message must mention tokens")
}

func TestCheckAndAlert_Legacy_RequestsBelowThreshold_AlertFires(t *testing.T) {
	srv, getMsgs := newMockDiscordServer(t, 200)
	defer srv.Close()

	a := NewDiscordRateLimitAlerter(srv.URL, 0.2)
	require.NotNil(t, a)
	a.cooldown = 0

	// requests: 100/1000 = 10% — below threshold; tokens: 5000/10000 = 50% — above
	state := legacyState("testkey", 1000, 100, 10000, 5000)
	a.CheckAndAlert(state)
	time.Sleep(100 * time.Millisecond)

	msgs := getMsgs()
	require.Len(t, msgs, 1, "one alert should fire for requests below threshold")
	assert.Contains(t, msgs[0], "requests", "alert message must mention requests")
}

func TestCheckAndAlert_Legacy_AboveThreshold_NoAlert(t *testing.T) {
	srv, getMsgs := newMockDiscordServer(t, 200)
	defer srv.Close()

	a := NewDiscordRateLimitAlerter(srv.URL, 0.2)
	require.NotNil(t, a)
	a.cooldown = 0

	// requests: 800/1000 = 80%, tokens: 8000/10000 = 80% — both above threshold
	state := legacyState("testkey", 1000, 800, 10000, 8000)
	a.CheckAndAlert(state)
	time.Sleep(50 * time.Millisecond)

	assert.Len(t, getMsgs(), 0, "no alert when all metrics are above threshold")
}

// --- OAuth token path ---

func TestCheckAndAlert_OAuth_5hUtilizationAboveThreshold_AlertFires(t *testing.T) {
	srv, getMsgs := newMockDiscordServer(t, 200)
	defer srv.Close()

	a := NewDiscordRateLimitAlerter(srv.URL, 0.2)
	require.NotNil(t, a)
	a.cooldown = 0

	state := oauthState("a3bb64f70a1d", UnifiedStatusAllowed, 0.85, 0.30)
	a.CheckAndAlert(state)
	time.Sleep(100 * time.Millisecond)

	msgs := getMsgs()
	require.Len(t, msgs, 2, "both 5h and 7d alerts should fire")
	combined := strings.Join(msgs, " ")
	assert.Contains(t, combined, "5h", "alert must mention 5h utilization")
}

func TestCheckAndAlert_OAuth_7dUtilizationAboveThreshold_AlertFires(t *testing.T) {
	srv, getMsgs := newMockDiscordServer(t, 200)
	defer srv.Close()

	a := NewDiscordRateLimitAlerter(srv.URL, 0.2)
	require.NotNil(t, a)
	a.cooldown = 0

	// 5h below threshold (3%), 7d above threshold (85%) — only 7d alert fires
	state := oauthState("a3bb64f70a1d", UnifiedStatusAllowed, 0.03, 0.85)
	a.CheckAndAlert(state)
	time.Sleep(100 * time.Millisecond)

	msgs := getMsgs()
	require.Len(t, msgs, 1, "only 7d alert fires when 5h is below threshold")
	assert.Contains(t, msgs[0], "7d", "alert must mention 7d utilization")
}

// HO-490: renamed from RateLimited + Overage — merged into single Rejected test
func TestCheckAndAlert_OAuth_Rejected_AlertFiresAndReturns(t *testing.T) {
	srv, getMsgs := newMockDiscordServer(t, 200)
	defer srv.Close()

	a := NewDiscordRateLimitAlerter(srv.URL, 0.2)
	require.NotNil(t, a)
	a.cooldown = 0

	state := oauthState("a3bb64f70a1d", UnifiedStatusRejected, 1.0, 1.0)
	a.CheckAndAlert(state)
	time.Sleep(100 * time.Millisecond)

	msgs := getMsgs()
	// rejected fires once and returns — no additional utilization alerts
	require.Len(t, msgs, 1, "only rejected alert fires (early return)")
	assert.Contains(t, msgs[0], "rejected", "alert must mention rejected status")
}

func TestCheckAndAlert_OAuth_AllowedBelowThreshold_NoAlert(t *testing.T) {
	srv, getMsgs := newMockDiscordServer(t, 200)
	defer srv.Close()

	a := NewDiscordRateLimitAlerter(srv.URL, 0.2)
	require.NotNil(t, a)
	a.cooldown = 0

	// 3% and 17% — both below 20% threshold
	state := oauthState("a3bb64f70a1d", UnifiedStatusAllowed, 0.03, 0.17)
	a.CheckAndAlert(state)
	time.Sleep(50 * time.Millisecond)

	assert.Len(t, getMsgs(), 0, "no alert when utilization is below threshold")
}

func TestCheckAndAlert_OAuth_MissingSentinel_NoAlert(t *testing.T) {
	srv, getMsgs := newMockDiscordServer(t, 200)
	defer srv.Close()

	a := NewDiscordRateLimitAlerter(srv.URL, 0.2)
	require.NotNil(t, a)
	a.cooldown = 0

	// -1 sentinel means no data — skip check
	state := oauthState("a3bb64f70a1d", UnifiedStatusAllowed, -1, -1)
	a.CheckAndAlert(state)
	time.Sleep(50 * time.Millisecond)

	assert.Len(t, getMsgs(), 0, "no alert when utilization data is missing (sentinel -1)")
}

// --- Cooldown ---

func TestCheckAndAlert_Cooldown_SameKeyFiredOnlyOnce(t *testing.T) {
	var callCount int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		w.WriteHeader(200)
	}))
	defer srv.Close()

	a := NewDiscordRateLimitAlerter(srv.URL, 0.2)
	require.NotNil(t, a)
	// Use default 1h cooldown

	state := legacyState("testkey", 1000, 100, 10000, 1000) // both below threshold

	for range 10 {
		a.CheckAndAlert(state)
	}
	time.Sleep(150 * time.Millisecond)

	assert.Equal(t, int32(2), atomic.LoadInt32(&callCount),
		"cooldown: requests and tokens each fire once within 1h")
}

func TestCheckAndAlert_Cooldown_IndependentKeysPerMetric(t *testing.T) {
	var callCount int32
	var mu sync.Mutex
	var contents []string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var payload map[string]string
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("failed to unmarshal payload: %v", err)
		}
		atomic.AddInt32(&callCount, 1)
		mu.Lock()
		contents = append(contents, payload["content"])
		mu.Unlock()
		w.WriteHeader(200)
	}))
	defer srv.Close()

	a := NewDiscordRateLimitAlerter(srv.URL, 0.2)
	require.NotNil(t, a)
	// Default 1h cooldown

	// First: requests below, tokens above → requests alert fires
	requestsOnly := legacyState("testkey", 1000, 100, 10000, 8000)
	a.CheckAndAlert(requestsOnly)
	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, int32(1), atomic.LoadInt32(&callCount), "requests alert fires first")

	// Second: requests still below (cooldown), tokens now also below → tokens alert fires
	both := legacyState("testkey", 1000, 100, 10000, 1000)
	a.CheckAndAlert(both)
	time.Sleep(100 * time.Millisecond)

	assert.Equal(t, int32(2), atomic.LoadInt32(&callCount),
		"tokens alert fires independently; requests suppressed by cooldown")

	mu.Lock()
	defer mu.Unlock()
	hasTokens := false
	for _, c := range contents {
		if strings.Contains(c, "tokens") {
			hasTokens = true
		}
	}
	assert.True(t, hasTokens, "second alert should be for tokens")
}

// --- Discord error handling ---

func TestCheckAndAlert_DiscordNon2xx_LogsErrorDoesNotPanic(t *testing.T) {
	logBuf, restore := captureLog(t)
	defer restore()

	srv, _ := newMockDiscordServer(t, 429) // Discord rate limited
	defer srv.Close()

	a := NewDiscordRateLimitAlerter(srv.URL, 0.2)
	require.NotNil(t, a)
	a.cooldown = 0

	state := legacyState("testkey", 1000, 100, 10000, 500)

	// Must not panic even when Discord returns non-2xx.
	a.CheckAndAlert(state)
	time.Sleep(200 * time.Millisecond)

	logged := logBuf.String()
	assert.Contains(t, logged, "429", "error log must include the HTTP status code")
	assert.Contains(t, logged, "ERROR", "log must include ERROR level indicator")
}

func TestCheckAndAlert_DiscordFailure_CooldownClearedForRetry(t *testing.T) {
	var callCount int32
	var shouldFail atomic.Bool
	shouldFail.Store(true)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		if shouldFail.Load() {
			w.WriteHeader(500)
		} else {
			w.WriteHeader(200)
		}
	}))
	defer srv.Close()

	a := NewDiscordRateLimitAlerter(srv.URL, 0.2)
	require.NotNil(t, a)
	// Use default 1h cooldown

	state := legacyState("testkey", 1000, 100, 10000, 8000) // only requests below threshold

	// First call: Discord returns 500 — cooldown entry is cleared
	a.CheckAndAlert(state)
	time.Sleep(150 * time.Millisecond)
	assert.Equal(t, int32(1), atomic.LoadInt32(&callCount), "first attempt fires")

	// Second call: cooldown was cleared on failure, so alert fires again
	shouldFail.Store(false)
	a.CheckAndAlert(state)
	time.Sleep(150 * time.Millisecond)
	assert.Equal(t, int32(2), atomic.LoadInt32(&callCount), "retry fires after failure clears cooldown")

	// Third call: second attempt succeeded, cooldown is now active
	a.CheckAndAlert(state)
	time.Sleep(150 * time.Millisecond)
	assert.Equal(t, int32(2), atomic.LoadInt32(&callCount), "no third alert within cooldown window")
}

// --- Instantiation ---

func TestNewDiscordRateLimitAlerter_EmptyWebhookURL_ReturnsNil(t *testing.T) {
	a := NewDiscordRateLimitAlerter("", 0.2)
	assert.Nil(t, a, "alerter must be nil when webhook URL is empty")
}

func TestNewDiscordRateLimitAlerter_EmptyWebhookURL_NoHTTPCall(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(200)
	}))
	defer srv.Close()

	a := NewDiscordRateLimitAlerter("", 0.2)
	assert.Nil(t, a)

	if a != nil {
		a.CheckAndAlert(legacyState("testkey", 1000, 100, 10000, 500))
		time.Sleep(50 * time.Millisecond)
	}

	assert.Equal(t, 0, callCount, "no HTTP calls should be made when alerter is nil")
}
