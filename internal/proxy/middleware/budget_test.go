package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/db"
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

func TestNewBudgetMiddleware_NilChecker(t *testing.T) {
	mw := NewBudgetMiddleware(nil)
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	mw(next).ServeHTTP(rec, req)
	if !called {
		t.Fatal("next handler not called with nil budget checker")
	}
}

// mockBudgetChecker implements BudgetChecker for testing.
type mockBudgetChecker struct {
	token db.VerificationToken
	err   error
}

func (m *mockBudgetChecker) GetVerificationToken(_ context.Context, _ string) (db.VerificationToken, error) {
	return m.token, m.err
}

func withTokenHash(r *http.Request, hash string) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), tokenHashKey, hash))
}

func TestNewBudgetMiddleware_NoTokenInContext(t *testing.T) {
	checker := &mockBudgetChecker{token: db.VerificationToken{}}
	mw := NewBudgetMiddleware(checker)
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	mw(next).ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestNewBudgetMiddleware_DBError(t *testing.T) {
	checker := &mockBudgetChecker{err: errors.New("db error")}
	mw := NewBudgetMiddleware(checker)
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	req := withTokenHash(httptest.NewRequest(http.MethodGet, "/", nil), "hash123")
	w := httptest.NewRecorder()
	mw(next).ServeHTTP(w, req)
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestNewBudgetMiddleware_BudgetExceeded(t *testing.T) {
	maxBudget := 10.0
	checker := &mockBudgetChecker{token: db.VerificationToken{MaxBudget: &maxBudget, Spend: 10.0}}
	mw := NewBudgetMiddleware(checker)
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	req := withTokenHash(httptest.NewRequest(http.MethodGet, "/", nil), "hash123")
	w := httptest.NewRecorder()
	mw(next).ServeHTTP(w, req)
	assert.Equal(t, http.StatusTooManyRequests, w.Code)
}

func TestNewBudgetMiddleware_Blocked(t *testing.T) {
	blocked := true
	checker := &mockBudgetChecker{token: db.VerificationToken{Blocked: &blocked}}
	mw := NewBudgetMiddleware(checker)
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	req := withTokenHash(httptest.NewRequest(http.MethodGet, "/", nil), "hash123")
	w := httptest.NewRecorder()
	mw(next).ServeHTTP(w, req)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestNewBudgetMiddleware_WithinBudget(t *testing.T) {
	maxBudget := 100.0
	checker := &mockBudgetChecker{token: db.VerificationToken{MaxBudget: &maxBudget, Spend: 5.0}}
	mw := NewBudgetMiddleware(checker)
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	req := withTokenHash(httptest.NewRequest(http.MethodGet, "/", nil), "hash123")
	w := httptest.NewRecorder()
	mw(next).ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}
