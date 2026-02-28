package pricing

// Tests for HO-71: Cache token cost calculation + 200K threshold pricing (T06–T13).
// These tests are intentionally FAILING until the feature is implemented by 魯班.

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// newCacheTestCalculator creates an isolated Calculator with claude-sonnet-4 pricing.
// Rates match Anthropic published prices (claude-sonnet-4-20250514).
func newCacheTestCalculator() *Calculator {
	return &Calculator{
		embedded: map[string]ModelInfo{
			"claude-sonnet-4-20250514": {
				InputCostPerToken:  3e-06,
				OutputCostPerToken: 1.5e-05,

				// Cache pricing
				CacheReadCostPerToken:     3e-07,
				CacheCreationCostPerToken: 3.75e-06,

				// 200K+ threshold pricing
				InputCostPerTokenAbove200k:         6e-06,
				OutputCostPerTokenAbove200k:        2.25e-05,
				CacheReadCostPerTokenAbove200k:     6e-07,
				CacheCreationCostPerTokenAbove200k: 7.5e-06,

				Mode:     "chat",
				Provider: "anthropic",
			},
		},
		models:    make(map[string]ModelInfo),
		overrides: make(map[string]ModelInfo),
	}
}

// ─── T06: 50K cache_read → cost = 50000 × 3e-07 (NOT 3e-06) ─────────────────

// TestCacheCost_CacheReadRate verifies that cache_read tokens are billed at
// cache_read rate (3e-07), not standard input rate (3e-06).
func TestCacheCost_CacheReadRate(t *testing.T) {
	c := newCacheTestCalculator()

	_, cacheReadCost, _, _ := c.CostWithCache("claude-sonnet-4-20250514", 0, 50000, 0, 0)

	const wantCacheReadCost = 50000 * 3e-07 // $0.015
	assert.InDelta(t, wantCacheReadCost, cacheReadCost, 1e-10,
		"T06: 50K cache_read must cost 50000 × 3e-07 = $0.015 (not 3e-06)")
}

// ─── T07: 1000 cache_creation → cost = 1000 × 3.75e-06 ──────────────────────

// TestCacheCost_CacheCreationRate verifies that cache_creation tokens use the
// write rate (3.75e-06).
func TestCacheCost_CacheCreationRate(t *testing.T) {
	c := newCacheTestCalculator()

	_, _, cacheCreationCost, _ := c.CostWithCache("claude-sonnet-4-20250514", 0, 0, 1000, 0)

	const wantCreationCost = 1000 * 3.75e-06
	assert.InDelta(t, wantCreationCost, cacheCreationCost, 1e-10,
		"T07: 1000 cache_creation must cost 1000 × 3.75e-06")
}

// ─── T08: 1 input + 50K cache_read → total = 1×3e-06 + 50000×3e-07 ──────────

// TestCacheCost_MixedInputAndCacheRead verifies that input tokens and
// cache_read tokens use their respective rates and are summed correctly.
func TestCacheCost_MixedInputAndCacheRead(t *testing.T) {
	c := newCacheTestCalculator()

	inputCost, cacheReadCost, _, _ := c.CostWithCache("claude-sonnet-4-20250514", 1, 50000, 0, 0)

	const wantInputCost = 1 * 3e-06
	const wantCacheReadCost = 50000 * 3e-07
	assert.InDelta(t, wantInputCost, inputCost, 1e-12,
		"T08: input cost must be 1 × 3e-06")
	assert.InDelta(t, wantCacheReadCost, cacheReadCost, 1e-10,
		"T08: cache_read cost must be 50000 × 3e-07")

	total := inputCost + cacheReadCost
	const wantTotal = wantInputCost + wantCacheReadCost
	assert.InDelta(t, wantTotal, total, 1e-10,
		"T08: total must be sum of input + cache_read costs")
}

// ─── T09: No cache → backward-compat ─────────────────────────────────────────

// TestCacheCost_NoCache_BackwardCompat verifies that CostWithCache with zero
// cache tokens behaves identically to the original Cost() for non-cache requests.
func TestCacheCost_NoCache_BackwardCompat(t *testing.T) {
	c := newCacheTestCalculator()

	inputCost, cacheReadCost, cacheCreationCost, completionCost := c.CostWithCache(
		"claude-sonnet-4-20250514", 100, 0, 0, 50)

	assert.Equal(t, 0.0, cacheReadCost, "T09: cacheReadCost must be 0 when no cache tokens")
	assert.Equal(t, 0.0, cacheCreationCost, "T09: cacheCreationCost must be 0 when no cache tokens")

	wantInput := 100 * 3e-06
	wantCompletion := 50 * 1.5e-05
	assert.InDelta(t, wantInput, inputCost, 1e-10, "T09: input cost backward compat")
	assert.InDelta(t, wantCompletion, completionCost, 1e-10, "T09: completion cost backward compat")
}

// ─── T10: 210K prompt → input_cost uses tiered rate 6e-06 ────────────────────

// TestThresholdPricing_Above200K_UsesHigherRate verifies that when total
// prompt tokens exceed 200K, the tiered input rate (6e-06) is applied.
func TestThresholdPricing_Above200K_UsesHigherRate(t *testing.T) {
	c := newCacheTestCalculator()

	inputCost, _, _, _ := c.CostWithCache("claude-sonnet-4-20250514", 210000, 0, 0, 0)

	const wantInputCost = 210000 * 6e-06
	assert.InDelta(t, wantInputCost, inputCost, 1e-5,
		"T10: 210K prompt must use tiered rate 6e-06 (not 3e-06)")
}

// ─── T11: 190K prompt → input_cost uses standard rate 3e-06 ──────────────────

// TestThresholdPricing_Below200K_UsesStandardRate verifies that below the
// 200K threshold, the standard input rate (3e-06) is applied.
func TestThresholdPricing_Below200K_UsesStandardRate(t *testing.T) {
	c := newCacheTestCalculator()

	inputCost, _, _, _ := c.CostWithCache("claude-sonnet-4-20250514", 190000, 0, 0, 0)

	const wantInputCost = 190000 * 3e-06
	assert.InDelta(t, wantInputCost, inputCost, 1e-5,
		"T11: 190K prompt must use standard rate 3e-06")
}

// ─── T12: 210K + 50K cache_read → cache_read_cost uses tiered rate 6e-07 ─────

// TestThresholdPricing_Above200K_CacheReadTiered verifies that cache_read
// tokens also use the tiered rate when total prompt exceeds 200K.
func TestThresholdPricing_Above200K_CacheReadTiered(t *testing.T) {
	c := newCacheTestCalculator()

	// promptTokens=210000, cacheReadTokens=50000
	// total context = 210000 + 50000 = 260000 > 200K → tiered rates
	_, cacheReadCost, _, _ := c.CostWithCache("claude-sonnet-4-20250514", 210000, 50000, 0, 0)

	const wantCacheReadCost = 50000 * 6e-07
	assert.InDelta(t, wantCacheReadCost, cacheReadCost, 1e-9,
		"T12: 50K cache_read at 210K context must use tiered rate 6e-07 (not 3e-07)")
}

// ─── T13: Model without threshold fields → no error ──────────────────────────

// TestThresholdPricing_NoThresholdFields_NoError verifies that a model without
// threshold pricing fields falls back to standard rates without panicking.
func TestThresholdPricing_NoThresholdFields_NoError(t *testing.T) {
	c := &Calculator{
		embedded: map[string]ModelInfo{
			"basic-model": {
				InputCostPerToken:  1e-06,
				OutputCostPerToken: 2e-06,
				// No cache or threshold fields — all zero
			},
		},
		models:    make(map[string]ModelInfo),
		overrides: make(map[string]ModelInfo),
	}

	assert.NotPanics(t, func() {
		inputCost, cacheReadCost, cacheCreationCost, completionCost := c.CostWithCache("basic-model", 300000, 0, 0, 100)
		// Should not error; use standard rates (no threshold defined)
		assert.Equal(t, 0.0, cacheReadCost, "T13: no cache tokens, cacheReadCost=0")
		assert.Equal(t, 0.0, cacheCreationCost, "T13: no cache tokens, cacheCreationCost=0")
		// When threshold fields are zero, fall back to standard rate
		wantInput := 300000 * 1e-06
		assert.InDelta(t, wantInput, inputCost, 1e-5, "T13: fallback to standard input rate")
		_ = completionCost
	}, "T13: model without threshold fields must not panic")
}
