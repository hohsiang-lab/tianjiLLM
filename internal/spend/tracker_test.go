package spend

import (
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/pricing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRecord_FallbackToPricingDefault(t *testing.T) {
	// Use a Tracker with an empty calculator (simulates the bug scenario)
	calc, _ := NewCalculator("")
	tracker := NewTracker(nil, calc, nil)

	// Pick a model that exists in pricing.Default()
	model := "gpt-4o"
	info := pricing.Default().GetModelInfo(model)
	require.NotNil(t, info, "gpt-4o must exist in embedded pricing data")

	prompt, completion := 100, 50
	expectedCost := pricing.Default().TotalCost(model, prompt, completion)
	assert.Greater(t, expectedCost, 0.0, "expected non-zero cost from pricing.Default()")

	rec := SpendRecord{
		Model:            model,
		PromptTokens:     prompt,
		CompletionTokens: completion,
		TotalTokens:      prompt + completion,
	}

	// Empty calculator should return 0
	calcCost := tracker.calculator.Calculate(model, prompt, completion)
	assert.Equal(t, 0.0, calcCost, "empty calculator should return 0")

	// calculateCost should fallback to pricing.Default() and return the expected cost
	got := tracker.calculateCost(rec)
	assert.Equal(t, expectedCost, got, "calculateCost should return expected cost via pricing.Default() fallback")
}

func TestRecord_StripProviderPrefix(t *testing.T) {
	// Verify that pricing.Default() handles provider-prefixed model names
	model := "anthropic/claude-sonnet-4-20250514"
	cost := pricing.Default().TotalCost(model, 1000, 500)
	require.Greater(t, cost, 0.0, "provider-prefixed model must exist in embedded pricing data")
}
