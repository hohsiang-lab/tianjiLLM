package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Budget Middleware Tests (context-based) ---

func budgetTestHandler(t *testing.T, maxBudget *float64, spend float64) *httptest.ResponseRecorder {
	t.Helper()
	mw := NewBudgetMiddleware()

	called := false
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	ctx := req.Context()
	ctx = context.WithValue(ctx, tokenHashKey, "testhash123")
	ctx = context.WithValue(ctx, spendKey, spend)
	if maxBudget != nil {
		ctx = context.WithValue(ctx, maxBudgetKey, *maxBudget)
	}
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if maxBudget == nil || spend < *maxBudget {
		assert.True(t, called, "handler should have been called")
	}

	return rr
}

func TestBudgetMiddleware_SpendBelowBudget(t *testing.T) {
	maxBudget := 100.0
	rr := budgetTestHandler(t, &maxBudget, 99.50)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestBudgetMiddleware_SpendExceedsBudget(t *testing.T) {
	maxBudget := 100.0
	rr := budgetTestHandler(t, &maxBudget, 100.01)
	assert.Equal(t, http.StatusTooManyRequests, rr.Code)
}

func TestBudgetMiddleware_NullBudgetPassThrough(t *testing.T) {
	rr := budgetTestHandler(t, nil, 999.0)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestBudgetMiddleware_SpendEqualsMax(t *testing.T) {
	maxBudget := 100.0
	rr := budgetTestHandler(t, &maxBudget, 100.0)
	require.Equal(t, http.StatusTooManyRequests, rr.Code, "spend == max_budget should be rejected (>= check)")
}

func TestBudgetMiddleware_ReadsFromContext(t *testing.T) {
	mw := NewBudgetMiddleware()

	maxBudget := 50.0
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	ctx := context.WithValue(req.Context(), maxBudgetKey, maxBudget)
	ctx = context.WithValue(ctx, spendKey, 60.0)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	mw(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Fatal("should not reach handler when budget exceeded")
	})).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusTooManyRequests, rr.Code)
}

// --- ModelBudgetLimiter Tests (existing) ---

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
