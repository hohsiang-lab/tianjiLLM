package strategy

import (
	"sync"
	"sync/atomic"

	"github.com/praxisllmlab/tianjiLLM/internal/router"
)

// LeastBusy picks the deployment with the fewest in-flight requests.
// Satisfies Strategy interface (Pick) — no interface change needed.
// Router calls Acquire/Release via type assertion.
type LeastBusy struct {
	mu       sync.RWMutex
	inflight map[string]*atomic.Int64
}

// NewLeastBusy creates a least-busy strategy.
func NewLeastBusy() *LeastBusy {
	return &LeastBusy{
		inflight: make(map[string]*atomic.Int64),
	}
}

func (lb *LeastBusy) Pick(deployments []*router.Deployment) *router.Deployment {
	if len(deployments) == 0 {
		return nil
	}

	lb.mu.RLock()
	defer lb.mu.RUnlock()

	var best *router.Deployment
	var bestCount int64 = -1

	for _, d := range deployments {
		count := int64(0)
		if counter, ok := lb.inflight[d.ID]; ok {
			count = counter.Load()
		}
		if bestCount < 0 || count < bestCount {
			best = d
			bestCount = count
		}
	}
	return best
}

// Acquire increments the in-flight counter for a deployment.
// Called by router before sending to provider.
func (lb *LeastBusy) Acquire(deploymentID string) {
	lb.mu.Lock()
	counter, ok := lb.inflight[deploymentID]
	if !ok {
		counter = &atomic.Int64{}
		lb.inflight[deploymentID] = counter
	}
	lb.mu.Unlock()
	counter.Add(1)
}

// Release decrements the in-flight counter for a deployment.
// Called by router in defer after response.
func (lb *LeastBusy) Release(deploymentID string) {
	lb.mu.RLock()
	counter, ok := lb.inflight[deploymentID]
	lb.mu.RUnlock()
	if ok {
		counter.Add(-1)
	}
}
