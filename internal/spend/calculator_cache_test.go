package spend

import (
	"testing"
)

func TestCalculateWithCache_CacheReadDiscountedRate(t *testing.T) {
	c := &Calculator{
		prices: map[string]ModelPricing{
			"claude-3-5-sonnet": {
				InputCostPerToken:              3.0 / 1_000_000,
				OutputCostPerToken:             15.0 / 1_000_000,
				CacheReadInputCostPerToken:     0.3 / 1_000_000,
				CacheCreationInputCostPerToken: 3.75 / 1_000_000,
			},
		},
	}

	cache := CacheTokens{
		CacheReadInputTokens:     1000,
		CacheCreationInputTokens: 500,
	}
	// prompt=2000 means: 2000 - 1000 - 500 = 500 regular tokens
	cost := c.CalculateWithCache("claude-3-5-sonnet", 2000, 100, cache)

	// Expected:
	// regular: 500 * 3.0/1M = 0.0000015
	// cache read: 1000 * 0.3/1M = 0.0000003
	// cache creation: 500 * 3.75/1M = 0.000001875
	// completion: 100 * 15.0/1M = 0.0000015
	// total = 0.000005775
	expected := 500*(3.0/1_000_000) + 1000*(0.3/1_000_000) + 500*(3.75/1_000_000) + 100*(15.0/1_000_000)
	if cost != expected {
		t.Errorf("CalculateWithCache = %v, want %v", cost, expected)
	}
}

func TestCalculateWithCache_NoCacheTokens(t *testing.T) {
	c := &Calculator{
		prices: map[string]ModelPricing{
			"gpt-4o": {
				InputCostPerToken:  2.5 / 1_000_000,
				OutputCostPerToken: 10.0 / 1_000_000,
			},
		},
	}

	cost := c.CalculateWithCache("gpt-4o", 1000, 200, CacheTokens{})
	expected := 1000*(2.5/1_000_000) + 200*(10.0/1_000_000)
	if diff := cost - expected; diff > 1e-10 || diff < -1e-10 {
		t.Errorf("CalculateWithCache (no cache) = %v, want %v", cost, expected)
	}
}

func TestCalculateWithCache_FallbackToInputRate(t *testing.T) {
	// Model has no cache rates configured — should fall back to InputCostPerToken
	c := &Calculator{
		prices: map[string]ModelPricing{
			"some-model": {
				InputCostPerToken:  1.0 / 1_000_000,
				OutputCostPerToken: 2.0 / 1_000_000,
			},
		},
	}

	cache := CacheTokens{CacheReadInputTokens: 500}
	cost := c.CalculateWithCache("some-model", 800, 50, cache)
	// regular: 800 - 500 = 300
	// cache read falls back to InputCostPerToken = 1.0/1M
	expected := 300*(1.0/1_000_000) + 500*(1.0/1_000_000) + 50*(2.0/1_000_000)
	if cost != expected {
		t.Errorf("FallbackToInputRate = %v, want %v", cost, expected)
	}
}
