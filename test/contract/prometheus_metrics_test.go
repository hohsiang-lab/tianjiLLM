package contract

// Tests for HO-15: Prometheus Metrics Exporter
// Updated to match Norman's architecture decision:
// - Separate metrics server on dedicated port (not on main router)
// - api_key label (hashed) on request counter & token counter
// - tianji_errors_total counter
// - LLM-tuned histogram buckets

import (
	"strings"
	"testing"
	"time"

	"github.com/praxisllmlab/tianjiLLM/internal/callback"
	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/prometheus/client_golang/prometheus"
	io_prometheus "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

func TestMetricsConfig_OnProxyConfig(t *testing.T) {
	cfg := &config.ProxyConfig{
		Metrics: config.MetricsConfig{
			Enabled: true,
		},
	}
	assert.True(t, cfg.Metrics.Enabled)
}

// ---------------------------------------------------------------------------
// AC-3: Requests counter with api_key label
// ---------------------------------------------------------------------------

func TestAC3_RequestsCounterWithAPIKeyLabel(t *testing.T) {
	cb := callback.NewPrometheusCallback()
	require.NotNil(t, cb)

	cb.LogSuccess(callback.LogData{
		Model:            "gpt-4o",
		Provider:         "openai",
		APIKey:           "sk-key-1",
		Latency:          200 * time.Millisecond,
		LLMAPILatency:    150 * time.Millisecond,
		PromptTokens:     100,
		CompletionTokens: 50,
		Cost:             0.001,
	})

	// Gather all metrics and check for api_key label
	families, err := prometheus.DefaultGatherer.Gather()
	require.NoError(t, err)

	found := false
	for _, f := range families {
		if f.GetName() == "tianji_requests_total" {
			for _, m := range f.GetMetric() {
				for _, lp := range m.GetLabel() {
					if lp.GetName() == "api_key" {
						found = true
						// Should be hashed, not raw
						assert.NotEqual(t, "sk-key-1", lp.GetValue())
						assert.Len(t, lp.GetValue(), 8)
					}
				}
			}
		}
	}
	assert.True(t, found, "tianji_requests_total should have api_key label")
}

// ---------------------------------------------------------------------------
// AC-7: Token usage counter with api_key label
// ---------------------------------------------------------------------------

func TestAC7_TokenCounterWithAPIKeyLabel(t *testing.T) {
	cb := callback.NewPrometheusCallback()
	require.NotNil(t, cb)

	cb.LogSuccess(callback.LogData{
		Model:            "gpt-4o",
		Provider:         "openai",
		APIKey:           "sk-key-token-test",
		Latency:          200 * time.Millisecond,
		LLMAPILatency:    150 * time.Millisecond,
		PromptTokens:     100,
		CompletionTokens: 50,
		Cost:             0.001,
	})

	families, err := prometheus.DefaultGatherer.Gather()
	require.NoError(t, err)

	hasPrompt := false
	hasCompletion := false
	for _, f := range families {
		if f.GetName() == "tianji_tokens_total" {
			for _, m := range f.GetMetric() {
				labels := labelMap(m.GetLabel())
				if labels["api_key"] != "" && labels["type"] == "prompt" {
					hasPrompt = true
				}
				if labels["api_key"] != "" && labels["type"] == "completion" {
					hasCompletion = true
				}
			}
		}
	}
	assert.True(t, hasPrompt, "Should have prompt token counter with api_key")
	assert.True(t, hasCompletion, "Should have completion token counter with api_key")
}

// ---------------------------------------------------------------------------
// AC-6: Latency histogram with LLM-tuned buckets
// ---------------------------------------------------------------------------

func TestAC6_LatencyHistogramLLMBuckets(t *testing.T) {
	cb := callback.NewPrometheusCallback()
	require.NotNil(t, cb)

	cb.LogSuccess(callback.LogData{
		Model:         "gpt-4o",
		Provider:      "openai",
		Latency:       200 * time.Millisecond,
		LLMAPILatency: 150 * time.Millisecond,
	})

	families, err := prometheus.DefaultGatherer.Gather()
	require.NoError(t, err)

	for _, f := range families {
		if f.GetName() == "tianji_request_total_latency_seconds" {
			for _, m := range f.GetMetric() {
				h := m.GetHistogram()
				require.NotNil(t, h)
				// Should have 11 buckets (LLM-tuned) + Inf
				bucketCount := len(h.GetBucket())
				assert.Equal(t, 11, bucketCount,
					"Expected 11 LLM-tuned buckets, got %d", bucketCount)
				// First bucket should be 0.05
				if bucketCount > 0 {
					assert.Equal(t, 0.05, h.GetBucket()[0].GetUpperBound())
				}
				return
			}
		}
	}
	t.Fatal("tianji_request_total_latency_seconds metric not found")
}

// ---------------------------------------------------------------------------
// AC-8: Error counter (tianji_errors_total)
// ---------------------------------------------------------------------------

func TestAC8_ErrorCounter(t *testing.T) {
	cb := callback.NewPrometheusCallback()
	require.NotNil(t, cb)

	cb.LogFailure(callback.LogData{
		Model:    "gpt-4o",
		Provider: "openai",
		APIKey:   "sk-key-err",
		Latency:  500 * time.Millisecond,
		Error:    assert.AnError,
	})

	families, err := prometheus.DefaultGatherer.Gather()
	require.NoError(t, err)

	found := false
	for _, f := range families {
		if f.GetName() == "tianji_errors_total" {
			for _, m := range f.GetMetric() {
				labels := labelMap(m.GetLabel())
				if labels["error_type"] != "" {
					found = true
					assert.NotEmpty(t, labels["error_type"])
				}
			}
		}
	}
	assert.True(t, found, "Should have tianji_errors_total with error_type label")
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func labelMap(labels []*io_prometheus.LabelPair) map[string]string {
	m := make(map[string]string, len(labels))
	for _, lp := range labels {
		m[lp.GetName()] = lp.GetValue()
	}
	return m
}

// Ensure strings is used (for any future assertions)
var _ = strings.Contains
