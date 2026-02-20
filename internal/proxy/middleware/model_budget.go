package middleware

import (
	"encoding/json"
	"net/http"

	"github.com/praxisllmlab/tianjiLLM/internal/model"
)

// NewModelMaxBudgetMiddleware returns middleware that enforces per-model budget
// limits at the virtual key level. It reads model_spend and model_max_budget
// from the VerificationToken attached to the request context.
//
// This is different from the router-level ModelBudgetLimiter which tracks
// aggregate deployment spend. This middleware limits individual key spend
// per model.
func NewModelMaxBudgetMiddleware(checker BudgetChecker) func(http.Handler) http.Handler {
	if checker == nil {
		return func(next http.Handler) http.Handler { return next }
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenHash, _ := r.Context().Value(tokenHashKey).(string)
			requestModel, _ := r.Context().Value(modelGroupKey).(string)

			if tokenHash == "" || requestModel == "" {
				next.ServeHTTP(w, r)
				return
			}

			tok, err := checker.GetVerificationToken(r.Context(), tokenHash)
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}

			// Parse model_max_budget JSONB → map[string]float64
			if len(tok.ModelMaxBudget) == 0 {
				next.ServeHTTP(w, r)
				return
			}

			var maxBudgets map[string]float64
			if err := json.Unmarshal(tok.ModelMaxBudget, &maxBudgets); err != nil {
				next.ServeHTTP(w, r)
				return
			}

			limit, hasLimit := maxBudgets[requestModel]
			if !hasLimit || limit <= 0 {
				next.ServeHTTP(w, r)
				return
			}

			// Parse model_spend JSONB → map[string]float64
			var spends map[string]float64
			if len(tok.ModelSpend) > 0 {
				_ = json.Unmarshal(tok.ModelSpend, &spends)
			}

			currentSpend := 0.0
			if spends != nil {
				currentSpend = spends[requestModel]
			}

			if currentSpend >= limit {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				writeJSONResponse(w, model.ErrorResponse{
					Error: model.ErrorDetail{
						Message: "model budget exceeded for " + requestModel,
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
