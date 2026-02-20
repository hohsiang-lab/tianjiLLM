package middleware

import (
	"context"
	"net/http"
	"sync"

	"github.com/praxisllmlab/tianjiLLM/internal/db"
	"github.com/praxisllmlab/tianjiLLM/internal/model"
)

// BudgetChecker checks if a key's spend exceeds its budget.
type BudgetChecker interface {
	GetVerificationToken(ctx context.Context, token string) (db.VerificationToken, error)
}

// NewBudgetMiddleware returns middleware that checks budget limits.
func NewBudgetMiddleware(checker BudgetChecker) func(http.Handler) http.Handler {
	if checker == nil {
		return func(next http.Handler) http.Handler { return next }
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenHash, ok := r.Context().Value(tokenHashKey).(string)
			if !ok || tokenHash == "" {
				next.ServeHTTP(w, r)
				return
			}

			token, err := checker.GetVerificationToken(r.Context(), tokenHash)
			if err != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusServiceUnavailable)
				writeJSONResponse(w, model.ErrorResponse{
					Error: model.ErrorDetail{
						Message: "unable to verify budget",
						Type:    "internal_error",
					},
				})
				return
			}

			// Check budget
			if token.MaxBudget != nil && token.Spend >= *token.MaxBudget {
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

			// Check if blocked
			if token.Blocked != nil && *token.Blocked {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				writeJSONResponse(w, model.ErrorResponse{
					Error: model.ErrorDetail{
						Message: "key is blocked",
						Type:    "permission_denied",
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
