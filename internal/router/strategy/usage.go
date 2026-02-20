package strategy

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/praxisllmlab/tianjiLLM/internal/router"
)

// UsageBased picks the deployment with the lowest current TPM/RPM usage
// using sliding window counters.
type UsageBased struct {
	mu       sync.RWMutex
	counters map[string]*usageCounter
	window   time.Duration
}

type usageCounter struct {
	requests atomic.Int64
	tokens   atomic.Int64
	resetAt  time.Time
}

// NewUsageBased creates a usage-based routing strategy.
func NewUsageBased(window time.Duration) *UsageBased {
	if window == 0 {
		window = 1 * time.Minute
	}
	return &UsageBased{
		counters: make(map[string]*usageCounter),
		window:   window,
	}
}

func (u *UsageBased) Pick(deployments []*router.Deployment) *router.Deployment {
	if len(deployments) == 0 {
		return nil
	}

	var best *router.Deployment
	var bestUsage float64 = -1

	for _, d := range deployments {
		c := u.getCounter(d.ID)

		// Reset if window expired
		if time.Now().After(c.resetAt) {
			c.requests.Store(0)
			c.tokens.Store(0)
			c.resetAt = time.Now().Add(u.window)
		}

		rpm := c.requests.Load()
		tpm := c.tokens.Load()

		// Calculate usage ratio against limits
		ratio := usageRatio(d, rpm, tpm)

		if bestUsage < 0 || ratio < bestUsage {
			bestUsage = ratio
			best = d
		}
	}

	return best
}

// RecordUsage records a request and token count for a deployment.
func (u *UsageBased) RecordUsage(deploymentID string, tokens int64) {
	c := u.getCounter(deploymentID)
	c.requests.Add(1)
	c.tokens.Add(tokens)
}

func (u *UsageBased) getCounter(id string) *usageCounter {
	u.mu.RLock()
	c, ok := u.counters[id]
	u.mu.RUnlock()
	if ok {
		return c
	}

	u.mu.Lock()
	defer u.mu.Unlock()
	if c, ok = u.counters[id]; ok {
		return c
	}
	c = &usageCounter{resetAt: time.Now().Add(u.window)}
	u.counters[id] = c
	return c
}

func usageRatio(d *router.Deployment, rpm, tpm int64) float64 {
	rpmLimit := int64(60)     // default
	tpmLimit := int64(100000) // default

	if d.Config.TianjiParams.RPM != nil {
		rpmLimit = *d.Config.TianjiParams.RPM
	}
	if d.Config.TianjiParams.TPM != nil {
		tpmLimit = *d.Config.TianjiParams.TPM
	}

	rpmRatio := float64(rpm) / float64(rpmLimit)
	tpmRatio := float64(tpm) / float64(tpmLimit)

	// Return the higher ratio — the more constrained dimension
	if rpmRatio > tpmRatio {
		return rpmRatio
	}
	return tpmRatio
}
