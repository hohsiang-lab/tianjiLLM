package pricing

import (
	_ "embed"
	"encoding/json"
	"strings"
	"sync"
)

//go:embed model_prices.json
var modelPricesJSON []byte

// ModelInfo holds pricing and capability data for a model.
type ModelInfo struct {
	InputCostPerToken  float64 `json:"input_cost_per_token"`
	OutputCostPerToken float64 `json:"output_cost_per_token"`
	MaxInputTokens     int     `json:"max_input_tokens"`
	MaxOutputTokens    int     `json:"max_output_tokens"`
	MaxTokens          int     `json:"max_tokens"`
	Mode               string  `json:"mode"`
	Provider           string  `json:"litellm_provider"`
}

// Calculator calculates LLM request costs from token counts.
type Calculator struct {
	mu        sync.RWMutex
	models    map[string]ModelInfo
	overrides map[string]ModelInfo
}

var defaultCalculator *Calculator
var once sync.Once

// Default returns the singleton pricing calculator loaded from embedded data.
func Default() *Calculator {
	once.Do(func() {
		defaultCalculator = &Calculator{
			models:    make(map[string]ModelInfo),
			overrides: make(map[string]ModelInfo),
		}
		// Parse the sample_spec key if present, skip it
		var raw map[string]json.RawMessage
		_ = json.Unmarshal(modelPricesJSON, &raw)

		for name, data := range raw {
			if name == "sample_spec" {
				continue
			}
			var info ModelInfo
			if err := json.Unmarshal(data, &info); err == nil {
				defaultCalculator.models[name] = info
			}
		}
	})
	return defaultCalculator
}

// Cost calculates the cost in USD for a request given token counts.
// Returns (promptCost, completionCost).
func (c *Calculator) Cost(model string, promptTokens, completionTokens int) (float64, float64) {
	info := c.lookup(model)
	if info == nil {
		return 0, 0
	}
	return float64(promptTokens) * info.InputCostPerToken,
		float64(completionTokens) * info.OutputCostPerToken
}

// TotalCost returns the total cost for a request.
func (c *Calculator) TotalCost(model string, promptTokens, completionTokens int) float64 {
	prompt, completion := c.Cost(model, promptTokens, completionTokens)
	return prompt + completion
}

// SetCustomPricing registers a custom pricing override for a model.
func (c *Calculator) SetCustomPricing(model string, info ModelInfo) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.overrides[model] = info
}

// GetModelInfo returns the model info for the given model name.
func (c *Calculator) GetModelInfo(model string) *ModelInfo {
	return c.lookup(model)
}

// ModelCostMap returns the full model pricing data as a raw map.
func ModelCostMap() map[string]ModelInfo {
	c := Default()
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make(map[string]ModelInfo, len(c.models))
	for k, v := range c.models {
		result[k] = v
	}
	return result
}

// lookup finds model info, checking overrides first, then embedded data.
// Tries exact match, then provider/model format.
func (c *Calculator) lookup(model string) *ModelInfo {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Check overrides first
	if info, ok := c.overrides[model]; ok {
		return &info
	}

	// Exact match in embedded data
	if info, ok := c.models[model]; ok {
		return &info
	}

	// Try with provider prefix stripped (e.g., "openai/gpt-4o" → "gpt-4o")
	if idx := strings.Index(model, "/"); idx >= 0 {
		bare := model[idx+1:]
		if info, ok := c.overrides[bare]; ok {
			return &info
		}
		if info, ok := c.models[bare]; ok {
			return &info
		}
	}

	return nil
}
