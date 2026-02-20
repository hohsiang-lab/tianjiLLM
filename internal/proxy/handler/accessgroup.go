package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/praxisllmlab/tianjiLLM/internal/db"
	"github.com/praxisllmlab/tianjiLLM/internal/model"
)

// AccessGroupNew handles POST /model_access_group/new.
func (h *Handlers) AccessGroupNew(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	var req struct {
		GroupAlias     *string  `json:"group_alias"`
		Models         []string `json:"models"`
		OrganizationID *string  `json:"organization_id"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "invalid request: " + err.Error(), Type: "invalid_request_error"},
		})
		return
	}

	group, err := h.DB.CreateAccessGroup(r.Context(), db.CreateAccessGroupParams{
		GroupID:        uuid.New().String(),
		GroupAlias:     req.GroupAlias,
		Models:         req.Models,
		OrganizationID: req.OrganizationID,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "create access group: " + err.Error(), Type: "internal_error"},
		})
		return
	}

	writeJSON(w, http.StatusOK, group)
}

// AccessGroupInfo handles GET /model_access_group/info/{group_id}.
func (h *Handlers) AccessGroupInfo(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	groupID := chi.URLParam(r, "group_id")
	if groupID == "" {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "group_id required", Type: "invalid_request_error"},
		})
		return
	}

	group, err := h.DB.GetAccessGroup(r.Context(), groupID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "access group not found", Type: "invalid_request_error"},
		})
		return
	}

	writeJSON(w, http.StatusOK, group)
}

// AccessGroupUpdate handles POST /model_access_group/update.
func (h *Handlers) AccessGroupUpdate(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	var req struct {
		GroupID string   `json:"group_id"`
		Models  []string `json:"models"`
	}
	if err := decodeJSON(r, &req); err != nil || req.GroupID == "" {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "group_id required", Type: "invalid_request_error"},
		})
		return
	}

	if err := h.DB.UpdateAccessGroup(r.Context(), db.UpdateAccessGroupParams{
		GroupID: req.GroupID,
		Models:  req.Models,
	}); err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "update access group: " + err.Error(), Type: "internal_error"},
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "updated", "group_id": req.GroupID})
}

// AccessGroupDelete handles DELETE /model_access_group/delete/{group_id}.
func (h *Handlers) AccessGroupDelete(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	groupID := chi.URLParam(r, "group_id")
	if groupID == "" {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "group_id required", Type: "invalid_request_error"},
		})
		return
	}

	if err := h.DB.DeleteAccessGroup(r.Context(), groupID); err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "delete access group: " + err.Error(), Type: "internal_error"},
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted", "group_id": groupID})
}
