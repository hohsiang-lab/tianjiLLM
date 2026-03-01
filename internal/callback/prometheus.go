package callback

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"strconv"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// llmBuckets are histogram buckets tuned for LLM workloads.
var llmBuckets = []float64{0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60, 120}

// PrometheusCallback exports metrics to Prometheus.
type PrometheusCallback struct {
	totalLatency     *prometheus.HistogramVec
	llmAPILatency    *prometheus.HistogramVec
	timeToFirstToken *prometheus.HistogramVec
	tokenCounter     *prometheus.CounterVec
	costCounter      *prometheus.CounterVec
	requestCounter   *prometheus.CounterVec
	errorCounter     *prometheus.CounterVec
}

var (
	prometheusOnce     sync.Once
	prometheusCallback *PrometheusCallback
)

// NewPrometheusCallback creates a Prometheus callback with standard metrics.
func NewPrometheusCallback() *PrometheusCallback {
	prometheusOnce.Do(func() {
		prometheusCallback = &PrometheusCallback{
			totalLatency: prometheus.NewHistogramVec(prometheus.HistogramOpts{
				Name:    "tianji_request_total_latency_seconds",
				Help:    "Total request latency including proxy overhead",
				Buckets: llmBuckets,
			}, []string{"model", "provider"}),

			llmAPILatency: prometheus.NewHistogramVec(prometheus.HistogramOpts{
				Name:    "tianji_llm_api_latency_seconds",
				Help:    "LLM API call latency",
				Buckets: llmBuckets,
			}, []string{"model", "provider"}),

			timeToFirstToken: prometheus.NewHistogramVec(prometheus.HistogramOpts{
				Name:    "tianji_time_to_first_token_seconds",
				Help:    "Time to first token for streaming requests",
				Buckets: llmBuckets,
			}, []string{"model", "provider"}),

			tokenCounter: prometheus.NewCounterVec(prometheus.CounterOpts{
				Name: "tianji_tokens_total",
				Help: "Total tokens processed",
			}, []string{"model", "provider", "type", "api_key"}),

			costCounter: prometheus.NewCounterVec(prometheus.CounterOpts{
				Name: "tianji_spend_total",
				Help: "Total spend in USD",
			}, []string{"model", "provider"}),

			requestCounter: prometheus.NewCounterVec(prometheus.CounterOpts{
				Name: "tianji_requests_total",
				Help: "Total number of requests",
			}, []string{"model", "provider", "status", "api_key"}),

			errorCounter: prometheus.NewCounterVec(prometheus.CounterOpts{
				Name: "tianji_errors_total",
				Help: "Total number of errors by type",
			}, []string{"model", "provider", "error_type"}),
		}

		prometheus.MustRegister(
			prometheusCallback.totalLatency,
			prometheusCallback.llmAPILatency,
			prometheusCallback.timeToFirstToken,
			prometheusCallback.tokenCounter,
			prometheusCallback.costCounter,
			prometheusCallback.requestCounter,
			prometheusCallback.errorCounter,
		)
	})

	return prometheusCallback
}

// hashAPIKey returns the first 8 hex chars of the SHA-256 hash of the key.
func hashAPIKey(key string) string {
	if key == "" {
		return "_none"
	}
	h := sha256.Sum256([]byte(key))
	return fmt.Sprintf("%x", h[:4])
}

func (p *PrometheusCallback) LogSuccess(data LogData) {
	labels := prometheus.Labels{"model": data.Model, "provider": data.Provider}
	apiKeyHash := hashAPIKey(data.APIKey)

	p.totalLatency.With(labels).Observe(data.Latency.Seconds())
	p.llmAPILatency.With(labels).Observe(data.LLMAPILatency.Seconds())

	p.tokenCounter.With(prometheus.Labels{
		"model": data.Model, "provider": data.Provider, "type": "prompt", "api_key": apiKeyHash,
	}).Add(float64(data.PromptTokens))

	p.tokenCounter.With(prometheus.Labels{
		"model": data.Model, "provider": data.Provider, "type": "completion", "api_key": apiKeyHash,
	}).Add(float64(data.CompletionTokens))

	p.costCounter.With(labels).Add(data.Cost)

	p.requestCounter.With(prometheus.Labels{
		"model": data.Model, "provider": data.Provider, "status": "success", "api_key": apiKeyHash,
	}).Inc()
}

func (p *PrometheusCallback) LogFailure(data LogData) {
	labels := prometheus.Labels{"model": data.Model, "provider": data.Provider}
	apiKeyHash := hashAPIKey(data.APIKey)

	p.totalLatency.With(labels).Observe(data.Latency.Seconds())

	status := "error"
	errorType := "unknown"
	if data.Error != nil {
		status = strconv.Itoa(http.StatusInternalServerError)
		errorType = "internal_error"
	}

	p.requestCounter.With(prometheus.Labels{
		"model": data.Model, "provider": data.Provider, "status": status, "api_key": apiKeyHash,
	}).Inc()

	p.errorCounter.With(prometheus.Labels{
		"model": data.Model, "provider": data.Provider, "error_type": errorType,
	}).Inc()
}

// Handler returns an HTTP handler for the /metrics endpoint.
func Handler() http.Handler {
	return promhttp.Handler()
}
