package callback

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// captureLog redirects the standard logger to a buffer for the duration of the test.
// Returns the buffer and a cleanup function that restores the original output.
func captureLog(t *testing.T) (*bytes.Buffer, func()) {
	t.Helper()
	buf := &bytes.Buffer{}
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
			w.Write([]byte("webhook error"))
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

// makeFullHeaders returns a valid set of all Anthropic rate limit headers.
func makeFullHeaders(inputLimit, inputRemaining int64, inputReset string,
	outputLimit, outputRemaining int64, outputReset string) http.Header {
	h := http.Header{}
	h.Set("anthropic-ratelimit-input-tokens-limit", itoa(inputLimit))
	h.Set("anthropic-ratelimit-input-tokens-remaining", itoa(inputRemaining))
	h.Set("anthropic-ratelimit-input-tokens-reset", inputReset)
	h.Set("anthropic-ratelimit-output-tokens-limit", itoa(outputLimit))
	h.Set("anthropic-ratelimit-output-tokens-remaining", itoa(outputRemaining))
	h.Set("anthropic-ratelimit-output-tokens-reset", outputReset)
	h.Set("anthropic-ratelimit-requests-limit", "1000")
	h.Set("anthropic-ratelimit-requests-remaining", "900")
	h.Set("anthropic-ratelimit-requests-reset", "2026-03-01T02:00:00Z")
	return h
}

func itoa(n int64) string {
	return strconv.FormatInt(n, 10)
}

// --- FR-003 / SC-007: missing header → error log, check skipped ---

func TestParseAnthropicHeaders_MissingInputHeader_LogsErrorAndSkipsInputCheck(t *testing.T) {
	// FR-003 / SC-007: When a required input-token header is absent,
	// ParseAnthropicRateLimitHeaders logs an error and sets InputParsed=false.
	logBuf, restore := captureLog(t)
	defer restore()

	h := http.Header{}
	// Deliberately omit all input-token headers; provide output headers only.
	h.Set("anthropic-ratelimit-output-tokens-limit", "8000")
	h.Set("anthropic-ratelimit-output-tokens-remaining", "1500")
	h.Set("anthropic-ratelimit-output-tokens-reset", "2026-03-01T02:00:00Z")
	h.Set("anthropic-ratelimit-requests-limit", "1000")
	h.Set("anthropic-ratelimit-requests-remaining", "900")
	h.Set("anthropic-ratelimit-requests-reset", "2026-03-01T02:00:00Z")

	state := ParseAnthropicRateLimitHeaders(h)

	// Input check must be skipped (InputParsed=false)
	assert.False(t, state.InputParsed, "InputParsed should be false when input headers are missing")

	// Error must be logged for missing header
	logged := logBuf.String()
	assert.Contains(t, logged, "anthropic-ratelimit-input-tokens-limit",
		"error log must mention the missing header name")
	assert.Contains(t, logged, "ERROR", "log must include ERROR level indicator")
}

// --- FR-004 / SC-008: header present but non-integer → error log with raw value, check skipped ---

func TestParseAnthropicHeaders_UnparseableInputHeader_LogsErrorWithRawValue(t *testing.T) {
	// FR-004 / SC-008
	logBuf, restore := captureLog(t)
	defer restore()

	h := makeFullHeaders(10000, 1000, "2026-03-01T02:00:00Z", 8000, 1500, "2026-03-01T02:00:00Z")
	// Overwrite with a non-integer value
	h.Set("anthropic-ratelimit-input-tokens-remaining", "not-a-number")

	state := ParseAnthropicRateLimitHeaders(h)

	assert.False(t, state.InputParsed, "InputParsed must be false for unparseable header")

	logged := logBuf.String()
	assert.Contains(t, logged, "not-a-number", "error log must include the raw invalid value")
	assert.Contains(t, logged, "anthropic-ratelimit-input-tokens-remaining",
		"error log must include the header name")
}

func TestParseAnthropicHeaders_UnparseableOutputHeader_LogsErrorWithRawValue(t *testing.T) {
	// FR-004 / SC-008 (output variant)
	logBuf, restore := captureLog(t)
	defer restore()

	h := makeFullHeaders(10000, 4000, "2026-03-01T02:00:00Z", 8000, 1500, "2026-03-01T02:00:00Z")
	h.Set("anthropic-ratelimit-output-tokens-limit", "bad_value")

	state := ParseAnthropicRateLimitHeaders(h)

	assert.False(t, state.OutputParsed, "OutputParsed must be false for unparseable header")

	logged := logBuf.String()
	assert.Contains(t, logged, "bad_value", "error log must include raw invalid value")
}

// --- FR-005: input and output are independent checks ---

func TestCheckAndAlert_InputAboveOutputBelow_OnlyOutputAlertFires(t *testing.T) {
	// FR-005: input above threshold (40%), output below (18.75%) → only output alert fires.
	srv, getMsgs := newMockDiscordServer(t, 200)
	defer srv.Close()

	a := NewDiscordRateLimitAlerter(srv.URL, 0.2)
	require.NotNil(t, a)
	a.cooldown = 0 // disable cooldown for test

	state := AnthropicRateLimitState{
		InputTokensLimit:      10000,
		InputTokensRemaining:  4000, // 40% – above threshold
		InputTokensReset:      "2026-03-01T02:00:00Z",
		InputParsed:           true,
		OutputTokensLimit:     8000,
		OutputTokensRemaining: 1500, // 18.75% – below threshold
		OutputTokensReset:     "2026-03-01T02:00:00Z",
		OutputParsed:          true,
	}

	a.CheckAndAlert(state)
	time.Sleep(100 * time.Millisecond)

	msgs := getMsgs()
	require.Len(t, msgs, 1, "exactly one alert should fire (output only)")
	assert.Contains(t, msgs[0], "output", "alert should mention output token type")
}

// --- FR-006 / SC-001: input below threshold → Discord POST with exact values ---

func TestCheckAndAlert_InputBelowThreshold_PostsDiscordWithExactValues(t *testing.T) {
	// FR-006 / SC-001
	srv, getMsgs := newMockDiscordServer(t, 200)
	defer srv.Close()

	a := NewDiscordRateLimitAlerter(srv.URL, 0.2)
	require.NotNil(t, a)
	a.cooldown = 0

	const (
		inputLimit     = int64(10000)
		inputRemaining = int64(1000) // 10% – below 20% threshold
		inputReset     = "2026-03-01T02:00:00Z"
	)

	state := AnthropicRateLimitState{
		InputTokensLimit:      inputLimit,
		InputTokensRemaining:  inputRemaining,
		InputTokensReset:      inputReset,
		InputParsed:           true,
		OutputTokensLimit:     8000,
		OutputTokensRemaining: 4000, // 50% – above threshold
		OutputTokensReset:     "2026-03-01T03:00:00Z",
		OutputParsed:          true,
	}

	a.CheckAndAlert(state)
	time.Sleep(100 * time.Millisecond)

	msgs := getMsgs()
	require.Len(t, msgs, 1, "exactly one alert should fire")

	content := msgs[0]
	assert.Contains(t, content, "10000", "message must contain exact InputTokensLimit")
	assert.Contains(t, content, "1000", "message must contain exact InputTokensRemaining")
	assert.Contains(t, content, inputReset, "message must contain exact InputTokensReset")
}

// --- FR-007 / SC-002: output below threshold → Discord POST with exact values ---

func TestCheckAndAlert_OutputBelowThreshold_PostsDiscordWithExactValues(t *testing.T) {
	// FR-007 / SC-002
	srv, getMsgs := newMockDiscordServer(t, 200)
	defer srv.Close()

	a := NewDiscordRateLimitAlerter(srv.URL, 0.2)
	require.NotNil(t, a)
	a.cooldown = 0

	const (
		outputLimit     = int64(8000)
		outputRemaining = int64(1500) // 18.75% – below threshold
		outputReset     = "2026-03-01T04:00:00Z"
	)

	state := AnthropicRateLimitState{
		InputTokensLimit:      10000,
		InputTokensRemaining:  4000, // above threshold
		InputTokensReset:      "2026-03-01T02:00:00Z",
		InputParsed:           true,
		OutputTokensLimit:     outputLimit,
		OutputTokensRemaining: outputRemaining,
		OutputTokensReset:     outputReset,
		OutputParsed:          true,
	}

	a.CheckAndAlert(state)
	time.Sleep(100 * time.Millisecond)

	msgs := getMsgs()
	require.Len(t, msgs, 1, "exactly one alert should fire")

	content := msgs[0]
	assert.Contains(t, content, "8000", "message must contain exact OutputTokensLimit")
	assert.Contains(t, content, "1500", "message must contain exact OutputTokensRemaining")
	assert.Contains(t, content, outputReset, "message must contain exact OutputTokensReset")
}

// --- FR-008 / SC-003: cooldown – same key fires only once per hour ---

func TestCheckAndAlert_Cooldown_SameKeyFiredOnlyOnce(t *testing.T) {
	// FR-008 / SC-003: 100 triggering responses within 1h → exactly 1 alert.
	var callCount int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		w.WriteHeader(200)
	}))
	defer srv.Close()

	a := NewDiscordRateLimitAlerter(srv.URL, 0.2)
	require.NotNil(t, a)
	// Use default 1h cooldown (do NOT set to 0)

	state := AnthropicRateLimitState{
		InputTokensLimit:     10000,
		InputTokensRemaining: 500, // 5% – below threshold
		InputTokensReset:     "2026-03-01T02:00:00Z",
		InputParsed:          true,
		// Output above threshold – won't trigger
		OutputTokensLimit:     8000,
		OutputTokensRemaining: 6000,
		OutputTokensReset:     "2026-03-01T02:00:00Z",
		OutputParsed:          true,
	}

	// Fire 10 triggering responses (simulating 100; keeping test fast)
	for i := 0; i < 10; i++ {
		a.CheckAndAlert(state)
	}
	time.Sleep(150 * time.Millisecond)

	assert.Equal(t, int32(1), atomic.LoadInt32(&callCount),
		"cooldown: only first alert should fire within 1h")
}

// --- FR-008: input and output use independent cooldown keys ---

func TestCheckAndAlert_Cooldown_InputOutputIndependentKeys(t *testing.T) {
	// FR-008: input on cooldown should NOT suppress output alert.
	var inputCount, outputCount int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var payload map[string]string
		json.Unmarshal(body, &payload)
		content := payload["content"]
		if strings.Contains(content, "input") {
			atomic.AddInt32(&inputCount, 1)
		}
		if strings.Contains(content, "output") {
			atomic.AddInt32(&outputCount, 1)
		}
		w.WriteHeader(200)
	}))
	defer srv.Close()

	a := NewDiscordRateLimitAlerter(srv.URL, 0.2)
	require.NotNil(t, a)
	// Default 1h cooldown

	belowThreshold := AnthropicRateLimitState{
		InputTokensLimit:      10000,
		InputTokensRemaining:  500, // 5% – below
		InputTokensReset:      "2026-03-01T02:00:00Z",
		InputParsed:           true,
		OutputTokensLimit:     8000,
		OutputTokensRemaining: 400, // 5% – below
		OutputTokensReset:     "2026-03-01T02:00:00Z",
		OutputParsed:          true,
	}

	// First call: both input and output alert should fire.
	a.CheckAndAlert(belowThreshold)
	time.Sleep(150 * time.Millisecond)

	assert.Equal(t, int32(1), atomic.LoadInt32(&inputCount), "first input alert should fire")
	assert.Equal(t, int32(1), atomic.LoadInt32(&outputCount), "first output alert should fire")

	// Second call: both should be suppressed (on cooldown).
	a.CheckAndAlert(belowThreshold)
	time.Sleep(150 * time.Millisecond)

	assert.Equal(t, int32(1), atomic.LoadInt32(&inputCount), "input on cooldown – no second alert")
	assert.Equal(t, int32(1), atomic.LoadInt32(&outputCount), "output on cooldown – no second alert")
}

func TestCheckAndAlert_Cooldown_InputCoolingOutputFires(t *testing.T) {
	// FR-008: When input is on cooldown but output just crossed threshold, output alert fires.
	var callCount int32
	var mu sync.Mutex
	var contents []string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var payload map[string]string
		json.Unmarshal(body, &payload)
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

	// First: input below threshold only → input alert fires.
	inputOnly := AnthropicRateLimitState{
		InputTokensLimit:      10000,
		InputTokensRemaining:  500, // below
		InputTokensReset:      "2026-03-01T02:00:00Z",
		InputParsed:           true,
		OutputTokensLimit:     8000,
		OutputTokensRemaining: 6000, // above
		OutputTokensReset:     "2026-03-01T02:00:00Z",
		OutputParsed:          true,
	}
	a.CheckAndAlert(inputOnly)
	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, int32(1), atomic.LoadInt32(&callCount), "input alert fires first")

	// Second: input still below (cooldown), output now also below → output alert fires.
	both := AnthropicRateLimitState{
		InputTokensLimit:      10000,
		InputTokensRemaining:  500, // still below – but on cooldown
		InputTokensReset:      "2026-03-01T02:00:00Z",
		InputParsed:           true,
		OutputTokensLimit:     8000,
		OutputTokensRemaining: 400, // now below
		OutputTokensReset:     "2026-03-01T02:00:00Z",
		OutputParsed:          true,
	}
	a.CheckAndAlert(both)
	time.Sleep(100 * time.Millisecond)

	assert.Equal(t, int32(2), atomic.LoadInt32(&callCount),
		"output alert fires independently; input suppressed by cooldown")

	mu.Lock()
	defer mu.Unlock()
	hasOutput := false
	for _, c := range contents {
		if strings.Contains(c, "output") {
			hasOutput = true
		}
	}
	assert.True(t, hasOutput, "second alert should be for output token type")
}

// --- FR-011 / SC-009: Discord non-2xx → log.Errorf with status + body, proxy unaffected ---

func TestCheckAndAlert_DiscordNon2xx_LogsErrorDoesNotPanic(t *testing.T) {
	// FR-011 / SC-009
	logBuf, restore := captureLog(t)
	defer restore()

	srv, _ := newMockDiscordServer(t, 429) // Discord rate limited
	defer srv.Close()

	a := NewDiscordRateLimitAlerter(srv.URL, 0.2)
	require.NotNil(t, a)
	a.cooldown = 0

	state := AnthropicRateLimitState{
		InputTokensLimit:      10000,
		InputTokensRemaining:  500, // below threshold
		InputTokensReset:      "2026-03-01T02:00:00Z",
		InputParsed:           true,
		OutputTokensLimit:     8000,
		OutputTokensRemaining: 6000,
		OutputTokensReset:     "2026-03-01T02:00:00Z",
		OutputParsed:          true,
	}

	// CheckAndAlert must not panic even when Discord returns non-2xx
	assert.NotPanics(t, func() {
		a.CheckAndAlert(state)
		time.Sleep(150 * time.Millisecond) // wait for goroutine
	})

	logged := logBuf.String()
	assert.Contains(t, logged, "429", "error log must include the HTTP status code")
	assert.Contains(t, logged, "ERROR", "log must include ERROR level indicator")
}

// --- SC-005: discord_webhook_url empty → alerter not instantiated, no HTTP call ---

func TestNewDiscordRateLimitAlerter_EmptyWebhookURL_ReturnsNil(t *testing.T) {
	// SC-005
	a := NewDiscordRateLimitAlerter("", 0.2)
	assert.Nil(t, a, "alerter must be nil when webhook URL is empty")
}

func TestNewDiscordRateLimitAlerter_EmptyWebhookURL_NoHTTPCall(t *testing.T) {
	// SC-005: verify that no HTTP calls are made when alerter is nil
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(200)
	}))
	defer srv.Close()

	// Instantiate with empty URL (ignoring srv.URL – simulating absent config)
	a := NewDiscordRateLimitAlerter("", 0.2)
	assert.Nil(t, a)

	// Callers must check nil before calling CheckAndAlert. No crash, no HTTP call.
	if a != nil {
		state := AnthropicRateLimitState{InputParsed: true, InputTokensLimit: 10000, InputTokensRemaining: 100}
		a.CheckAndAlert(state)
		time.Sleep(50 * time.Millisecond)
	}

	assert.Equal(t, 0, callCount, "no HTTP calls should be made when alerter is nil")
}

// --- Additional: no alert when above threshold ---

func TestCheckAndAlert_InputAboveThreshold_NoAlert(t *testing.T) {
	srv, getMsgs := newMockDiscordServer(t, 200)
	defer srv.Close()

	a := NewDiscordRateLimitAlerter(srv.URL, 0.2)
	require.NotNil(t, a)
	a.cooldown = 0

	state := AnthropicRateLimitState{
		InputTokensLimit:      10000,
		InputTokensRemaining:  4000, // 40% – above 20% threshold
		InputTokensReset:      "2026-03-01T02:00:00Z",
		InputParsed:           true,
		OutputTokensLimit:     8000,
		OutputTokensRemaining: 5000, // 62.5% – above threshold
		OutputTokensReset:     "2026-03-01T02:00:00Z",
		OutputParsed:          true,
	}

	a.CheckAndAlert(state)
	time.Sleep(50 * time.Millisecond)

	assert.Len(t, getMsgs(), 0, "no alert when both token types are above threshold")
}

// --- Additional: ParseAnthropicRateLimitHeaders happy path ---

func TestParseAnthropicHeaders_AllPresent_ReturnsParsedState(t *testing.T) {
	_, restore := captureLog(t)
	defer restore()

	h := makeFullHeaders(10000, 1000, "2026-03-01T02:00:00Z", 8000, 1500, "2026-03-01T03:00:00Z")

	state := ParseAnthropicRateLimitHeaders(h)

	assert.True(t, state.InputParsed)
	assert.True(t, state.OutputParsed)
	assert.Equal(t, int64(10000), state.InputTokensLimit)
	assert.Equal(t, int64(1000), state.InputTokensRemaining)
	assert.Equal(t, "2026-03-01T02:00:00Z", state.InputTokensReset)
	assert.Equal(t, int64(8000), state.OutputTokensLimit)
	assert.Equal(t, int64(1500), state.OutputTokensRemaining)
	assert.Equal(t, "2026-03-01T03:00:00Z", state.OutputTokensReset)
}
