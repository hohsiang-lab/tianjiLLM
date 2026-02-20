package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/praxisllmlab/tianjiLLM/internal/db"
	"github.com/praxisllmlab/tianjiLLM/internal/model"
)

// OrgNew handles POST /organization/new.
func (h *Handlers) OrgNew(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	var req struct {
		OrganizationAlias *string  `json:"organization_alias"`
		MaxBudget         *float64 `json:"max_budget"`
		Models            []string `json:"models"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "invalid request: " + err.Error(), Type: "invalid_request_error"},
		})
		return
	}

	org, err := h.DB.CreateOrganization(r.Context(), db.CreateOrganizationParams{
		OrganizationID:    uuid.New().String(),
		OrganizationAlias: req.OrganizationAlias,
		MaxBudget:         req.MaxBudget,
		Models:            req.Models,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "create organization: " + err.Error(), Type: "internal_error"},
		})
		return
	}

	writeJSON(w, http.StatusOK, org)
}

// OrgInfo handles GET /organization/info?organization_id=...
func (h *Handlers) OrgInfo(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	orgID := r.URL.Query().Get("organization_id")
	if orgID == "" {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "organization_id required", Type: "invalid_request_error"},
		})
		return
	}

	org, err := h.DB.GetOrganization(r.Context(), orgID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "organization not found", Type: "invalid_request_error"},
		})
		return
	}

	writeJSON(w, http.StatusOK, org)
}

// OrgUpdate handles POST /organization/update.
func (h *Handlers) OrgUpdate(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	var req struct {
		OrganizationID    string   `json:"organization_id"`
		OrganizationAlias *string  `json:"organization_alias"`
		MaxBudget         *float64 `json:"max_budget"`
	}
	if err := decodeJSON(r, &req); err != nil || req.OrganizationID == "" {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "organization_id required", Type: "invalid_request_error"},
		})
		return
	}

	if _, err := h.DB.UpdateOrganization(r.Context(), db.UpdateOrganizationParams{
		OrganizationID:    req.OrganizationID,
		OrganizationAlias: req.OrganizationAlias,
		MaxBudget:         req.MaxBudget,
	}); err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "update organization: " + err.Error(), Type: "internal_error"},
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "updated", "organization_id": req.OrganizationID})
}

// OrgDelete handles DELETE /organization/delete/{organization_id}.
func (h *Handlers) OrgDelete(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	orgID := chi.URLParam(r, "organization_id")
	if orgID == "" {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "organization_id required", Type: "invalid_request_error"},
		})
		return
	}

	if err := h.DB.DeleteOrganization(r.Context(), orgID); err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "delete organization: " + err.Error(), Type: "internal_error"},
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted", "organization_id": orgID})
}

// OrgMemberAdd handles POST /organization/member_add.
func (h *Handlers) OrgMemberAdd(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	var req struct {
		UserID         string  `json:"user_id"`
		OrganizationID string  `json:"organization_id"`
		UserRole       *string `json:"user_role"`
		BudgetID       *string `json:"budget_id"`
	}
	if err := decodeJSON(r, &req); err != nil || req.UserID == "" || req.OrganizationID == "" {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "user_id and organization_id required", Type: "invalid_request_error"},
		})
		return
	}

	member, err := h.DB.AddOrgMember(r.Context(), db.AddOrgMemberParams{
		UserID:         req.UserID,
		OrganizationID: req.OrganizationID,
		UserRole:       req.UserRole,
		BudgetID:       req.BudgetID,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "add org member: " + err.Error(), Type: "internal_error"},
		})
		return
	}

	writeJSON(w, http.StatusOK, member)
}

// OrgMemberUpdate handles PATCH /organization/member_update.
func (h *Handlers) OrgMemberUpdate(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	var req struct {
		UserID         string  `json:"user_id"`
		OrganizationID string  `json:"organization_id"`
		UserRole       *string `json:"user_role"`
		BudgetID       *string `json:"budget_id"`
	}
	if err := decodeJSON(r, &req); err != nil || req.UserID == "" || req.OrganizationID == "" {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "user_id and organization_id required", Type: "invalid_request_error"},
		})
		return
	}

	member, err := h.DB.UpdateOrgMember(r.Context(), db.UpdateOrgMemberParams{
		UserID:         req.UserID,
		OrganizationID: req.OrganizationID,
		UserRole:       req.UserRole,
		BudgetID:       req.BudgetID,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "update org member: " + err.Error(), Type: "internal_error"},
		})
		return
	}

	writeJSON(w, http.StatusOK, member)
}

// OrgMemberDelete handles DELETE /organization/member_delete.
func (h *Handlers) OrgMemberDelete(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	var req struct {
		UserID         string `json:"user_id"`
		OrganizationID string `json:"organization_id"`
	}
	if err := decodeJSON(r, &req); err != nil || req.UserID == "" || req.OrganizationID == "" {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "user_id and organization_id required", Type: "invalid_request_error"},
		})
		return
	}

	if err := h.DB.DeleteOrgMember(r.Context(), db.DeleteOrgMemberParams{
		UserID:         req.UserID,
		OrganizationID: req.OrganizationID,
	}); err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "delete org member: " + err.Error(), Type: "internal_error"},
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status":          "deleted",
		"user_id":         req.UserID,
		"organization_id": req.OrganizationID,
	})
}
