package router

import (
	"sync"
	"time"

	"github.com/praxisllmlab/tianjiLLM/internal/config"
)

// Deployment represents a single provider deployment with health tracking.
type Deployment struct {
	ID           string
	ProviderName string
	ModelName    string
	Region       string // geographic region for region-based routing
	Config       *config.ModelConfig

	mu            sync.Mutex
	failures      int
	successes     int
	allowedFails  int
	cooldownTime  time.Duration
	cooldownUntil time.Time

	// Latency tracking for lowest-latency strategy
	latencyEMA   time.Duration // exponential moving average
	latencyAlpha float64       // EMA smoothing factor (0.3 by default)
}

// IsHealthy returns true if the deployment is not in cooldown.
func (d *Deployment) IsHealthy() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return time.Now().After(d.cooldownUntil)
}

// RecordSuccess records a successful call, resetting failure count.
func (d *Deployment) RecordSuccess(latency time.Duration) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.failures = 0
	d.successes++

	// Update latency EMA
	alpha := d.latencyAlpha
	if alpha == 0 {
		alpha = 0.3
	}
	if d.latencyEMA == 0 {
		d.latencyEMA = latency
	} else {
		d.latencyEMA = time.Duration(float64(d.latencyEMA)*(1-alpha) + float64(latency)*alpha)
	}
}

// RecordFailure records a failed call. After exceeding allowed failures,
// the deployment enters cooldown.
func (d *Deployment) RecordFailure() {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.failures++
	if d.failures >= d.allowedFails {
		d.cooldownUntil = time.Now().Add(d.cooldownTime)
		d.failures = 0
	}
}

// LatencyEMA returns the current exponential moving average latency.
func (d *Deployment) LatencyEMA() time.Duration {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.latencyEMA
}

// APIKey returns the API key from the deployment config.
func (d *Deployment) APIKey() string {
	if d.Config.TianjiParams.APIKey != nil {
		return *d.Config.TianjiParams.APIKey
	}
	return ""
}
