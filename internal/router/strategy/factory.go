package strategy

import (
	"fmt"
	"time"

	"github.com/praxisllmlab/tianjiLLM/internal/callback"
	"github.com/praxisllmlab/tianjiLLM/internal/router"
)

// Options holds optional dependencies for strategy construction.
type Options struct {
	RateLimitStore       callback.RateLimitStore
	UtilizationThreshold float64
	AlertFn              AlertFunc
}

// Option configures strategy construction.
type Option func(*Options)

// WithRateLimitStore injects a RateLimitStore for utilization-based strategies.
func WithRateLimitStore(store callback.RateLimitStore) Option {
	return func(o *Options) { o.RateLimitStore = store }
}

// WithUtilizationThreshold sets the utilization threshold (0-100).
func WithUtilizationThreshold(threshold float64) Option {
	return func(o *Options) { o.UtilizationThreshold = threshold }
}

// WithAlertFunc sets the alert function for token switch notifications.
func WithAlertFunc(fn AlertFunc) Option {
	return func(o *Options) { o.AlertFn = fn }
}

// NewFromConfig creates a Strategy from a config string name.
func NewFromConfig(name string, opts ...Option) (router.Strategy, error) {
	var o Options
	for _, opt := range opts {
		opt(&o)
	}

	switch name {
	case "simple-shuffle", "":
		return NewShuffle(), nil
	case "least-busy":
		return NewLeastBusy(), nil
	case "lowest-latency":
		return NewLowestLatency(), nil
	case "lowest-cost":
		return NewLowestCost(), nil
	case "usage-based":
		return NewUsageBased(time.Minute), nil
	case "lowest-tpm-rpm":
		return NewLowestTPMRPM(NewShuffle()), nil
	case "priority":
		return NewPriorityQueue(NewShuffle()), nil
	case "lowest-utilization":
		return NewLowestUtilization(o.RateLimitStore, o.UtilizationThreshold, o.AlertFn), nil
	default:
		return nil, fmt.Errorf("unknown routing strategy: %s", name)
	}
}
