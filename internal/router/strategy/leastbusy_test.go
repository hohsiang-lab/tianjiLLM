package strategy

import (
	"sync"
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/praxisllmlab/tianjiLLM/internal/router"
	"github.com/stretchr/testify/assert"
)

func namedDeployments(ids ...string) []*router.Deployment {
	var deps []*router.Deployment
	for _, id := range ids {
		deps = append(deps, &router.Deployment{
			ID:           id,
			ProviderName: "openai",
			ModelName:    "gpt-4o",
			Config:       &config.ModelConfig{},
		})
	}
	return deps
}

func TestLeastBusy_PicksLowestCount(t *testing.T) {
	lb := NewLeastBusy()
	deps := namedDeployments("a", "b", "c")

	// a has 3, b has 1, c has 2
	lb.Acquire("a")
	lb.Acquire("a")
	lb.Acquire("a")
	lb.Acquire("b")
	lb.Acquire("c")
	lb.Acquire("c")

	picked := lb.Pick(deps)
	assert.Equal(t, "b", picked.ID)
}

func TestLeastBusy_EmptyDeployments(t *testing.T) {
	lb := NewLeastBusy()
	assert.Nil(t, lb.Pick(nil))
}

func TestLeastBusy_AllZero(t *testing.T) {
	lb := NewLeastBusy()
	deps := namedDeployments("a", "b")

	picked := lb.Pick(deps)
	assert.NotNil(t, picked)
}

func TestLeastBusy_AcquireRelease(t *testing.T) {
	lb := NewLeastBusy()
	deps := namedDeployments("a", "b")

	lb.Acquire("a")
	lb.Acquire("a")
	lb.Acquire("b")

	// a=2, b=1 → picks b
	assert.Equal(t, "b", lb.Pick(deps).ID)

	lb.Release("a")
	lb.Release("a")
	// a=0, b=1 → picks a
	assert.Equal(t, "a", lb.Pick(deps).ID)
}

func TestLeastBusy_ConcurrentAcquireRelease(t *testing.T) {
	lb := NewLeastBusy()

	var wg sync.WaitGroup
	for range 100 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			lb.Acquire("deploy-1")
			lb.Release("deploy-1")
		}()
	}
	wg.Wait()

	lb.mu.RLock()
	counter := lb.inflight["deploy-1"]
	lb.mu.RUnlock()
	assert.Equal(t, int64(0), counter.Load())
}

func TestBudgetLimiter_ExcludesExhaustedProviders(t *testing.T) {
	mock := &mockSpendQuerier{
		spends: map[string]float64{
			"openai":    100.0,
			"anthropic": 50.0,
		},
	}

	inner := &firstPicker{}
	bl := NewBudgetLimiter(
		map[string]float64{
			"openai":    80.0, // budget exceeded
			"anthropic": 100.0,
		},
		inner,
		mock,
	)

	deps := []*router.Deployment{
		{ID: "1", ProviderName: "openai", Config: &config.ModelConfig{}},
		{ID: "2", ProviderName: "anthropic", Config: &config.ModelConfig{}},
	}

	picked := bl.Pick(deps)
	assert.NotNil(t, picked)
	assert.Equal(t, "2", picked.ID)
}

func TestBudgetLimiter_AllExhausted(t *testing.T) {
	mock := &mockSpendQuerier{
		spends: map[string]float64{
			"openai": 100.0,
		},
	}

	bl := NewBudgetLimiter(
		map[string]float64{"openai": 50.0},
		&firstPicker{},
		mock,
	)

	deps := []*router.Deployment{
		{ID: "1", ProviderName: "openai", Config: &config.ModelConfig{}},
	}

	assert.Nil(t, bl.Pick(deps))
}

func TestBudgetLimiter_NoBudgetConstraint(t *testing.T) {
	mock := &mockSpendQuerier{spends: map[string]float64{}}

	bl := NewBudgetLimiter(
		map[string]float64{},
		&firstPicker{},
		mock,
	)

	deps := namedDeployments("a", "b")
	picked := bl.Pick(deps)
	assert.NotNil(t, picked)
	assert.Equal(t, "a", picked.ID)
}

// helpers

type mockSpendQuerier struct {
	spends map[string]float64
}

func (m *mockSpendQuerier) GetProviderSpend(provider string) float64 {
	return m.spends[provider]
}

type firstPicker struct{}

func (f *firstPicker) Pick(deps []*router.Deployment) *router.Deployment {
	if len(deps) == 0 {
		return nil
	}
	return deps[0]
}
