package pricing

import (
	"encoding/json"
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newGPT55LongContextTestCalculator() *Calculator {
	return &Calculator{
		embedded: map[string]ModelInfo{
			"gpt-5.5": {
				InputCostPerToken:         5e-06,
				OutputCostPerToken:        3e-05,
				CacheReadCostPerToken:     5e-07,
				CacheCreationCostPerToken: 0,

				LongContextInputThresholdTokens:         above272kThresholdTokens,
				InputCostPerTokenAboveThreshold:         1e-05,
				OutputCostPerTokenAboveThreshold:        4.5e-05,
				CacheReadCostPerTokenAboveThreshold:     1e-06,
				CacheCreationCostPerTokenAboveThreshold: 0,

				Mode:     "chat",
				Provider: "openai",
			},
		},
		models:    make(map[string]ModelInfo),
		overrides: make(map[string]ModelInfo),
	}
}

func TestGPT55LongContextPricing_Below272KUsesBaseRates(t *testing.T) {
	c := newGPT55LongContextTestCalculator()

	inputSideCost, completionCost := c.Cost("gpt-5.5", TokenUsage{
		PromptTokens:     271000,
		CompletionTokens: 1000,
	})

	assert.InDelta(t, 271000*5e-06, inputSideCost, 1e-10)
	assert.InDelta(t, 1000*3e-05, completionCost, 1e-10)
}

func TestGPT55LongContextPricing_Above272KUsesLongContextRates(t *testing.T) {
	c := newGPT55LongContextTestCalculator()

	inputSideCost, completionCost := c.Cost("gpt-5.5", TokenUsage{
		PromptTokens:     273000,
		CompletionTokens: 1000,
	})

	assert.InDelta(t, 273000*1e-05, inputSideCost, 1e-10)
	assert.InDelta(t, 1000*4.5e-05, completionCost, 1e-10)
}

func TestGPT55LongContextPricing_Above272KUsesCacheReadLongContextRate(t *testing.T) {
	c := newGPT55LongContextTestCalculator()

	inputSideCost, completionCost := c.Cost("gpt-5.5", TokenUsage{
		PromptTokens:         1000,
		CacheReadInputTokens: 272001,
		CompletionTokens:     100,
	})

	assert.InDelta(t, 1000*1e-05+272001*1e-06, inputSideCost, 1e-10)
	assert.InDelta(t, 100*4.5e-05, completionCost, 1e-10)
}

func TestGPT55LongContextPricing_StrippedProviderUsesLongContextRates(t *testing.T) {
	c := newGPT55LongContextTestCalculator()

	inputSideCost, completionCost := c.Cost("openai/gpt-5.5", TokenUsage{
		PromptTokens:     273000,
		CompletionTokens: 1000,
	})

	assert.InDelta(t, 273000*1e-05, inputSideCost, 1e-10)
	assert.InDelta(t, 1000*4.5e-05, completionCost, 1e-10)
}

func TestReloadFromDBPreservesLongContextThresholdPricing(t *testing.T) {
	c := newTestCalculator()
	c.ReloadFromDB([]db.ModelPricing{
		{
			ModelName:                                 "gpt-5.5",
			InputCostPerToken:                         5e-06,
			OutputCostPerToken:                        3e-05,
			CacheReadInputTokenCost:                   5e-07,
			LongContextInputThresholdTokens:           above272kThresholdTokens,
			InputCostPerTokenAboveThreshold:           1e-05,
			OutputCostPerTokenAboveThreshold:          4.5e-05,
			CacheReadInputTokenCostAboveThreshold:     1e-06,
			CacheCreationInputTokenCostAboveThreshold: 0,
		},
	})

	inputSideCost, completionCost := c.Cost("gpt-5.5", TokenUsage{
		PromptTokens:         1000,
		CacheReadInputTokens: 272001,
		CompletionTokens:     100,
	})

	assert.InDelta(t, 1000*1e-05+272001*1e-06, inputSideCost, 1e-10)
	assert.InDelta(t, 100*4.5e-05, completionCost, 1e-10)
}

func TestModelInfoUnmarshalMapsLiteLLMAbove272KPricing(t *testing.T) {
	var info ModelInfo
	err := json.Unmarshal([]byte(`{
		"input_cost_per_token": 5e-06,
		"output_cost_per_token": 3e-05,
		"cache_read_input_token_cost": 5e-07,
		"input_cost_per_token_above_272k_tokens": 1e-05,
		"output_cost_per_token_above_272k_tokens": 4.5e-05,
		"cache_read_input_token_cost_above_272k_tokens": 1e-06
	}`), &info)
	require.NoError(t, err)

	assert.Equal(t, above272kThresholdTokens, info.LongContextInputThresholdTokens)
	assert.Equal(t, 1e-05, info.InputCostPerTokenAboveThreshold)
	assert.Equal(t, 4.5e-05, info.OutputCostPerTokenAboveThreshold)
	assert.Equal(t, 1e-06, info.CacheReadCostPerTokenAboveThreshold)
}

func TestModelInfoUnmarshalMapsGenericLongContextPricingSuffix(t *testing.T) {
	var info ModelInfo
	err := json.Unmarshal([]byte(`{
		"input_cost_per_token": 1e-07,
		"output_cost_per_token": 4e-07,
		"cache_read_input_token_cost": 5e-08,
		"input_cost_per_token_above_128k_tokens": 1.5e-07,
		"output_cost_per_token_above_128k_tokens": 6e-07,
		"cache_read_input_token_cost_above_128k_tokens": 7.5e-08
	}`), &info)
	require.NoError(t, err)

	assert.Equal(t, 128000, info.LongContextInputThresholdTokens)
	assert.Equal(t, 1.5e-07, info.InputCostPerTokenAboveThreshold)
	assert.Equal(t, 6e-07, info.OutputCostPerTokenAboveThreshold)
	assert.Equal(t, 7.5e-08, info.CacheReadCostPerTokenAboveThreshold)
}

func TestModelInfoUnmarshalFills272KCostsWhenGenericThresholdExists(t *testing.T) {
	var info ModelInfo
	err := json.Unmarshal([]byte(`{
		"long_context_input_threshold_tokens": 272000,
		"input_cost_per_token_above_272k_tokens": 1e-05,
		"output_cost_per_token_above_272k_tokens": 4.5e-05,
		"cache_read_input_token_cost_above_272k_tokens": 1e-06
	}`), &info)
	require.NoError(t, err)

	assert.Equal(t, above272kThresholdTokens, info.LongContextInputThresholdTokens)
	assert.Equal(t, 1e-05, info.InputCostPerTokenAboveThreshold)
	assert.Equal(t, 4.5e-05, info.OutputCostPerTokenAboveThreshold)
	assert.Equal(t, 1e-06, info.CacheReadCostPerTokenAboveThreshold)
}

func TestUpstreamModelEntryNormalizesLiteLLMAbove272KPricing(t *testing.T) {
	var entry upstreamModelEntry
	err := json.Unmarshal([]byte(`{
		"input_cost_per_token": 5e-06,
		"output_cost_per_token": 3e-05,
		"cache_read_input_token_cost": 5e-07,
		"input_cost_per_token_above_272k_tokens": 1e-05,
		"output_cost_per_token_above_272k_tokens": 4.5e-05,
		"cache_read_input_token_cost_above_272k_tokens": 1e-06
	}`), &entry)
	require.NoError(t, err)

	threshold, input, output, cacheRead, cacheCreation := entry.normalizedLongContextPricing()

	assert.Equal(t, above272kThresholdTokens, threshold)
	assert.Equal(t, 1e-05, input)
	assert.Equal(t, 4.5e-05, output)
	assert.Equal(t, 1e-06, cacheRead)
	assert.Zero(t, cacheCreation)
}

func TestUpstreamModelEntryNormalizesGenericLongContextPricingSuffix(t *testing.T) {
	var entry upstreamModelEntry
	err := json.Unmarshal([]byte(`{
		"input_cost_per_token": 1e-07,
		"output_cost_per_token": 4e-07,
		"cache_read_input_token_cost": 5e-08,
		"max_input_tokens": 2000000.0,
		"input_cost_per_token_above_128k_tokens": 1.5e-07,
		"output_cost_per_token_above_128k_tokens": 6e-07,
		"cache_read_input_token_cost_above_128k_tokens": 7.5e-08
	}`), &entry)
	require.NoError(t, err)

	threshold, input, output, cacheRead, cacheCreation := entry.normalizedLongContextPricing()

	assert.Equal(t, 2000000, entry.MaxInputTokens)
	assert.Equal(t, 128000, threshold)
	assert.Equal(t, 1.5e-07, input)
	assert.Equal(t, 6e-07, output)
	assert.Equal(t, 7.5e-08, cacheRead)
	assert.Zero(t, cacheCreation)
}

func TestUpstreamModelEntryNormalizes272KCostsWhenGenericThresholdExists(t *testing.T) {
	var entry upstreamModelEntry
	err := json.Unmarshal([]byte(`{
		"long_context_input_threshold_tokens": 272000,
		"input_cost_per_token_above_272k_tokens": 1e-05,
		"output_cost_per_token_above_272k_tokens": 4.5e-05,
		"cache_read_input_token_cost_above_272k_tokens": 1e-06
	}`), &entry)
	require.NoError(t, err)

	threshold, input, output, cacheRead, cacheCreation := entry.normalizedLongContextPricing()

	assert.Equal(t, above272kThresholdTokens, threshold)
	assert.Equal(t, 1e-05, input)
	assert.Equal(t, 4.5e-05, output)
	assert.Equal(t, 1e-06, cacheRead)
	assert.Zero(t, cacheCreation)
}
