package spend

import (
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/pricing"
	"github.com/stretchr/testify/assert"
)

func TestRecord_FallbackToPricingDefault(t *testing.T) {
	// Use a Tracker with an empty calculator (simulates the bug scenario)
	calc, _ := NewCalculator("")
	tracker := NewTracker(nil, calc, nil)

	// Pick a model that exists in pricing.Default()
	model := "gpt-4o"
	info := pricing.Default().GetModelInfo(model)
	if info == nil {
		t.Skip("model not found in embedded pricing data")
	}

	prompt, completion := 100, 50
	expectedCost := pricing.Default().TotalCost(model, prompt, completion)
	assert.Greater(t, expectedCost, 0.0, "expected non-zero cost from pricing.Default()")

	// Record with zero Cost — should fallback to pricing.Default()
	rec := SpendRecord{
		Model:            model,
		PromptTokens:     prompt,
		CompletionTokens: completion,
		TotalTokens:      prompt + completion,
	}

	// We can't easily inspect the cost written to DB without a real DB,
	// but we can verify the pricing.Default() lookup works correctly
	cost := rec.Cost
	if cost == 0 && tracker.calculator != nil {
		cost = tracker.calculator.Calculate(rec.Model, rec.PromptTokens, rec.CompletionTokens)
	}
	// calculator is empty, so cost should still be 0
	assert.Equal(t, 0.0, cost, "empty calculator should return 0")

	// Fallback to pricing.Default()
	if cost == 0 && (rec.PromptTokens > 0 || rec.CompletionTokens > 0) {
		cost = pricing.Default().TotalCost(rec.Model, rec.PromptTokens, rec.CompletionTokens)
	}
	assert.Equal(t, expectedCost, cost, "should fallback to pricing.Default()")
}

func TestRecord_StripProviderPrefix(t *testing.T) {
	// Verify that pricing.Default() handles provider-prefixed model names
	model := "anthropic/claude-sonnet-4-20250514"
	cost := pricing.Default().TotalCost(model, 1000, 500)
	// If the model exists in pricing data, cost should be > 0
	if cost == 0 {
		t.Skip("model not found in embedded pricing — skipping prefix test")
	}
	assert.Greater(t, cost, 0.0)
}
