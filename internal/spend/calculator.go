package spend

import (
	"encoding/json"
	"os"
	"sync"
)

// Calculator computes the cost of a model call based on token counts and model pricing.
type Calculator struct {
	mu     sync.RWMutex
	prices map[string]ModelPricing
}

// ModelPricing holds cost per token for a model.
type ModelPricing struct {
	InputCostPerToken  float64 `json:"input_cost_per_token"`
	OutputCostPerToken float64 `json:"output_cost_per_token"`
	MaxTokens          int     `json:"max_tokens"`
	MaxInputTokens               int     `json:"max_input_tokens"`
	CacheReadInputCostPerToken     float64 `json:"cache_read_input_token_cost"`
	CacheCreationInputCostPerToken float64 `json:"cache_creation_input_token_cost"`
}

// NewCalculator creates a Calculator with pricing loaded from a JSON file.
func NewCalculator(pricingPath string) (*Calculator, error) {
	c := &Calculator{prices: make(map[string]ModelPricing)}

	if pricingPath == "" {
		return c, nil
	}

	data, err := os.ReadFile(pricingPath)
	if err != nil {
		return c, nil // pricing file is optional
	}

	var prices map[string]ModelPricing
	if err := json.Unmarshal(data, &prices); err != nil {
		return c, nil
	}

	c.prices = prices
	return c, nil
}

// Calculate returns the cost of a call based on prompt/completion tokens.
func (c *Calculator) Calculate(model string, promptTokens, completionTokens int) float64 {
	c.mu.RLock()
	pricing, ok := c.prices[model]
	c.mu.RUnlock()

	if !ok {
		return 0
	}

	return float64(promptTokens)*pricing.InputCostPerToken +
		float64(completionTokens)*pricing.OutputCostPerToken
}

// CacheTokens holds Anthropic cache token counts for a request.
type CacheTokens struct {
	CacheReadInputTokens     int
	CacheCreationInputTokens int
}

// CalculateWithCache returns the cost of a call, applying differential cache pricing.
// Falls back to standard InputCostPerToken when cache rates are absent.
func (c *Calculator) CalculateWithCache(model string, promptTokens, completionTokens int, cache CacheTokens) float64 {
	c.mu.RLock()
	p, ok := c.prices[model]
	c.mu.RUnlock()

	if !ok {
		return 0
	}

	cost := float64(completionTokens) * p.OutputCostPerToken

	cacheReadRate := p.InputCostPerToken
	if p.CacheReadInputCostPerToken > 0 {
		cacheReadRate = p.CacheReadInputCostPerToken
	}

	cacheCreationRate := p.InputCostPerToken
	if p.CacheCreationInputCostPerToken > 0 {
		cacheCreationRate = p.CacheCreationInputCostPerToken
	}

	regularPrompt := promptTokens - cache.CacheReadInputTokens - cache.CacheCreationInputTokens
	if regularPrompt < 0 {
		regularPrompt = 0
	}

	cost += float64(regularPrompt) * p.InputCostPerToken
	cost += float64(cache.CacheReadInputTokens) * cacheReadRate
	cost += float64(cache.CacheCreationInputTokens) * cacheCreationRate

	return cost
}
