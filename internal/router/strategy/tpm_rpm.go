package strategy

import (
	"sync"

	"github.com/praxisllmlab/tianjiLLM/internal/router"
)

// LowestTPMRPM selects the deployment with the lowest current TPM usage
// relative to its configured TPM limit. Falls back to round-robin if
// no TPM limits are configured.
type LowestTPMRPM struct {
	mu    sync.Mutex
	usage map[string]int64 // deploymentID → current TPM usage
	inner router.Strategy  // fallback when no limits configured
}

// NewLowestTPMRPM creates a new lowest-TPM/RPM strategy.
func NewLowestTPMRPM(fallback router.Strategy) *LowestTPMRPM {
	return &LowestTPMRPM{
		usage: make(map[string]int64),
		inner: fallback,
	}
}

func (s *LowestTPMRPM) Pick(deployments []*router.Deployment) *router.Deployment {
	if len(deployments) == 0 {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	var best *router.Deployment
	bestUtil := float64(2) // > 1.0 so any valid deployment wins

	for _, d := range deployments {
		limit := int64(0)
		if d.Config != nil && d.Config.TianjiParams.TPM != nil {
			limit = *d.Config.TianjiParams.TPM
		}
		if limit <= 0 {
			continue
		}

		current := s.usage[d.ID]
		utilization := float64(current) / float64(limit)
		if utilization < bestUtil {
			bestUtil = utilization
			best = d
		}
	}

	if best != nil {
		return best
	}

	// No TPM limits configured — fall back to inner strategy
	return s.inner.Pick(deployments)
}

// RecordUsage adds tokens to a deployment's current usage window.
func (s *LowestTPMRPM) RecordUsage(deploymentID string, tokens int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.usage[deploymentID] += tokens
}

// ResetUsage clears the usage window (called periodically, e.g. every minute).
func (s *LowestTPMRPM) ResetUsage() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.usage = make(map[string]int64)
}
