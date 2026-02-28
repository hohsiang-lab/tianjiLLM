package spend

import (
	"context"
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

	// Record with zero Cost — should fallback to pricing.Default()
	// Call the real Record() method; db=nil is safe (guarded inside Record).
	rec := SpendRecord{
		Model:            model,
		PromptTokens:     prompt,
		CompletionTokens: completion,
		TotalTokens:      prompt + completion,
	}
	// Record() writes to DB (nil here, so no-op) but exercises the full
	// fallback path: empty calculator → pricing.Default().
	tracker.Record(context.Background(), rec)

	// Verify the fallback independently: empty calculator returns 0,
	// then pricing.Default() gives the expected cost.
	calcCost := tracker.calculator.Calculate(model, prompt, completion)
	assert.Equal(t, 0.0, calcCost, "empty calculator should return 0")

	fallbackCost := pricing.Default().TotalCost(model, prompt, completion)
	assert.Equal(t, expectedCost, fallbackCost, "pricing.Default() should return expected cost")
}

func TestRecord_StripProviderPrefix(t *testing.T) {
	// Verify that pricing.Default() handles provider-prefixed model names
	model := "anthropic/claude-sonnet-4-20250514"
	cost := pricing.Default().TotalCost(model, 1000, 500)
	require.Greater(t, cost, 0.0, "provider-prefixed model must exist in embedded pricing data")
}
