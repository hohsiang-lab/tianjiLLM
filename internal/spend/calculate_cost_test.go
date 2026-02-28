package spend

import (
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/pricing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ──────────────────────────────────────────────────────────────
// calculateCost — fallback logic
// ──────────────────────────────────────────────────────────────

func TestCalculateCost_RecCostDirectly(t *testing.T) {
	t.Parallel()
	tracker := NewTracker(nil, nil)
	rec := SpendRecord{
		Model:            "whatever-model",
		PromptTokens:     100,
		CompletionTokens: 50,
		Cost:             0.42,
	}
	got := tracker.calculateCost(rec)
	assert.Equal(t, 0.42, got, "should use rec.Cost directly when > 0")
}

func TestCalculateCost_FallbackToPricingDefault(t *testing.T) {
	t.Parallel()
	tracker := NewTracker(nil, nil)

	model := "gpt-4o"
	info := pricing.Default().GetModelInfo(model)
	require.NotNil(t, info, "gpt-4o must exist in embedded pricing")

	rec := SpendRecord{
		Model:            model,
		PromptTokens:     100,
		CompletionTokens: 50,
	}

	got := tracker.calculateCost(rec)
	expected := pricing.Default().TotalCost(model, pricing.TokenUsage{PromptTokens: 100, CompletionTokens: 50})
	assert.Equal(t, expected, got)
	assert.Greater(t, got, 0.0)
}

func TestCalculateCost_AllFallbackMiss_ZeroTokens(t *testing.T) {
	t.Parallel()
	tracker := NewTracker(nil, nil)

	rec := SpendRecord{
		Model:            "nonexistent-model-xyz",
		PromptTokens:     0,
		CompletionTokens: 0,
	}
	got := tracker.calculateCost(rec)
	assert.Equal(t, 0.0, got)
}

func TestCalculateCost_AllFallbackMiss_UnknownModel(t *testing.T) {
	t.Parallel()
	tracker := NewTracker(nil, nil)

	rec := SpendRecord{
		Model:            "nonexistent-model-xyz",
		PromptTokens:     100,
		CompletionTokens: 50,
	}
	got := tracker.calculateCost(rec)
	assert.Equal(t, 0.0, got, "unknown model with no pricing data should return 0")
}

func TestCalculateCost_ProviderPrefixModel(t *testing.T) {
	t.Parallel()
	tracker := NewTracker(nil, nil)

	model := "anthropic/claude-sonnet-4-20250514"
	rec := SpendRecord{
		Model:            model,
		PromptTokens:     1000,
		CompletionTokens: 500,
	}
	got := tracker.calculateCost(rec)
	assert.Greater(t, got, 0.0, "provider-prefixed model should resolve via pricing.Default()")
}

func TestCalculateCost_CacheTokens_IncludedInCost(t *testing.T) {
	t.Parallel()
	tracker := NewTracker(nil, nil)

	// claude-sonnet-4-20250514 has cache pricing
	rec := SpendRecord{
		Model:                    "claude-sonnet-4-20250514",
		PromptTokens:             50001, // total (1 regular + 50000 cache_read)
		CompletionTokens:         500,
		CacheReadInputTokens:     50000,
		CacheCreationInputTokens: 0,
	}
	got := tracker.calculateCost(rec)
	// cache_read fee alone: 50000 × 3e-07 = $0.015
	assert.Greater(t, got, 0.01, "cost must include cache_read fee")
}

func TestCalculateCost_RecCostTakesPrecedence(t *testing.T) {
	t.Parallel()
	tracker := NewTracker(nil, nil)
	rec := SpendRecord{
		Model:            "test-model",
		PromptTokens:     1000,
		CompletionTokens: 500,
		Cost:             99.99,
	}
	got := tracker.calculateCost(rec)
	assert.Equal(t, 99.99, got, "rec.Cost > 0 should take precedence")
}
