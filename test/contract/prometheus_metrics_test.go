package contract

// Tests for HO-15: Prometheus Metrics Exporter
// Written BEFORE implementation — these should FAIL until the feature is built.
// Based on specs/prometheus-metrics-exporter/spec.md

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/praxisllmlab/tianjiLLM/internal/callback"
	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/praxisllmlab/tianjiLLM/internal/proxy"
	"github.com/praxisllmlab/tianjiLLM/internal/proxy/handler"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func newMetricsTestServer(t *testing.T, metricsEnabled, requireAuth, perKeyMetrics bool) *proxy.Server {
	t.Helper()
	apiKey := "sk-test-key"
	cfg := &config.ProxyConfig{
		ModelList: []config.ModelConfig{
			{
				ModelName: "gpt-4o",
				TianjiParams: config.TianjiParams{
					Model:  "openai/gpt-4o",
					APIKey: &apiKey,
				},
			},
		},
		GeneralSettings: config.GeneralSettings{
			MasterKey: "sk-master",
		},
		// HO-15: MetricsConfig must exist on ProxyConfig — compile error until implemented
		Metrics: config.MetricsConfig{
			Enabled:       metricsEnabled,
			RequireAuth:   requireAuth,
			PerKeyMetrics: perKeyMetrics,
		},
	}

	handlers := &handler.Handlers{Config: cfg}

	return proxy.NewServer(proxy.ServerConfig{
		Handlers:  handlers,
		MasterKey: cfg.GeneralSettings.MasterKey,
	})
}

func makeLogData(model, provider, apiKey string) callback.LogData {
	return callback.LogData{
		Model:            model,
		Provider:         provider,
		Latency:          200 * time.Millisecond,
		LLMAPILatency:    150 * time.Millisecond,
		PromptTokens:     100,
		CompletionTokens: 50,
		Cost:             0.001,
		// HO-15: APIKey field must exist on LogData — compile error until implemented
		APIKey:           apiKey,
	}
}

func makeErrorLogData(model, provider, apiKey string, err error) callback.LogData {
	d := makeLogData(model, provider, apiKey)
	d.Error = err
	return d
}

// ---------------------------------------------------------------------------
// AC-1: /metrics endpoint accessible
// ---------------------------------------------------------------------------

func TestAC1_MetricsEndpointAccessible(t *testing.T) {
	srv := newMetricsTestServer(t, true, false, false)

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	resp := w.Result()
	body, _ := io.ReadAll(resp.Body)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode, "GET /metrics should return 200")

	ct := resp.Header.Get("Content-Type")
	assert.True(t,
		strings.Contains(ct, "text/plain") || strings.Contains(ct, "application/openmetrics-text"),
		"Content-Type should be Prometheus format, got: %s", ct,
	)

	assert.Contains(t, string(body), "tianji_requests_total",
		"Response body should contain tianji_requests_total metric")
}

// ---------------------------------------------------------------------------
// AC-2: Default no auth; require_auth=true needs auth
// ---------------------------------------------------------------------------

func TestAC2_MetricsNoAuthByDefault(t *testing.T) {
	srv := newMetricsTestServer(t, true, false, false)

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code,
		"GET /metrics without auth should return 200 when require_auth=false")
}

func TestAC2_MetricsRequireAuth_NoToken(t *testing.T) {
	srv := newMetricsTestServer(t, true, true, false)

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	assert.True(t,
		w.Code == http.StatusUnauthorized || w.Code == http.StatusForbidden,
		"GET /metrics without token when require_auth=true should return 401/403, got %d", w.Code,
	)
}

func TestAC2_MetricsRequireAuth_WithValidToken(t *testing.T) {
	srv := newMetricsTestServer(t, true, true, false)

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	req.Header.Set("Authorization", "Bearer sk-master")
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code,
		"GET /metrics with valid token should return 200")
}

// ---------------------------------------------------------------------------
// AC-3: Requests counter by model
// ---------------------------------------------------------------------------

func TestAC3_RequestsCounterByModel(t *testing.T) {
	srv := newMetricsTestServer(t, true, false, false)

	// Log a request via callback
	cb := callback.NewPrometheusCallback()
	require.NotNil(t, cb)
	cb.LogSuccess(makeLogData("gpt-4o", "openai", "sk-key-1"))

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	body := w.Body.String()
	assert.Contains(t, body, `tianji_requests_total`)
	assert.Contains(t, body, `model="gpt-4o"`)
}

// ---------------------------------------------------------------------------
// AC-4: Requests counter by api_key (per_key_metrics=true)
// ---------------------------------------------------------------------------

func TestAC4_RequestsCounterByApiKey(t *testing.T) {
	srv := newMetricsTestServer(t, true, false, true)

	cb := callback.NewPrometheusCallback()
	require.NotNil(t, cb)

	cb.LogSuccess(makeLogData("gpt-4o", "openai", "sk-key-alpha"))
	cb.LogSuccess(makeLogData("gpt-4o", "openai", "sk-key-beta"))

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	body := w.Body.String()

	// Should have api_key label
	assert.Contains(t, body, `api_key=`,
		"Metrics should contain api_key label when per_key_metrics=true")

	// api_key should be hashed, not raw
	assert.NotContains(t, body, `api_key="sk-key-alpha"`)
	assert.NotContains(t, body, `api_key="sk-key-beta"`)

	// Should have 2 distinct api_key values
	lines := strings.Split(body, "\n")
	apiKeyValues := map[string]bool{}
	for _, line := range lines {
		if strings.HasPrefix(line, "tianji_requests_total{") && strings.Contains(line, "api_key=") {
			idx := strings.Index(line, `api_key="`)
			if idx >= 0 {
				rest := line[idx+len(`api_key="`):]
				end := strings.Index(rest, `"`)
				if end >= 0 {
					apiKeyValues[rest[:end]] = true
				}
			}
		}
	}
	assert.GreaterOrEqual(t, len(apiKeyValues), 2,
		"Should have at least 2 distinct api_key values, got: %v", apiKeyValues)
}

// ---------------------------------------------------------------------------
// AC-5: Per-key disabled → api_key="_all"
// ---------------------------------------------------------------------------

func TestAC5_PerKeyDisabled_ApiKeyIsAll(t *testing.T) {
	srv := newMetricsTestServer(t, true, false, false)

	cb := callback.NewPrometheusCallback()
	require.NotNil(t, cb)
	cb.LogSuccess(makeLogData("gpt-4o", "openai", "sk-key-1"))

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	body := w.Body.String()
	assert.Contains(t, body, `api_key="_all"`,
		"When per_key_metrics=false, api_key label should be '_all'")
}

// ---------------------------------------------------------------------------
// AC-6: Latency histogram
// ---------------------------------------------------------------------------

func TestAC6_LatencyHistogram(t *testing.T) {
	srv := newMetricsTestServer(t, true, false, false)

	cb := callback.NewPrometheusCallback()
	require.NotNil(t, cb)
	cb.LogSuccess(makeLogData("gpt-4o", "openai", "sk-key-1"))

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	body := w.Body.String()
	assert.Contains(t, body, "tianji_request_total_latency_seconds_bucket",
		"Should contain latency histogram buckets")
}

// ---------------------------------------------------------------------------
// AC-7: Token usage counter
// ---------------------------------------------------------------------------

func TestAC7_TokenUsageCounter(t *testing.T) {
	srv := newMetricsTestServer(t, true, false, false)

	cb := callback.NewPrometheusCallback()
	require.NotNil(t, cb)
	cb.LogSuccess(makeLogData("gpt-4o", "openai", "sk-key-1"))

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	body := w.Body.String()
	assert.Contains(t, body, `tianji_tokens_total{`)
	assert.Contains(t, body, `type="prompt"`)
	assert.Contains(t, body, `type="completion"`)
}

// ---------------------------------------------------------------------------
// AC-8: Error counter (tianji_errors_total)
// ---------------------------------------------------------------------------

func TestAC8_ErrorCounter(t *testing.T) {
	srv := newMetricsTestServer(t, true, false, false)

	cb := callback.NewPrometheusCallback()
	require.NotNil(t, cb)
	cb.LogFailure(makeErrorLogData("gpt-4o", "openai", "sk-key-1", assert.AnError))

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	body := w.Body.String()
	assert.Contains(t, body, "tianji_errors_total",
		"Should contain tianji_errors_total metric")
	assert.Contains(t, body, `error_type=`,
		"tianji_errors_total should have error_type label")
}

// ---------------------------------------------------------------------------
// AC-9: Config flag controls endpoint (METRICS_ENABLED=false → 404)
// ---------------------------------------------------------------------------

func TestAC9_MetricsDisabled_Returns404(t *testing.T) {
	srv := newMetricsTestServer(t, false, false, false)

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w := httptest.NewRecorder()
	srv.Router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code,
		"GET /metrics should return 404 when metrics.enabled=false")
}

// ---------------------------------------------------------------------------
// Config struct tests
// ---------------------------------------------------------------------------

func TestMetricsConfig_StructExists(t *testing.T) {
	cfg := config.MetricsConfig{
		Enabled:       true,
		RequireAuth:   false,
		PerKeyMetrics: true,
	}
	assert.True(t, cfg.Enabled)
	assert.False(t, cfg.RequireAuth)
	assert.True(t, cfg.PerKeyMetrics)
}
