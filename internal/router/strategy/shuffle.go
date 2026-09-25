package strategy

import (
	"math/rand/v2"

	"github.com/praxisllmlab/tianjiLLM/internal/router"
)

// Shuffle implements weighted random selection (simple-shuffle).
// Each deployment has equal weight by default.
type Shuffle struct{}

func NewShuffle() *Shuffle {
	return &Shuffle{}
}

func (s *Shuffle) Pick(deployments []*router.Deployment) *router.Deployment {
	if len(deployments) == 0 {
		return nil
	}
	return deployments[rand.IntN(len(deployments))]
}
