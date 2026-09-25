package middleware

import (
	"net/http"
	"sync"

	"github.com/praxisllmlab/tianjiLLM/internal/model"
)

// NewBudgetMiddleware returns middleware that checks budget limits.
// Reads max_budget and spend from request context (injected by auth middleware).
func NewBudgetMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			maxBudget, hasBudget := r.Context().Value(maxBudgetKey).(float64)
			spend, _ := r.Context().Value(spendKey).(float64)

			if hasBudget && spend >= maxBudget {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				writeJSONResponse(w, model.ErrorResponse{
					Error: model.ErrorDetail{
						Message: "budget exceeded",
						Type:    "budget_exceeded",
						Code:    "budget_exceeded",
					},
				})
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// ModelBudgetLimiter tracks cumulative spend per model and enforces per-model budget limits.
type ModelBudgetLimiter struct {
	mu     sync.Mutex
	spend  map[string]float64 // model name → cumulative spend
	limits map[string]float64 // model name → max budget
}

// NewModelBudgetLimiter creates a per-model budget limiter with given limits.
func NewModelBudgetLimiter(limits map[string]float64) *ModelBudgetLimiter {
	return &ModelBudgetLimiter{
		spend:  make(map[string]float64),
		limits: limits,
	}
}

// RecordSpend adds cost to the cumulative spend for a model.
func (m *ModelBudgetLimiter) RecordSpend(modelName string, cost float64) {
	m.mu.Lock()
	m.spend[modelName] += cost
	m.mu.Unlock()
}

// Check returns model.ErrBudgetExceeded if the model's cumulative spend exceeds its budget limit.
func (m *ModelBudgetLimiter) Check(modelName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	limit, ok := m.limits[modelName]
	if !ok || limit <= 0 {
		return nil
	}
	if m.spend[modelName] >= limit {
		return model.ErrBudgetExceeded
	}
	return nil
}

// GetSpend returns the current cumulative spend for a model.
func (m *ModelBudgetLimiter) GetSpend(modelName string) float64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.spend[modelName]
}

// ResetSpend resets all model spend counters (e.g., for monthly reset).
func (m *ModelBudgetLimiter) ResetSpend() {
	m.mu.Lock()
	m.spend = make(map[string]float64)
	m.mu.Unlock()
}
