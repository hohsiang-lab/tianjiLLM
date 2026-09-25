package handler

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/praxisllmlab/tianjiLLM/internal/db"
	"github.com/praxisllmlab/tianjiLLM/internal/model"
)

// UserInfo handles GET /user/info/{user_id}.
func (h *Handlers) UserInfo(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	userID := chi.URLParam(r, "user_id")
	if userID == "" {
		userID = r.URL.Query().Get("user_id")
	}
	if userID == "" {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "user_id required", Type: "invalid_request_error"},
		})
		return
	}

	user, err := h.DB.GetUser(r.Context(), userID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "user not found", Type: "not_found"},
		})
		return
	}

	writeJSON(w, http.StatusOK, user)
}

// UserUpdate handles POST /user/update.
func (h *Handlers) UserUpdate(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	var req struct {
		UserID    string   `json:"user_id"`
		UserAlias *string  `json:"user_alias"`
		UserEmail *string  `json:"user_email"`
		UserRole  string   `json:"user_role"`
		MaxBudget *float64 `json:"max_budget"`
		Models    []string `json:"models"`
		TPMLimit  *int64   `json:"tpm_limit"`
		RPMLimit  *int64   `json:"rpm_limit"`
	}
	if err := decodeJSON(r, &req); err != nil || req.UserID == "" {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "user_id required", Type: "invalid_request_error"},
		})
		return
	}

	result, err := h.DB.UpdateUser(r.Context(), db.UpdateUserParams{
		UserID:    req.UserID,
		UserAlias: req.UserAlias,
		UserEmail: req.UserEmail,
		UserRole:  req.UserRole,
		MaxBudget: req.MaxBudget,
		Models:    req.Models,
		TpmLimit:  req.TPMLimit,
		RpmLimit:  req.RPMLimit,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "update user: " + err.Error(), Type: "internal_error"},
		})
		return
	}

	h.createAuditLog(r.Context(), "updated", "UserTable", req.UserID, "", "", nil, req)
	writeJSON(w, http.StatusOK, result)
}

// UserDailyActivity handles GET /user/daily_activity.
func (h *Handlers) UserDailyActivity(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "user_id required", Type: "invalid_request_error"},
		})
		return
	}

	startDate := time.Now().AddDate(0, 0, -30)
	endDate := time.Now()

	if s := r.URL.Query().Get("start_date"); s != "" {
		if t, err := time.Parse("2006-01-02", s); err == nil {
			startDate = t
		}
	}
	if s := r.URL.Query().Get("end_date"); s != "" {
		if t, err := time.Parse("2006-01-02", s); err == nil {
			endDate = t
		}
	}

	result, err := h.DB.GetUserDailyActivity(r.Context(), db.GetUserDailyActivityParams{
		User:        userID,
		Starttime:   pgtype.Timestamptz{Time: startDate, Valid: true},
		Starttime_2: pgtype.Timestamptz{Time: endDate, Valid: true},
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "daily activity: " + err.Error(), Type: "internal_error"},
		})
		return
	}

	writeJSON(w, http.StatusOK, result)
}
