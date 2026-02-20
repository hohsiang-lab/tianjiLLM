package strategy

import (
	"testing"
	"time"

	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/praxisllmlab/tianjiLLM/internal/router"
	"github.com/stretchr/testify/assert"
)

func TestLowestLatency_Pick_Empty(t *testing.T) {
	l := NewLowestLatency()
	assert.Nil(t, l.Pick(nil))
}

func TestLowestLatency_Pick_NoData(t *testing.T) {
	l := NewLowestLatency()
	deployments := makeDeployments(3)

	// No latency data — should pick the first deployment to gather data
	picked := l.Pick(deployments)
	assert.Equal(t, deployments[0], picked)
}

func TestLowestLatency_Pick_ByLatency(t *testing.T) {
	l := NewLowestLatency()

	deployments := []*router.Deployment{
		{ID: "slow", ProviderName: "openai", ModelName: "gpt-4o", Config: &config.ModelConfig{}},
		{ID: "fast", ProviderName: "openai", ModelName: "gpt-4o", Config: &config.ModelConfig{}},
		{ID: "medium", ProviderName: "openai", ModelName: "gpt-4o", Config: &config.ModelConfig{}},
	}

	// Record latencies
	deployments[0].RecordSuccess(500 * time.Millisecond) // slow
	deployments[1].RecordSuccess(100 * time.Millisecond) // fast
	deployments[2].RecordSuccess(300 * time.Millisecond) // medium

	picked := l.Pick(deployments)
	assert.Equal(t, "fast", picked.ID)
}

func TestLowestLatency_Pick_EMAUpdates(t *testing.T) {
	d := &router.Deployment{
		ID: "test", ProviderName: "openai", ModelName: "gpt-4o",
		Config: &config.ModelConfig{},
	}

	// First call sets the EMA
	d.RecordSuccess(100 * time.Millisecond)
	assert.Equal(t, 100*time.Millisecond, d.LatencyEMA())

	// Subsequent calls update via EMA (alpha=0.3)
	d.RecordSuccess(200 * time.Millisecond)
	ema := d.LatencyEMA()
	// EMA = 100ms * 0.7 + 200ms * 0.3 = 70ms + 60ms = 130ms
	assert.InDelta(t, 130*time.Millisecond, ema, float64(5*time.Millisecond))
}
