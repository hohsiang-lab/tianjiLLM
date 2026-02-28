package spend

import (
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/pricing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRecord_FallbackToPricingDefault(t *testing.T) {
	tracker := NewTracker(nil, nil)

	model := "gpt-4o"
	info := pricing.Default().GetModelInfo(model)
	require.NotNil(t, info, "gpt-4o must exist in embedded pricing data")

	rec := SpendRecord{
		Model:            model,
		PromptTokens:     100,
		CompletionTokens: 50,
	}

	expectedCost := pricing.Default().TotalCost(model, pricing.TokenUsage{PromptTokens: 100, CompletionTokens: 50})
	assert.Greater(t, expectedCost, 0.0)

	got := tracker.calculateCost(rec)
	assert.Equal(t, expectedCost, got)
}

func TestRecord_StripProviderPrefix(t *testing.T) {
	model := "anthropic/claude-sonnet-4-20250514"
	cost := pricing.Default().TotalCost(model, pricing.TokenUsage{PromptTokens: 1000, CompletionTokens: 500})
	require.Greater(t, cost, 0.0, "provider-prefixed model must exist in embedded pricing data")
}
