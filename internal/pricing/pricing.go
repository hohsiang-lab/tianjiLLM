package pricing

import (
	_ "embed"
	"encoding/json"
	"strconv"
	"strings"
	"sync"

	"github.com/praxisllmlab/tianjiLLM/internal/db"
)

//go:embed model_prices.json
var modelPricesJSON []byte

const (
	legacyAbove200kThresholdTokens = 200000
	above272kThresholdTokens       = 272000
)

// ModelInfo holds pricing and capability data for a model.
type ModelInfo struct {
	InputCostPerToken  float64 `json:"input_cost_per_token"`
	OutputCostPerToken float64 `json:"output_cost_per_token"`
	MaxInputTokens     int     `json:"max_input_tokens"`
	MaxOutputTokens    int     `json:"max_output_tokens"`
	MaxTokens          int     `json:"max_tokens"`
	Mode               string  `json:"mode"`
	Provider           string  `json:"litellm_provider"`

	// Cache token pricing (Anthropic prompt cache)
	CacheReadCostPerToken     float64 `json:"cache_read_input_token_cost"`
	CacheCreationCostPerToken float64 `json:"cache_creation_input_token_cost"`

	// Threshold pricing (Anthropic 200K+ context)
	InputCostPerTokenAbove200k         float64 `json:"input_cost_per_token_above_200k_tokens"`
	OutputCostPerTokenAbove200k        float64 `json:"output_cost_per_token_above_200k_tokens"`
	CacheReadCostPerTokenAbove200k     float64 `json:"cache_read_input_token_cost_above_200k_tokens"`
	CacheCreationCostPerTokenAbove200k float64 `json:"cache_creation_input_token_cost_above_200k_tokens"`

	// Generic long-context threshold pricing (for example OpenAI GPT-5.5 >272K input tokens).
	LongContextInputThresholdTokens         int     `json:"long_context_input_threshold_tokens"`
	InputCostPerTokenAboveThreshold         float64 `json:"input_cost_per_token_above_threshold"`
	OutputCostPerTokenAboveThreshold        float64 `json:"output_cost_per_token_above_threshold"`
	CacheReadCostPerTokenAboveThreshold     float64 `json:"cache_read_input_token_cost_above_threshold"`
	CacheCreationCostPerTokenAboveThreshold float64 `json:"cache_creation_input_token_cost_above_threshold"`
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
			InputCostPerToken:                       e.InputCostPerToken,
			OutputCostPerToken:                      e.OutputCostPerToken,
			MaxInputTokens:                          int(e.MaxInputTokens),
			MaxOutputTokens:                         int(e.MaxOutputTokens),
			MaxTokens:                               int(e.MaxTokens),
			Mode:                                    e.Mode,
			Provider:                                e.Provider,
			CacheReadCostPerToken:                   e.CacheReadInputTokenCost,
			CacheCreationCostPerToken:               e.CacheCreationInputTokenCost,
			CacheReadCostPerTokenAbove200k:          e.CacheReadInputTokenCostAbove200k,
			CacheCreationCostPerTokenAbove200k:      e.CacheCreationInputTokenCostAbove200k,
			LongContextInputThresholdTokens:         int(e.LongContextInputThresholdTokens),
			InputCostPerTokenAboveThreshold:         e.InputCostPerTokenAboveThreshold,
			OutputCostPerTokenAboveThreshold:        e.OutputCostPerTokenAboveThreshold,
			CacheReadCostPerTokenAboveThreshold:     e.CacheReadInputTokenCostAboveThreshold,
			CacheCreationCostPerTokenAboveThreshold: e.CacheCreationInputTokenCostAboveThreshold,
		}
	}
	c.mu.Lock()
	c.models = newModels
	c.mu.Unlock()
}

// TokenUsage carries all token counts for a single LLM call.
// PromptTokens is the regular (non-cache) input token count only.
// CacheReadInputTokens and CacheCreationInputTokens are tracked separately.
// Threshold pricing is evaluated against the sum of all three input-side counts.
type TokenUsage struct {
	PromptTokens             int // regular input tokens only (excludes cache_read and cache_creation)
	CompletionTokens         int
	CacheReadInputTokens     int
	CacheCreationInputTokens int
}

// Cost calculates the cost in USD for a request given token counts.
// Returns (inputSideCost, outputCost) where inputSideCost includes regular
// input, cache_read, and cache_creation costs (with long-context threshold applied).
func (c *Calculator) Cost(model string, usage TokenUsage) (float64, float64) {
	info := c.lookup(model)
	if info == nil {
		return 0, 0
	}

	inputRate := info.InputCostPerToken
	outputRate := info.OutputCostPerToken
	cacheReadRate := info.CacheReadCostPerToken
	cacheCreationRate := info.CacheCreationCostPerToken

	totalTokens := usage.PromptTokens + usage.CacheReadInputTokens + usage.CacheCreationInputTokens
	if threshold, ok := info.thresholdPricing(); ok && totalTokens > threshold.inputTokens {
		inputRate = costRateWithOverride(inputRate, threshold.inputRate)
		outputRate = costRateWithOverride(outputRate, threshold.outputRate)
		cacheReadRate = costRateWithOverride(cacheReadRate, threshold.cacheReadRate)
		cacheCreationRate = costRateWithOverride(cacheCreationRate, threshold.cacheCreationRate)
	}

	promptCost := float64(usage.PromptTokens)*inputRate +
		float64(usage.CacheReadInputTokens)*cacheReadRate +
		float64(usage.CacheCreationInputTokens)*cacheCreationRate
	completionCost := float64(usage.CompletionTokens) * outputRate

	return promptCost, completionCost
}

type thresholdPricing struct {
	inputTokens       int
	inputRate         float64
	outputRate        float64
	cacheReadRate     float64
	cacheCreationRate float64
}

func (m ModelInfo) thresholdPricing() (thresholdPricing, bool) {
	if m.LongContextInputThresholdTokens > 0 {
		return thresholdPricing{
			inputTokens:       m.LongContextInputThresholdTokens,
			inputRate:         m.InputCostPerTokenAboveThreshold,
			outputRate:        m.OutputCostPerTokenAboveThreshold,
			cacheReadRate:     m.CacheReadCostPerTokenAboveThreshold,
			cacheCreationRate: m.CacheCreationCostPerTokenAboveThreshold,
		}, true
	}

	if m.InputCostPerTokenAbove200k > 0 ||
		m.OutputCostPerTokenAbove200k > 0 ||
		m.CacheReadCostPerTokenAbove200k > 0 ||
		m.CacheCreationCostPerTokenAbove200k > 0 {
		return thresholdPricing{
			inputTokens:       legacyAbove200kThresholdTokens,
			inputRate:         m.InputCostPerTokenAbove200k,
			outputRate:        m.OutputCostPerTokenAbove200k,
			cacheReadRate:     m.CacheReadCostPerTokenAbove200k,
			cacheCreationRate: m.CacheCreationCostPerTokenAbove200k,
		}, true
	}

	return thresholdPricing{}, false
}

func costRateWithOverride(baseRate, overrideRate float64) float64 {
	if overrideRate > 0 {
		return overrideRate
	}
	return baseRate
}

func mergeThresholdPricing(base thresholdPricing, override thresholdPricing) thresholdPricing {
	base.inputRate = costRateWithOverride(base.inputRate, override.inputRate)
	base.outputRate = costRateWithOverride(base.outputRate, override.outputRate)
	base.cacheReadRate = costRateWithOverride(base.cacheReadRate, override.cacheReadRate)
	base.cacheCreationRate = costRateWithOverride(base.cacheCreationRate, override.cacheCreationRate)
	return base
}

func normalizeLongContextPricingFromJSON(data []byte, explicit thresholdPricing) (thresholdPricing, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return explicit, err
	}

	byThreshold := make(map[int]thresholdPricing)
	for key, value := range raw {
		fieldName, threshold, ok := parseThresholdPricingKey(key)
		if !ok {
			continue
		}

		var rate float64
		if err := json.Unmarshal(value, &rate); err != nil {
			return explicit, err
		}
		if rate <= 0 {
			continue
		}

		pricing := byThreshold[threshold]
		pricing.inputTokens = threshold
		switch fieldName {
		case "input_cost_per_token":
			pricing.inputRate = rate
		case "output_cost_per_token":
			pricing.outputRate = rate
		case "cache_read_input_token_cost":
			pricing.cacheReadRate = rate
		case "cache_creation_input_token_cost":
			pricing.cacheCreationRate = rate
		}
		byThreshold[threshold] = pricing
	}

	if len(byThreshold) == 0 {
		return explicit, nil
	}

	threshold := explicit.inputTokens
	if threshold == 0 {
		for candidate := range byThreshold {
			if threshold == 0 || candidate < threshold {
				threshold = candidate
			}
		}
	}

	suffixPricing, ok := byThreshold[threshold]
	if !ok {
		return explicit, nil
	}

	explicit.inputTokens = threshold
	return mergeThresholdPricing(explicit, suffixPricing), nil
}

func parseThresholdPricingKey(key string) (fieldName string, threshold int, ok bool) {
	const (
		marker = "_above_"
		suffix = "k_tokens"
	)

	if !strings.HasSuffix(key, suffix) {
		return "", 0, false
	}

	idx := strings.LastIndex(key, marker)
	if idx < 0 {
		return "", 0, false
	}

	fieldName = key[:idx]
	switch fieldName {
	case "input_cost_per_token",
		"output_cost_per_token",
		"cache_read_input_token_cost",
		"cache_creation_input_token_cost":
	default:
		return "", 0, false
	}

	thresholdText := strings.TrimSuffix(key[idx+len(marker):], suffix)
	thresholdK, err := strconv.Atoi(thresholdText)
	if err != nil || thresholdK <= 0 {
		return "", 0, false
	}

	return fieldName, thresholdK * 1000, true
}

// TotalCost returns the total cost for a request.
func (c *Calculator) TotalCost(model string, usage TokenUsage) float64 {
	prompt, completion := c.Cost(model, usage)
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

// modelInfoJSON is the intermediate type used for JSON unmarshaling of ModelInfo.
// The token count fields use float64 to accept both integer and float JSON values
// (e.g., 2000000 and 2000000.0 are both valid in the upstream model_prices.json).
type modelInfoJSON struct {
	InputCostPerToken  float64 `json:"input_cost_per_token"`
	OutputCostPerToken float64 `json:"output_cost_per_token"`
	MaxInputTokens     float64 `json:"max_input_tokens"`
	MaxOutputTokens    float64 `json:"max_output_tokens"`
	MaxTokens          float64 `json:"max_tokens"`
	Mode               string  `json:"mode"`
	Provider           string  `json:"litellm_provider"`

	CacheReadCostPerToken     float64 `json:"cache_read_input_token_cost"`
	CacheCreationCostPerToken float64 `json:"cache_creation_input_token_cost"`

	InputCostPerTokenAbove200k         float64 `json:"input_cost_per_token_above_200k_tokens"`
	OutputCostPerTokenAbove200k        float64 `json:"output_cost_per_token_above_200k_tokens"`
	CacheReadCostPerTokenAbove200k     float64 `json:"cache_read_input_token_cost_above_200k_tokens"`
	CacheCreationCostPerTokenAbove200k float64 `json:"cache_creation_input_token_cost_above_200k_tokens"`

	LongContextInputThresholdTokens         float64 `json:"long_context_input_threshold_tokens"`
	InputCostPerTokenAboveThreshold         float64 `json:"input_cost_per_token_above_threshold"`
	OutputCostPerTokenAboveThreshold        float64 `json:"output_cost_per_token_above_threshold"`
	CacheReadCostPerTokenAboveThreshold     float64 `json:"cache_read_input_token_cost_above_threshold"`
	CacheCreationCostPerTokenAboveThreshold float64 `json:"cache_creation_input_token_cost_above_threshold"`
}

// UnmarshalJSON implements json.Unmarshaler for ModelInfo.
// It handles float JSON values for integer fields (e.g. max_input_tokens: 2000000.0).
func (m *ModelInfo) UnmarshalJSON(data []byte) error {
	var raw modelInfoJSON
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	m.InputCostPerToken = raw.InputCostPerToken
	m.OutputCostPerToken = raw.OutputCostPerToken
	m.MaxInputTokens = int(raw.MaxInputTokens)
	m.MaxOutputTokens = int(raw.MaxOutputTokens)
	m.MaxTokens = int(raw.MaxTokens)
	m.Mode = raw.Mode
	m.Provider = raw.Provider
	m.CacheReadCostPerToken = raw.CacheReadCostPerToken
	m.CacheCreationCostPerToken = raw.CacheCreationCostPerToken
	m.InputCostPerTokenAbove200k = raw.InputCostPerTokenAbove200k
	m.OutputCostPerTokenAbove200k = raw.OutputCostPerTokenAbove200k
	m.CacheReadCostPerTokenAbove200k = raw.CacheReadCostPerTokenAbove200k
	m.CacheCreationCostPerTokenAbove200k = raw.CacheCreationCostPerTokenAbove200k

	threshold, err := normalizeLongContextPricingFromJSON(data, thresholdPricing{
		inputTokens:       int(raw.LongContextInputThresholdTokens),
		inputRate:         raw.InputCostPerTokenAboveThreshold,
		outputRate:        raw.OutputCostPerTokenAboveThreshold,
		cacheReadRate:     raw.CacheReadCostPerTokenAboveThreshold,
		cacheCreationRate: raw.CacheCreationCostPerTokenAboveThreshold,
	})
	if err != nil {
		return err
	}

	m.LongContextInputThresholdTokens = threshold.inputTokens
	m.InputCostPerTokenAboveThreshold = threshold.inputRate
	m.OutputCostPerTokenAboveThreshold = threshold.outputRate
	m.CacheReadCostPerTokenAboveThreshold = threshold.cacheReadRate
	m.CacheCreationCostPerTokenAboveThreshold = threshold.cacheCreationRate
	return nil
}
