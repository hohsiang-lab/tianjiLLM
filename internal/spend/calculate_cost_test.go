package spend

import (
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/pricing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ──────────────────────────────────────────────────────────────
// calculateCost — three-layer fallback
// ──────────────────────────────────────────────────────────────

func TestCalculateCost_RecCostDirectly(t *testing.T) {
	t.Parallel()
	tracker := NewTracker(nil, nil, nil)
	rec := SpendRecord{
		Model:            "whatever-model",
		PromptTokens:     100,
		CompletionTokens: 50,
		Cost:             0.42,
	}
	got := tracker.calculateCost(rec)
	assert.Equal(t, 0.42, got, "should use rec.Cost directly when > 0")
}

func TestCalculateCost_RecCostZero_CalculatorHit(t *testing.T) {
	t.Parallel()
	calc, _ := NewCalculator("")
	// Load a custom price into the calculator
	calc.mu.Lock()
	calc.prices["test-model"] = ModelPricing{
		InputCostPerToken:  0.001,
		OutputCostPerToken: 0.002,
	}
	calc.mu.Unlock()

	tracker := NewTracker(nil, calc, nil)
	rec := SpendRecord{
		Model:            "test-model",
		PromptTokens:     1000,
		CompletionTokens: 500,
	}
	got := tracker.calculateCost(rec)
	expected := 1000*0.001 + 500*0.002
	assert.InDelta(t, expected, got, 1e-9, "should use calculator when rec.Cost is 0")
}

func TestCalculateCost_FallbackToPricingDefault(t *testing.T) {
	t.Parallel()
	calc, _ := NewCalculator("") // empty calculator
	tracker := NewTracker(nil, calc, nil)

	model := "gpt-4o"
	info := pricing.Default().GetModelInfo(model)
	require.NotNil(t, info, "gpt-4o must exist in embedded pricing")

	rec := SpendRecord{
		Model:            model,
		PromptTokens:     100,
		CompletionTokens: 50,
	}

	got := tracker.calculateCost(rec)
	expected := pricing.Default().TotalCost(model, 100, 50)
	assert.Equal(t, expected, got)
	assert.Greater(t, got, 0.0)
}

func TestCalculateCost_AllFallbackMiss_ZeroTokens(t *testing.T) {
	t.Parallel()
	calc, _ := NewCalculator("")
	tracker := NewTracker(nil, calc, nil)

	rec := SpendRecord{
		Model:            "nonexistent-model-xyz",
		PromptTokens:     0,
		CompletionTokens: 0,
	}
	got := tracker.calculateCost(rec)
	assert.Equal(t, 0.0, got, "zero tokens should not trigger pricing.Default fallback")
}

func TestCalculateCost_AllFallbackMiss_UnknownModel(t *testing.T) {
	t.Parallel()
	calc, _ := NewCalculator("")
	tracker := NewTracker(nil, calc, nil)

	rec := SpendRecord{
		Model:            "nonexistent-model-xyz",
		PromptTokens:     100,
		CompletionTokens: 50,
	}
	got := tracker.calculateCost(rec)
	// pricing.Default() won't have this model either, so cost should be 0
	assert.Equal(t, 0.0, got, "unknown model with no pricing data should return 0")
}

func TestCalculateCost_NilCalculator(t *testing.T) {
	t.Parallel()
	tracker := NewTracker(nil, nil, nil)

	model := "gpt-4o"
	rec := SpendRecord{
		Model:            model,
		PromptTokens:     100,
		CompletionTokens: 50,
	}
	got := tracker.calculateCost(rec)
	expected := pricing.Default().TotalCost(model, 100, 50)
	assert.Equal(t, expected, got, "nil calculator should skip to pricing.Default()")
	assert.Greater(t, got, 0.0)
}

func TestCalculateCost_ProviderPrefixModel(t *testing.T) {
	t.Parallel()
	tracker := NewTracker(nil, nil, nil)

	// Provider-prefixed model should be handled by pricing.Default() which strips prefix
	model := "anthropic/claude-sonnet-4-20250514"
	rec := SpendRecord{
		Model:            model,
		PromptTokens:     1000,
		CompletionTokens: 500,
	}
	got := tracker.calculateCost(rec)
	assert.Greater(t, got, 0.0, "provider-prefixed model should resolve via pricing.Default()")
}

func TestCalculateCost_RecCostTakesPrecedenceOverCalculator(t *testing.T) {
	t.Parallel()
	calc, _ := NewCalculator("")
	calc.mu.Lock()
	calc.prices["test-model"] = ModelPricing{
		InputCostPerToken:  0.001,
		OutputCostPerToken: 0.002,
	}
	calc.mu.Unlock()

	tracker := NewTracker(nil, calc, nil)
	rec := SpendRecord{
		Model:            "test-model",
		PromptTokens:     1000,
		CompletionTokens: 500,
		Cost:             99.99,
	}
	got := tracker.calculateCost(rec)
	assert.Equal(t, 99.99, got, "rec.Cost > 0 should take precedence over calculator")
}
