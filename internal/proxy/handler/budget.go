package handler

import (
	"net/http"

	"github.com/google/uuid"

	"github.com/praxisllmlab/tianjiLLM/internal/db"
	"github.com/praxisllmlab/tianjiLLM/internal/model"
)

// BudgetNew handles POST /budget/new.
func (h *Handlers) BudgetNew(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	var req struct {
		MaxBudget           *float64 `json:"max_budget"`
		SoftBudget          *float64 `json:"soft_budget"`
		MaxParallelRequests *int32   `json:"max_parallel_requests"`
		TPMLimit            *int64   `json:"tpm_limit"`
		RPMLimit            *int64   `json:"rpm_limit"`
		BudgetDuration      *string  `json:"budget_duration"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "invalid request: " + err.Error(), Type: "invalid_request_error"},
		})
		return
	}

	budget, err := h.DB.CreateBudget(r.Context(), db.CreateBudgetParams{
		BudgetID:            uuid.New().String(),
		MaxBudget:           req.MaxBudget,
		SoftBudget:          req.SoftBudget,
		MaxParallelRequests: req.MaxParallelRequests,
		TpmLimit:            req.TPMLimit,
		RpmLimit:            req.RPMLimit,
		BudgetDuration:      req.BudgetDuration,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "create budget: " + err.Error(), Type: "internal_error"},
		})
		return
	}

	writeJSON(w, http.StatusOK, budget)
}

// BudgetInfo handles GET|POST /budget/info.
// Accepts budget_id as query param (GET) or JSON body field (POST).
func (h *Handlers) BudgetInfo(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	budgetID := r.URL.Query().Get("budget_id")
	if budgetID == "" && r.Method == "POST" {
		var req struct {
			BudgetID string `json:"budget_id"`
		}
		if err := decodeJSON(r, &req); err == nil {
			budgetID = req.BudgetID
		}
	}
	if budgetID == "" {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "budget_id required", Type: "invalid_request_error"},
		})
		return
	}

	budget, err := h.DB.GetBudget(r.Context(), budgetID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "budget not found", Type: "invalid_request_error"},
		})
		return
	}

	writeJSON(w, http.StatusOK, budget)
}

// BudgetUpdate handles POST /budget/update.
func (h *Handlers) BudgetUpdate(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	var req struct {
		BudgetID   string   `json:"budget_id"`
		MaxBudget  *float64 `json:"max_budget"`
		SoftBudget *float64 `json:"soft_budget"`
		TPMLimit   *int64   `json:"tpm_limit"`
		RPMLimit   *int64   `json:"rpm_limit"`
	}
	if err := decodeJSON(r, &req); err != nil || req.BudgetID == "" {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "budget_id required", Type: "invalid_request_error"},
		})
		return
	}

	budget, err := h.DB.UpdateBudget(r.Context(), db.UpdateBudgetParams{
		BudgetID:   req.BudgetID,
		MaxBudget:  req.MaxBudget,
		SoftBudget: req.SoftBudget,
		TpmLimit:   req.TPMLimit,
		RpmLimit:   req.RPMLimit,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "update budget: " + err.Error(), Type: "internal_error"},
		})
		return
	}

	writeJSON(w, http.StatusOK, budget)
}

// BudgetList handles GET /budget/list.
func (h *Handlers) BudgetList(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	budgets, err := h.DB.ListBudgets(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "list budgets: " + err.Error(), Type: "internal_error"},
		})
		return
	}

	writeJSON(w, http.StatusOK, budgets)
}

// BudgetDelete handles POST /budget/delete.
func (h *Handlers) BudgetDelete(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	var req struct {
		BudgetID string `json:"budget_id"`
	}
	if err := decodeJSON(r, &req); err != nil || req.BudgetID == "" {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "budget_id required", Type: "invalid_request_error"},
		})
		return
	}

	if err := h.DB.DeleteBudget(r.Context(), req.BudgetID); err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "delete budget: " + err.Error(), Type: "internal_error"},
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted", "budget_id": req.BudgetID})
}

// BudgetSettings handles GET /budget/settings.
// Returns budget configuration defaults.
func (h *Handlers) BudgetSettings(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"budget_duration_options": []string{"daily", "weekly", "monthly"},
	})
}
