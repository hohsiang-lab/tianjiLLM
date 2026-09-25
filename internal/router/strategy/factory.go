package strategy

import (
	"fmt"
	"time"

	"github.com/praxisllmlab/tianjiLLM/internal/router"
)

// NewFromConfig creates a Strategy from a config string name.
func NewFromConfig(name string) (router.Strategy, error) {
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
	default:
		return nil, fmt.Errorf("unknown routing strategy: %s", name)
	}
}
