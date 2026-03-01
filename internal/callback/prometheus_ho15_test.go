package callback

import (
	"crypto/sha256"
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

// scrapeMetrics hits the /metrics handler and returns the body string.
func scrapeMetrics(t *testing.T) string {
	t.Helper()
	rec := httptest.NewRecorder()
	Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	body, err := io.ReadAll(rec.Body)
	require.NoError(t, err)
	return string(body)
}

// apiKeyHash returns the expected sha256[:8] hash for a given key.
func apiKeyHash(key string) string {
	h := sha256.Sum256([]byte(key))
	return fmt.Sprintf("%x", h[:])[:8]
}

// ---------------------------------------------------------------------------
// AC1: GET /metrics returns 200 + text/plain with # HELP lines
// ---------------------------------------------------------------------------
func TestAC1_MetricsEndpoint_Returns200_WithHelp(t *testing.T) {
	rec := httptest.NewRecorder()
	Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))

	assert.Equal(t, http.StatusOK, rec.Code)
	ct := rec.Header().Get("Content-Type")
	assert.True(t, strings.Contains(ct, "text/plain"),
		"Content-Type should contain text/plain, got: %s", ct)
	assert.Contains(t, rec.Body.String(), "# HELP")
}

// ---------------------------------------------------------------------------
// AC2: GET /metrics does not require auth header
// ---------------------------------------------------------------------------
func TestAC2_MetricsEndpoint_NoAuth(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	req.Header.Del("Authorization")
	rec := httptest.NewRecorder()
	Handler().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code,
		"GET /metrics should return 200 without auth header")
}

// ---------------------------------------------------------------------------
// AC3: After successful proxy request, metrics contain model + api_key_hash label
// ---------------------------------------------------------------------------
func TestAC3_LogSuccess_HasModelAndApiKeyHashLabel(t *testing.T) {
	cb := NewPrometheusCallback()
	apiKey := "sk-test-key-12345"

	cb.LogSuccess(LogData{
		Model:            "gpt-4o",
		Provider:         "openai",
		APIKey:           apiKey,
		Latency:          200 * time.Millisecond,
		LLMAPILatency:    150 * time.Millisecond,
		PromptTokens:     100,
		CompletionTokens: 50,
		Cost:             0.001,
	})

	body := scrapeMetrics(t)

	// requestCounter should have api_key_hash label
	assert.Contains(t, body, `api_key_hash="`,
		"metrics should contain api_key_hash label after LogSuccess")
	assert.Contains(t, body, `model="gpt-4o"`,
		"metrics should contain model label")

	expectedHash := apiKeyHash(apiKey)
	assert.Contains(t, body, fmt.Sprintf(`api_key_hash="%s"`, expectedHash),
		"api_key_hash should be sha256[:8] of the API key")
}

// ---------------------------------------------------------------------------
// AC4: api_key_hash is sha256[:8], no plaintext key in metrics
// ---------------------------------------------------------------------------
func TestAC4_ApiKeyHash_NoPlaintextKey(t *testing.T) {
	cb := NewPrometheusCallback()
	apiKey := "sk-secret-key-abc123"

	cb.LogSuccess(LogData{
		Model:            "gpt-4o-mini",
		Provider:         "openai",
		APIKey:           apiKey,
		Latency:          100 * time.Millisecond,
		LLMAPILatency:    80 * time.Millisecond,
		PromptTokens:     50,
		CompletionTokens: 25,
		Cost:             0.0005,
	})

	body := scrapeMetrics(t)

	// Must NOT contain plaintext key
	assert.NotContains(t, body, apiKey,
		"metrics must not contain plaintext API key")

	// Must contain the hash
	expectedHash := apiKeyHash(apiKey)
	assert.Contains(t, body, expectedHash,
		"metrics should contain sha256[:8] hash of API key")
}

// ---------------------------------------------------------------------------
// AC5: After failed request, requestCounter has status="error" label
// ---------------------------------------------------------------------------
func TestAC5_LogFailure_StatusErrorLabel(t *testing.T) {
	cb := NewPrometheusCallback()

	cb.LogFailure(LogData{
		Model:    "gpt-4o",
		Provider: "openai",
		Latency:  500 * time.Millisecond,
		Error:    fmt.Errorf("upstream timeout"),
	})

	body := scrapeMetrics(t)

	// When Error != nil, status should be "error", not "500"
	// The current bug: Error != nil -> status = "500" instead of "error"
	assert.Regexp(t, `tianji_requests_total\{.*model="gpt-4o".*status="error".*\}`, body,
		"failed request with Error != nil should have status=error label")
}

// ---------------------------------------------------------------------------
// AC6: After successful request, tokenCounter has input/output token counts
// ---------------------------------------------------------------------------
func TestAC6_LogSuccess_TokenCounters(t *testing.T) {
	cb := NewPrometheusCallback()

	cb.LogSuccess(LogData{
		Model:            "gpt-4o",
		Provider:         "openai",
		Latency:          200 * time.Millisecond,
		LLMAPILatency:    150 * time.Millisecond,
		PromptTokens:     123,
		CompletionTokens: 456,
		Cost:             0.01,
	})

	body := scrapeMetrics(t)

	// Check prompt (input) tokens are recorded
	assert.Regexp(t, `tianji_tokens_total\{.*type="prompt".*\} [1-9]`, body,
		"should have prompt token count > 0")
	// Check completion (output) tokens are recorded
	assert.Regexp(t, `tianji_tokens_total\{.*type="completion".*\} [1-9]`, body,
		"should have completion token count > 0")
}

// ---------------------------------------------------------------------------
// AC7: Latency histogram has observe records
// ---------------------------------------------------------------------------
func TestAC7_LogSuccess_LatencyHistogramObserved(t *testing.T) {
	cb := NewPrometheusCallback()

	cb.LogSuccess(LogData{
		Model:            "gpt-4o",
		Provider:         "openai",
		Latency:          300 * time.Millisecond,
		LLMAPILatency:    250 * time.Millisecond,
		PromptTokens:     10,
		CompletionTokens: 5,
		Cost:             0.0001,
	})

	body := scrapeMetrics(t)

	// totalLatency histogram should have at least 1 observation
	assert.Contains(t, body, `tianji_request_total_latency_seconds_count`,
		"latency histogram should have observation count")
	assert.Regexp(t, `tianji_request_total_latency_seconds_count\{.*\} [1-9]`, body,
		"latency histogram observation count should be > 0")
}
