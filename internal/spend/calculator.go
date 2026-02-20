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
	MaxInputTokens     int     `json:"max_input_tokens"`
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
