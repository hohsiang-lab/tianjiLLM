package pricing

import (
	_ "embed"
	"encoding/json"
	"strings"
	"sync"

	"github.com/praxisllmlab/tianjiLLM/internal/db"
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
// Three-layer architecture:
//   - embedded: build-time data from model_prices.json (immutable after init)
//   - models:   DB-synced data (replaceable via ReloadFromDB)
//   - overrides: runtime custom pricing (highest priority)
type Calculator struct {
	mu        sync.RWMutex
	embedded  map[string]ModelInfo // immutable after Default() init
	models    map[string]ModelInfo // DB-synced, replaced atomically
	overrides map[string]ModelInfo // runtime custom overrides
}

var defaultCalculator *Calculator
var once sync.Once

// Default returns the singleton pricing calculator loaded from embedded data.
func Default() *Calculator {
	once.Do(func() {
		defaultCalculator = &Calculator{
			embedded:  make(map[string]ModelInfo),
			models:    make(map[string]ModelInfo),
			overrides: make(map[string]ModelInfo),
		}
		var raw map[string]json.RawMessage
		_ = json.Unmarshal(modelPricesJSON, &raw)

		for name, data := range raw {
			if name == "sample_spec" {
				continue
			}
			var info ModelInfo
			if err := json.Unmarshal(data, &info); err == nil {
				defaultCalculator.embedded[name] = info
				defaultCalculator.models[name] = info
			}
		}
	})
	return defaultCalculator
}

// ReloadFromDB atomically replaces the models layer with DB-synced data.
func (c *Calculator) ReloadFromDB(entries []db.ModelPricing) {
	newModels := make(map[string]ModelInfo, len(entries))
	for _, e := range entries {
		newModels[e.ModelName] = ModelInfo{
			InputCostPerToken:  e.InputCostPerToken,
			OutputCostPerToken: e.OutputCostPerToken,
			MaxInputTokens:     int(e.MaxInputTokens),
			MaxOutputTokens:    int(e.MaxOutputTokens),
			MaxTokens:          int(e.MaxTokens),
			Mode:               e.Mode,
			Provider:           e.Provider,
		}
	}
	c.mu.Lock()
	c.models = newModels
	c.mu.Unlock()
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

// ModelCostMap returns a merged copy of all three pricing layers.
// Priority: overrides > models (DB-synced) > embedded (build-time).
func ModelCostMap() map[string]ModelInfo {
	c := Default()
	c.mu.RLock()
	defer c.mu.RUnlock()
	merged := make(map[string]ModelInfo, len(c.embedded)+len(c.models)+len(c.overrides))
	for k, v := range c.embedded {
		merged[k] = v
	}
	for k, v := range c.models {
		merged[k] = v
	}
	for k, v := range c.overrides {
		merged[k] = v
	}
	return merged
}

// lookup finds model info with three-layer fallback: overrides → models → embedded.
// Each layer tries exact match first, then strips provider prefix.
func (c *Calculator) lookup(model string) *ModelInfo {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.lookupInLayers(model)
}

// lookupInLayers performs the actual lookup without acquiring the lock.
// Caller must hold at least a read lock.
func (c *Calculator) lookupInLayers(model string) *ModelInfo {
	bare := ""
	if idx := strings.Index(model, "/"); idx >= 0 {
		bare = model[idx+1:]
	}

	// Layer 1: overrides (highest priority)
	if info, ok := c.overrides[model]; ok {
		return &info
	}
	if bare != "" {
		if info, ok := c.overrides[bare]; ok {
			return &info
		}
	}

	// Layer 2: DB-synced models
	if info, ok := c.models[model]; ok {
		return &info
	}
	if bare != "" {
		if info, ok := c.models[bare]; ok {
			return &info
		}
	}

	// Layer 3: embedded (build-time fallback)
	if info, ok := c.embedded[model]; ok {
		return &info
	}
	if bare != "" {
		if info, ok := c.embedded[bare]; ok {
			return &info
		}
	}

	return nil
}
