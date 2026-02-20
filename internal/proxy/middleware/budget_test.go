package middleware

import (
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/model"
	"github.com/stretchr/testify/assert"
)

func TestModelBudgetLimiter_WithinBudget(t *testing.T) {
	m := NewModelBudgetLimiter(map[string]float64{
		"gpt-4": 10.0,
	})

	m.RecordSpend("gpt-4", 5.0)
	assert.NoError(t, m.Check("gpt-4"))
	assert.InDelta(t, 5.0, m.GetSpend("gpt-4"), 0.001)
}

func TestModelBudgetLimiter_ExceedsBudget(t *testing.T) {
	m := NewModelBudgetLimiter(map[string]float64{
		"gpt-4": 10.0,
	})

	m.RecordSpend("gpt-4", 10.0)
	err := m.Check("gpt-4")
	assert.ErrorIs(t, err, model.ErrBudgetExceeded)
}

func TestModelBudgetLimiter_NoLimit(t *testing.T) {
	m := NewModelBudgetLimiter(map[string]float64{})

	// Model without limit should always pass
	m.RecordSpend("gpt-4", 9999.0)
	assert.NoError(t, m.Check("gpt-4"))
}

func TestModelBudgetLimiter_MultipleModels(t *testing.T) {
	m := NewModelBudgetLimiter(map[string]float64{
		"gpt-4":       10.0,
		"gpt-4o-mini": 5.0,
	})

	m.RecordSpend("gpt-4", 8.0)
	m.RecordSpend("gpt-4o-mini", 5.0)

	assert.NoError(t, m.Check("gpt-4"))
	assert.ErrorIs(t, m.Check("gpt-4o-mini"), model.ErrBudgetExceeded)
}

func TestModelBudgetLimiter_Reset(t *testing.T) {
	m := NewModelBudgetLimiter(map[string]float64{
		"gpt-4": 10.0,
	})

	m.RecordSpend("gpt-4", 10.0)
	assert.ErrorIs(t, m.Check("gpt-4"), model.ErrBudgetExceeded)

	m.ResetSpend()
	assert.NoError(t, m.Check("gpt-4"))
	assert.Equal(t, 0.0, m.GetSpend("gpt-4"))
}
