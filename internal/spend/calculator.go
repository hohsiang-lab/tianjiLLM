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
	InputCostPerToken              float64 `json:"input_cost_per_token"`
	OutputCostPerToken             float64 `json:"output_cost_per_token"`
	CacheReadInputCostPerToken     float64 `json:"cache_read_input_token_cost"`
	CacheCreationInputCostPerToken float64 `json:"cache_creation_input_token_cost"`
	MaxTokens                      int     `json:"max_tokens"`
	MaxInputTokens                 int     `json:"max_input_tokens"`
}

// CacheTokens holds Anthropic prompt cache token counts for a request.
type CacheTokens struct {
	ReadInputTokens     int
	CreationInputTokens int
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

// CalculateWithCache computes cost accounting for Anthropic cache token differential pricing.
// promptTokens must already include cache read and creation tokens.
// For non-Anthropic providers, pass CacheTokens{} and the result is identical to Calculate.
func (c *Calculator) CalculateWithCache(model string, promptTokens, completionTokens int, cache CacheTokens) float64 {
	c.mu.RLock()
	p, ok := c.prices[model]
	c.mu.RUnlock()
	if !ok {
		return 0
	}

	// promptTokens = baseInput + cacheRead + cacheCreation; reverse to get pure input.
	baseInputTokens := promptTokens - cache.ReadInputTokens - cache.CreationInputTokens

	cost := float64(baseInputTokens)*p.InputCostPerToken +
		float64(completionTokens)*p.OutputCostPerToken

	readRate := p.CacheReadInputCostPerToken
	if readRate == 0 {
		readRate = p.InputCostPerToken // fallback
	}
	creationRate := p.CacheCreationInputCostPerToken
	if creationRate == 0 {
		creationRate = p.InputCostPerToken // fallback
	}

	cost += float64(cache.ReadInputTokens) * readRate
	cost += float64(cache.CreationInputTokens) * creationRate
	return cost
}
