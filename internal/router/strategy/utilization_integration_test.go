package strategy_test

import (
	"sync"
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/callback"
	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/praxisllmlab/tianjiLLM/internal/router"
	"github.com/praxisllmlab/tianjiLLM/internal/router/strategy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeDep(id, apiKey string) *router.Deployment {
	key := apiKey
	return &router.Deployment{
		ID:           id,
		ProviderName: "anthropic",
		ModelName:    "claude-sonnet-4-5-20250929",
		Config: &config.ModelConfig{
			ModelName: id,
			TianjiParams: config.TianjiParams{
				Model:  "anthropic/claude-sonnet-4-5-20250929",
				APIKey: &key,
			},
		},
	}
}

func setState(store *callback.InMemoryRateLimitStore, apiKey string, util5h float64, status string) {
	key := callback.RateLimitCacheKey(apiKey)
	store.Set(key, callback.AnthropicOAuthRateLimitState{
		TokenKey:             key,
		Unified5hUtilization: util5h,
		Unified5hStatus:      status,
	})
}

// Scenario 1: active < threshold → no switch
func TestIntegration_ActiveBelowThreshold_NoSwitch(t *testing.T) {
	t.Parallel()
	store := callback.NewInMemoryRateLimitStore()
	setState(store, "key-A", 0.50, "allowed") // 50%
	setState(store, "key-B", 0.30, "allowed") // 30%

	lu := strategy.NewLowestUtilization(store, 80, nil)
	depA := makeDep("dep-A", "key-A")
	depB := makeDep("dep-B", "key-B")

	// First pick → selects lowest (key-B at 30%)
	d1 := lu.Pick([]*router.Deployment{depA, depB})
	require.NotNil(t, d1)
	firstKey := callback.RateLimitCacheKey(d1.APIKey())

	// Subsequent picks should stay on same key (both below 80%)
	for i := 0; i < 10; i++ {
		d := lu.Pick([]*router.Deployment{depA, depB})
		require.NotNil(t, d)
		assert.Equal(t, firstKey, callback.RateLimitCacheKey(d.APIKey()), "should not switch when below threshold")
	}
}

// Scenario 2: active ≥ threshold → switch to lowest
func TestIntegration_ActiveAboveThreshold_SwitchToLowest(t *testing.T) {
	t.Parallel()
	store := callback.NewInMemoryRateLimitStore()
	setState(store, "key-A", 0.85, "allowed") // 85% - above 80
	setState(store, "key-B", 0.20, "allowed") // 20%
	setState(store, "key-C", 0.40, "allowed") // 40%

	lu := strategy.NewLowestUtilization(store, 80, nil)
	depA := makeDep("dep-A", "key-A")
	depB := makeDep("dep-B", "key-B")
	depC := makeDep("dep-C", "key-C")

	// Force active to key-A
	lu.Pick([]*router.Deployment{depA}) // cold start → picks A

	// Now A is active, but above threshold. With all 3 available, should switch to B (lowest)
	d := lu.Pick([]*router.Deployment{depA, depB, depC})
	require.NotNil(t, d)
	assert.Equal(t, callback.RateLimitCacheKey("key-B"), callback.RateLimitCacheKey(d.APIKey()))
}

// Scenario 3: tie → LRU (least recently used)
func TestIntegration_TieBreak_LRU(t *testing.T) {
	t.Parallel()
	store := callback.NewInMemoryRateLimitStore()
	setState(store, "key-A", 0.50, "allowed")
	setState(store, "key-B", 0.50, "allowed")

	lu := strategy.NewLowestUtilization(store, 80, nil)
	depA := makeDep("dep-A", "key-A")
	depB := makeDep("dep-B", "key-B")

	// Cold start with tie: first call picks one (deterministic by iteration order is not guaranteed,
	// but the LRU tie-break ensures least-recently-used wins).
	d := lu.Pick([]*router.Deployment{depA, depB})
	require.NotNil(t, d)
	// Both are valid on first pick (zero lastUsedAt), so we just verify it returns something.
	assert.Contains(t, []string{
		callback.RateLimitCacheKey("key-A"),
		callback.RateLimitCacheKey("key-B"),
	}, callback.RateLimitCacheKey(d.APIKey()))
}

// Scenario 4: rate_limited → skipped
func TestIntegration_RateLimited_Skipped(t *testing.T) {
	t.Parallel()
	store := callback.NewInMemoryRateLimitStore()
	setState(store, "key-A", 0.10, "rate_limited")
	setState(store, "key-B", 0.60, "allowed")
	setState(store, "key-C", 0.70, "allowed")

	lu := strategy.NewLowestUtilization(store, 80, nil)
	depA := makeDep("dep-A", "key-A")
	depB := makeDep("dep-B", "key-B")
	depC := makeDep("dep-C", "key-C")

	d := lu.Pick([]*router.Deployment{depA, depB, depC})
	require.NotNil(t, d)
	// key-A is rate_limited → should pick key-B (60% < 70%)
	assert.Equal(t, callback.RateLimitCacheKey("key-B"), callback.RateLimitCacheKey(d.APIKey()))
}

// Scenario 5: all rate_limited → fallback shuffle
func TestIntegration_AllRateLimited_FallbackShuffle(t *testing.T) {
	t.Parallel()
	store := callback.NewInMemoryRateLimitStore()
	setState(store, "key-A", 0.99, "rate_limited")
	setState(store, "key-B", 0.99, "rate_limited")

	lu := strategy.NewLowestUtilization(store, 80, nil)
	depA := makeDep("dep-A", "key-A")
	depB := makeDep("dep-B", "key-B")

	// Should still return a deployment (via shuffle fallback)
	d := lu.Pick([]*router.Deployment{depA, depB})
	require.NotNil(t, d, "all rate_limited should still return a deployment via shuffle")
}

// Scenario 6: cold start → shuffle → sets active token after first round
func TestIntegration_ColdStart_ShuffleThenSetActive(t *testing.T) {
	t.Parallel()
	store := callback.NewInMemoryRateLimitStore()
	// No data in store.

	lu := strategy.NewLowestUtilization(store, 80, nil)
	depA := makeDep("dep-A", "key-A")
	depB := makeDep("dep-B", "key-B")

	// First pick: cold start with no utilization data → shuffle
	d1 := lu.Pick([]*router.Deployment{depA, depB})
	require.NotNil(t, d1, "cold start should return a deployment")

	// After first pick, activeAnthropicToken is set. Subsequent picks should return same token.
	d2 := lu.Pick([]*router.Deployment{depA, depB})
	require.NotNil(t, d2)
	assert.Equal(t, callback.RateLimitCacheKey(d1.APIKey()), callback.RateLimitCacheKey(d2.APIKey()),
		"should stick with same token after cold start")
}

// Scenario 7: Discord notification on switch (mock alert)
func TestIntegration_SwitchTriggersDiscordAlert(t *testing.T) {
	t.Parallel()
	store := callback.NewInMemoryRateLimitStore()
	setState(store, "key-A", 0.90, "allowed") // above threshold
	setState(store, "key-B", 0.20, "allowed")

	var mu sync.Mutex
	var alerts []string
	alertFn := func(msg string) {
		mu.Lock()
		defer mu.Unlock()
		alerts = append(alerts, msg)
	}

	lu := strategy.NewLowestUtilization(store, 80, alertFn)
	depA := makeDep("dep-A", "key-A")
	depB := makeDep("dep-B", "key-B")

	// Force active to key-A
	lu.Pick([]*router.Deployment{depA})

	// Pick again → should switch and alert
	lu.Pick([]*router.Deployment{depA, depB})

	mu.Lock()
	defer mu.Unlock()
	require.Len(t, alerts, 1, "should have triggered exactly one alert")
	assert.Contains(t, alerts[0], "OAuth token switched")
}

// Scenario 8: regression — existing strategies unaffected
func TestIntegration_ExistingStrategies_Unaffected(t *testing.T) {
	t.Parallel()
	strategies := []string{"simple-shuffle", "least-busy", "lowest-latency", "lowest-cost", "usage-based", "lowest-tpm-rpm", "priority"}

	for _, name := range strategies {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			s, err := strategy.NewFromConfig(name)
			require.NoError(t, err, "strategy %q should still work", name)

			depA := makeDep("dep-A", "key-A")
			depB := makeDep("dep-B", "key-B")

			d := s.Pick([]*router.Deployment{depA, depB})
			require.NotNil(t, d, "strategy %q should return a deployment", name)
		})
	}
}
