package strategy

import (
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/praxisllmlab/tianjiLLM/internal/router"
	"github.com/stretchr/testify/assert"
)

func makeDeployments(n int) []*router.Deployment {
	result := make([]*router.Deployment, n)
	for i := range n {
		result[i] = &router.Deployment{
			ID:           string(rune('A' + i)),
			ProviderName: "openai",
			ModelName:    "gpt-4o",
			Config:       &config.ModelConfig{},
		}
	}
	return result
}

func TestShuffle_Pick_Empty(t *testing.T) {
	s := NewShuffle()
	assert.Nil(t, s.Pick(nil))
	assert.Nil(t, s.Pick([]*router.Deployment{}))
}

func TestShuffle_Pick_Single(t *testing.T) {
	s := NewShuffle()
	deployments := makeDeployments(1)
	picked := s.Pick(deployments)
	assert.Equal(t, deployments[0], picked)
}

func TestShuffle_Pick_Distribution(t *testing.T) {
	s := NewShuffle()
	deployments := makeDeployments(3)

	counts := make(map[string]int)
	for range 300 {
		d := s.Pick(deployments)
		counts[d.ID]++
	}

	// Each should be picked roughly 100 times (with some variance)
	for _, d := range deployments {
		assert.Greater(t, counts[d.ID], 50,
			"deployment %s should be picked at least 50 times out of 300", d.ID)
	}
}
