package strategy

import (
	"math"

	"github.com/praxisllmlab/tianjiLLM/internal/pricing"
	"github.com/praxisllmlab/tianjiLLM/internal/router"
)

// LowestCost picks the deployment with the lowest input cost per token.
// Falls back to embedded pricing table when ModelInfo.InputCost is not set.
type LowestCost struct{}

func NewLowestCost() *LowestCost {
	return &LowestCost{}
}

func (l *LowestCost) Pick(deployments []*router.Deployment) *router.Deployment {
	if len(deployments) == 0 {
		return nil
	}

	var best *router.Deployment
	bestCost := math.MaxFloat64

	for _, d := range deployments {
		cost := inputCost(d)
		if cost < bestCost {
			bestCost = cost
			best = d
		}
	}

	return best
}

func inputCost(d *router.Deployment) float64 {
	// Config-level override takes priority
	if d.Config.ModelInfo != nil && d.Config.ModelInfo.InputCost != nil {
		return *d.Config.ModelInfo.InputCost
	}

	// Fall back to embedded pricing table
	info := pricing.Default().GetModelInfo(d.ModelName)
	if info != nil && info.InputCostPerToken > 0 {
		return info.InputCostPerToken
	}

	return math.MaxFloat64
}
