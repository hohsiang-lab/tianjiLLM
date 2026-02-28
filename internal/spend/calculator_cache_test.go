package spend

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func pricesWithCache(t *testing.T) *Calculator {
	t.Helper()
	prices := map[string]ModelPricing{
		"claude-3-5-sonnet-20241022": {
			InputCostPerToken:              0.000003,
			OutputCostPerToken:             0.000015,
			CacheReadInputCostPerToken:     0.0000003,
			CacheCreationInputCostPerToken: 0.00000375,
		},
		"gpt-4o": {
			InputCostPerToken:  0.000005,
			OutputCostPerToken: 0.000015,
		},
	}
	data, _ := json.Marshal(prices)
	path := filepath.Join(t.TempDir(), "prices.json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	c, err := NewCalculator(path)
	if err != nil {
		t.Fatalf("NewCalculator: %v", err)
	}
	return c
}

// AC-3: 50K cache read tokens @ $0.30/M = $0.015
func TestCalculateWithCache_CacheRead(t *testing.T) {
	c := pricesWithCache(t)
	// promptTokens = 0 base + 50000 cache read; completionTokens = 0
	cost := c.CalculateWithCache("claude-3-5-sonnet-20241022", 50000, 0, CacheTokens{ReadInputTokens: 50000})
	want := 50000 * 0.0000003
	if cost != want {
		t.Fatalf("cache read cost = %f, want %f", cost, want)
	}
}

// AC-4: 10K cache creation tokens @ $3.75/M = $0.0375
func TestCalculateWithCache_CacheCreation(t *testing.T) {
	c := pricesWithCache(t)
	cost := c.CalculateWithCache("claude-3-5-sonnet-20241022", 10000, 0, CacheTokens{CreationInputTokens: 10000})
	want := 10000 * 0.00000375
	if cost != want {
		t.Fatalf("cache creation cost = %f, want %f", cost, want)
	}
}

// AC-5: CacheTokens{} zero value = same as Calculate
func TestCalculateWithCache_NoCache(t *testing.T) {
	c := pricesWithCache(t)
	prompt, completion := 1000, 500
	want := c.Calculate("claude-3-5-sonnet-20241022", prompt, completion)
	got := c.CalculateWithCache("claude-3-5-sonnet-20241022", prompt, completion, CacheTokens{})
	if got != want {
		t.Fatalf("no-cache: got %f, want %f", got, want)
	}
}

// AC-5 for non-Anthropic model
func TestCalculateWithCache_NoCache_GPT4o(t *testing.T) {
	c := pricesWithCache(t)
	prompt, completion := 200, 100
	want := c.Calculate("gpt-4o", prompt, completion)
	got := c.CalculateWithCache("gpt-4o", prompt, completion, CacheTokens{})
	if got != want {
		t.Fatalf("gpt4o no-cache: got %f, want %f", got, want)
	}
}

// AC-6: Pricing JSON without cache rates falls back to input_cost_per_token
func TestCalculateWithCache_FallbackToInputRate(t *testing.T) {
	c := pricesWithCache(t) // gpt-4o has no cache rates
	// 1000 cache read tokens, fallback to input rate $0.000005
	cost := c.CalculateWithCache("gpt-4o", 1000, 0, CacheTokens{ReadInputTokens: 1000})
	want := 1000 * 0.000005
	if cost != want {
		t.Fatalf("fallback cost = %f, want %f", cost, want)
	}
}

// Unknown model returns 0
func TestCalculateWithCache_UnknownModel(t *testing.T) {
	c := pricesWithCache(t)
	cost := c.CalculateWithCache("unknown-model", 1000, 500, CacheTokens{ReadInputTokens: 100})
	if cost != 0 {
		t.Fatalf("unknown model cost = %f, want 0", cost)
	}
}

// Mixed: base input + cache read + cache creation + completion
func TestCalculateWithCache_Mixed(t *testing.T) {
	c := pricesWithCache(t)
	// promptTokens = 500 base + 1000 cache_read + 200 cache_creation = 1700
	// completionTokens = 300
	cache := CacheTokens{ReadInputTokens: 1000, CreationInputTokens: 200}
	cost := c.CalculateWithCache("claude-3-5-sonnet-20241022", 1700, 300, cache)
	want := 500*0.000003 + 300*0.000015 + 1000*0.0000003 + 200*0.00000375
	if cost != want {
		t.Fatalf("mixed cost = %f, want %f", cost, want)
	}
}
