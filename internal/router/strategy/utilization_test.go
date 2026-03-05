package strategy

import (
	"sync"
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/callback"
	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/praxisllmlab/tianjiLLM/internal/router"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeDeployment(id, apiKey string) *router.Deployment {
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

func setStoreState(store *callback.InMemoryRateLimitStore, apiKey string, util5h float64, status string) {
	key := callback.RateLimitCacheKey(apiKey)
	store.Set(key, callback.AnthropicOAuthRateLimitState{
		TokenKey:             key,
		Unified5hUtilization: util5h,
		Unified5hStatus:      status,
	})
}

func TestLowestUtilization_ActiveBelowThreshold_NoSwitch(t *testing.T) {
	t.Parallel()
	store := callback.NewInMemoryRateLimitStore()
	setStoreState(store, "key-A", 0.30, "allowed") // 30%
	setStoreState(store, "key-B", 0.10, "allowed") // 10%

	lu := NewLowestUtilization(store, 80, nil)

	depA := makeDeployment("dep-A", "key-A")
	depB := makeDeployment("dep-B", "key-B")

	// First pick sets active to lowest (B=10%).
	d1 := lu.Pick([]*router.Deployment{depA, depB})
	require.NotNil(t, d1)
	activeKey := callback.RateLimitCacheKey(d1.APIKey())

	// Second pick should stay on same key (below threshold).
	d2 := lu.Pick([]*router.Deployment{depA, depB})
	require.NotNil(t, d2)
	assert.Equal(t, activeKey, callback.RateLimitCacheKey(d2.APIKey()))
}

func TestLowestUtilization_ActiveAboveThreshold_Switches(t *testing.T) {
	t.Parallel()
	store := callback.NewInMemoryRateLimitStore()
	setStoreState(store, "key-A", 0.85, "allowed") // 85% — above 80 threshold
	setStoreState(store, "key-B", 0.20, "allowed") // 20%

	var alertMsg string
	alertFn := func(msg string) { alertMsg = msg }
	lu := NewLowestUtilization(store, 80, alertFn)

	depA := makeDeployment("dep-A", "key-A")
	depB := makeDeployment("dep-B", "key-B")

	// Force activeKey to key-A.
	lu.mu.Lock()
	lu.activeKey = callback.RateLimitCacheKey("key-A")
	lu.mu.Unlock()

	d := lu.Pick([]*router.Deployment{depA, depB})
	require.NotNil(t, d)
	assert.Equal(t, callback.RateLimitCacheKey("key-B"), callback.RateLimitCacheKey(d.APIKey()))
	assert.Contains(t, alertMsg, "OAuth token switched")
}

func TestLowestUtilization_TieBreak_LRU(t *testing.T) {
	t.Parallel()
	store := callback.NewInMemoryRateLimitStore()
	setStoreState(store, "key-A", 0.50, "allowed")
	setStoreState(store, "key-B", 0.50, "allowed")

	lu := NewLowestUtilization(store, 80, nil)
	// key-A was used more recently than key-B.
	lu.mu.Lock()
	lu.lastUsedAt[callback.RateLimitCacheKey("key-A")] = lu.lastUsedAt[callback.RateLimitCacheKey("key-B")].Add(1)
	lu.mu.Unlock()

	depA := makeDeployment("dep-A", "key-A")
	depB := makeDeployment("dep-B", "key-B")

	d := lu.Pick([]*router.Deployment{depA, depB})
	require.NotNil(t, d)
	// Should pick key-B (older lastUsedAt = LRU).
	assert.Equal(t, callback.RateLimitCacheKey("key-B"), callback.RateLimitCacheKey(d.APIKey()))
}

func TestLowestUtilization_RateLimited_Skipped(t *testing.T) {
	t.Parallel()
	store := callback.NewInMemoryRateLimitStore()
	setStoreState(store, "key-A", 0.10, "rate_limited")
	setStoreState(store, "key-B", 0.60, "allowed")

	lu := NewLowestUtilization(store, 80, nil)

	depA := makeDeployment("dep-A", "key-A")
	depB := makeDeployment("dep-B", "key-B")

	d := lu.Pick([]*router.Deployment{depA, depB})
	require.NotNil(t, d)
	assert.Equal(t, callback.RateLimitCacheKey("key-B"), callback.RateLimitCacheKey(d.APIKey()))
}

func TestLowestUtilization_AllRateLimited_FallbackShuffle(t *testing.T) {
	t.Parallel()
	store := callback.NewInMemoryRateLimitStore()
	setStoreState(store, "key-A", 0.99, "rate_limited")
	setStoreState(store, "key-B", 0.99, "rate_limited")

	lu := NewLowestUtilization(store, 80, nil)

	depA := makeDeployment("dep-A", "key-A")
	depB := makeDeployment("dep-B", "key-B")

	d := lu.Pick([]*router.Deployment{depA, depB})
	require.NotNil(t, d, "should return a deployment even when all are rate_limited (fallback shuffle)")
}

func TestLowestUtilization_ColdStart_NoData_Shuffle(t *testing.T) {
	t.Parallel()
	store := callback.NewInMemoryRateLimitStore()
	// No data in store at all.

	lu := NewLowestUtilization(store, 80, nil)

	depA := makeDeployment("dep-A", "key-A")
	depB := makeDeployment("dep-B", "key-B")

	d := lu.Pick([]*router.Deployment{depA, depB})
	require.NotNil(t, d, "cold start should still return a deployment via shuffle")
}

func TestLowestUtilization_Concurrent(t *testing.T) {
	t.Parallel()
	store := callback.NewInMemoryRateLimitStore()
	setStoreState(store, "key-A", 0.20, "allowed")
	setStoreState(store, "key-B", 0.40, "allowed")
	setStoreState(store, "key-C", 0.60, "allowed")

	lu := NewLowestUtilization(store, 80, nil)

	depA := makeDeployment("dep-A", "key-A")
	depB := makeDeployment("dep-B", "key-B")
	depC := makeDeployment("dep-C", "key-C")
	deps := []*router.Deployment{depA, depB, depC}

	var wg sync.WaitGroup
	results := make([]*router.Deployment, 100)
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			results[idx] = lu.Pick(deps)
		}(i)
	}
	wg.Wait()

	for i, d := range results {
		assert.NotNil(t, d, "goroutine %d got nil deployment", i)
	}
}

func TestLowestUtilization_EmptyDeployments(t *testing.T) {
	t.Parallel()
	store := callback.NewInMemoryRateLimitStore()
	lu := NewLowestUtilization(store, 80, nil)

	d := lu.Pick(nil)
	assert.Nil(t, d)
}

func TestLowestUtilization_ActiveKeyRateLimited_Switches(t *testing.T) {
	t.Parallel()
	store := callback.NewInMemoryRateLimitStore()
	setStoreState(store, "key-A", 0.50, "rate_limited") // active but rate_limited
	setStoreState(store, "key-B", 0.30, "allowed")

	var switched bool
	lu := NewLowestUtilization(store, 80, func(msg string) { switched = true })

	lu.mu.Lock()
	lu.activeKey = callback.RateLimitCacheKey("key-A")
	lu.mu.Unlock()

	depA := makeDeployment("dep-A", "key-A")
	depB := makeDeployment("dep-B", "key-B")

	d := lu.Pick([]*router.Deployment{depA, depB})
	require.NotNil(t, d)
	assert.Equal(t, callback.RateLimitCacheKey("key-B"), callback.RateLimitCacheKey(d.APIKey()))
	assert.True(t, switched, "should have triggered switch alert")
}
