package strategy

import (
	"math"
	"time"

	"github.com/praxisllmlab/tianjiLLM/internal/router"
)

// LowestLatency picks the deployment with the lowest EMA latency.
type LowestLatency struct{}

func NewLowestLatency() *LowestLatency {
	return &LowestLatency{}
}

func (l *LowestLatency) Pick(deployments []*router.Deployment) *router.Deployment {
	if len(deployments) == 0 {
		return nil
	}

	var best *router.Deployment
	bestLatency := time.Duration(math.MaxInt64)

	for _, d := range deployments {
		lat := d.LatencyEMA()
		if lat == 0 {
			// No latency data yet — prefer this deployment to gather data
			return d
		}
		if lat < bestLatency {
			bestLatency = lat
			best = d
		}
	}

	return best
}
