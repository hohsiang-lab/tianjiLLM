package handler

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/praxisllmlab/tianjiLLM/internal/db"
	"github.com/praxisllmlab/tianjiLLM/internal/model"
)

type userCreateRequest struct {
	UserAlias *string  `json:"user_alias"`
	UserEmail *string  `json:"user_email"`
	UserRole  string   `json:"user_role"`
	MaxBudget *float64 `json:"max_budget"`
	Models    []string `json:"models"`
	TPMLimit  *int64   `json:"tpm_limit"`
	RPMLimit  *int64   `json:"rpm_limit"`
}

// UserNew handles POST /user/new
func (h *Handlers) UserNew(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	var req userCreateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "invalid request: " + err.Error(), Type: "invalid_request_error"},
		})
		return
	}

	user, err := h.DB.CreateUser(r.Context(), db.CreateUserParams{
		UserID:    uuid.New().String(),
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
			Error: model.ErrorDetail{Message: "create user: " + err.Error(), Type: "internal_error"},
		})
		return
	}

	h.createAuditLog(r.Context(), "created", "UserTable", user.UserID, "", "", nil, req)
	h.dispatchEvent(r.Context(), "user_created", user.UserID, req)
	writeJSON(w, http.StatusOK, user)
}

// UserList handles GET /user/list
func (h *Handlers) UserList(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	users, err := h.DB.ListUsers(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "list users: " + err.Error(), Type: "internal_error"},
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"users": users})
}

// UserDelete handles POST /user/delete
func (h *Handlers) UserDelete(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	var req struct {
		UserIDs []string `json:"user_ids"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "invalid request: " + err.Error(), Type: "invalid_request_error"},
		})
		return
	}

	for _, id := range req.UserIDs {
		if err := h.DB.DeleteUser(r.Context(), id); err != nil {
			writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
				Error: model.ErrorDetail{Message: "delete user: " + err.Error(), Type: "internal_error"},
			})
			return
		}
	}

	for _, id := range req.UserIDs {
		h.createAuditLog(r.Context(), "deleted", "UserTable", id, "", "", nil, nil)
		h.dispatchEvent(r.Context(), "user_deleted", id, nil)
	}
	writeJSON(w, http.StatusOK, map[string]any{"deleted_users": req.UserIDs})
}
