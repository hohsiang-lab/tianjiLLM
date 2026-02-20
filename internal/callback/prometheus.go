package callback

import (
	"net/http"
	"strconv"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// PrometheusCallback exports metrics to Prometheus.
type PrometheusCallback struct {
	totalLatency     *prometheus.HistogramVec
	llmAPILatency    *prometheus.HistogramVec
	timeToFirstToken *prometheus.HistogramVec
	tokenCounter     *prometheus.CounterVec
	costCounter      *prometheus.CounterVec
	requestCounter   *prometheus.CounterVec
}

var (
	prometheusOnce     sync.Once
	prometheusCallback *PrometheusCallback
)

// NewPrometheusCallback creates a Prometheus callback with standard metrics.
func NewPrometheusCallback() *PrometheusCallback {
	prometheusOnce.Do(func() {
		buckets := prometheus.DefBuckets

		prometheusCallback = &PrometheusCallback{
			totalLatency: prometheus.NewHistogramVec(prometheus.HistogramOpts{
				Name:    "tianji_request_total_latency_seconds",
				Help:    "Total request latency including proxy overhead",
				Buckets: buckets,
			}, []string{"model", "provider"}),

			llmAPILatency: prometheus.NewHistogramVec(prometheus.HistogramOpts{
				Name:    "tianji_llm_api_latency_seconds",
				Help:    "LLM API call latency",
				Buckets: buckets,
			}, []string{"model", "provider"}),

			timeToFirstToken: prometheus.NewHistogramVec(prometheus.HistogramOpts{
				Name:    "tianji_time_to_first_token_seconds",
				Help:    "Time to first token for streaming requests",
				Buckets: buckets,
			}, []string{"model", "provider"}),

			tokenCounter: prometheus.NewCounterVec(prometheus.CounterOpts{
				Name: "tianji_tokens_total",
				Help: "Total tokens processed",
			}, []string{"model", "provider", "type"}),

			costCounter: prometheus.NewCounterVec(prometheus.CounterOpts{
				Name: "tianji_spend_total",
				Help: "Total spend in USD",
			}, []string{"model", "provider"}),

			requestCounter: prometheus.NewCounterVec(prometheus.CounterOpts{
				Name: "tianji_requests_total",
				Help: "Total number of requests",
			}, []string{"model", "provider", "status"}),
		}

		prometheus.MustRegister(
			prometheusCallback.totalLatency,
			prometheusCallback.llmAPILatency,
			prometheusCallback.timeToFirstToken,
			prometheusCallback.tokenCounter,
			prometheusCallback.costCounter,
			prometheusCallback.requestCounter,
		)
	})

	return prometheusCallback
}

func (p *PrometheusCallback) LogSuccess(data LogData) {
	labels := prometheus.Labels{"model": data.Model, "provider": data.Provider}

	p.totalLatency.With(labels).Observe(data.Latency.Seconds())
	p.llmAPILatency.With(labels).Observe(data.LLMAPILatency.Seconds())

	p.tokenCounter.With(prometheus.Labels{
		"model": data.Model, "provider": data.Provider, "type": "prompt",
	}).Add(float64(data.PromptTokens))

	p.tokenCounter.With(prometheus.Labels{
		"model": data.Model, "provider": data.Provider, "type": "completion",
	}).Add(float64(data.CompletionTokens))

	p.costCounter.With(labels).Add(data.Cost)

	p.requestCounter.With(prometheus.Labels{
		"model": data.Model, "provider": data.Provider, "status": "success",
	}).Inc()
}

func (p *PrometheusCallback) LogFailure(data LogData) {
	labels := prometheus.Labels{"model": data.Model, "provider": data.Provider}

	p.totalLatency.With(labels).Observe(data.Latency.Seconds())

	status := "error"
	if data.Error != nil {
		status = strconv.Itoa(http.StatusInternalServerError)
	}

	p.requestCounter.With(prometheus.Labels{
		"model": data.Model, "provider": data.Provider, "status": status,
	}).Inc()
}

// Handler returns an HTTP handler for the /metrics endpoint.
func Handler() http.Handler {
	return promhttp.Handler()
}
