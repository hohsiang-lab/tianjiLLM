package handler

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/praxisllmlab/tianjiLLM/internal/db"
	"github.com/praxisllmlab/tianjiLLM/internal/model"
)

type teamCreateRequest struct {
	TeamAlias *string  `json:"team_alias"`
	MaxBudget *float64 `json:"max_budget"`
	Models    []string `json:"models"`
	TPMLimit  *int64   `json:"tpm_limit"`
	RPMLimit  *int64   `json:"rpm_limit"`
}

// TeamNew handles POST /team/new
func (h *Handlers) TeamNew(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	var req teamCreateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "invalid request: " + err.Error(), Type: "invalid_request_error"},
		})
		return
	}

	team, err := h.DB.CreateTeam(r.Context(), db.CreateTeamParams{
		TeamID:    uuid.New().String(),
		TeamAlias: req.TeamAlias,
		MaxBudget: req.MaxBudget,
		Models:    req.Models,
		TpmLimit:  req.TPMLimit,
		RpmLimit:  req.RPMLimit,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "create team: " + err.Error(), Type: "internal_error"},
		})
		return
	}

	h.createAuditLog(r.Context(), "created", "TeamTable", team.TeamID, "", "", nil, req)
	h.dispatchEvent(r.Context(), "team_created", team.TeamID, req)
	writeJSON(w, http.StatusOK, team)
}

// TeamList handles GET /team/list
func (h *Handlers) TeamList(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	teams, err := h.DB.ListTeams(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "list teams: " + err.Error(), Type: "internal_error"},
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"teams": teams})
}

// TeamDelete handles POST /team/delete
func (h *Handlers) TeamDelete(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	var req struct {
		TeamIDs []string `json:"team_ids"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "invalid request: " + err.Error(), Type: "invalid_request_error"},
		})
		return
	}

	for _, id := range req.TeamIDs {
		if err := h.DB.DeleteTeam(r.Context(), id); err != nil {
			writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
				Error: model.ErrorDetail{Message: "delete team: " + err.Error(), Type: "internal_error"},
			})
			return
		}
	}

	for _, id := range req.TeamIDs {
		h.createAuditLog(r.Context(), "deleted", "TeamTable", id, "", "", nil, nil)
		h.dispatchEvent(r.Context(), "team_deleted", id, nil)
	}
	writeJSON(w, http.StatusOK, map[string]any{"deleted_teams": req.TeamIDs})
}

// TeamUpdate handles POST /team/update.
func (h *Handlers) TeamUpdate(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	var req struct {
		TeamID    string   `json:"team_id"`
		TeamAlias *string  `json:"team_alias"`
		MaxBudget *float64 `json:"max_budget"`
		Models    []string `json:"models"`
		TPMLimit  *int64   `json:"tpm_limit"`
		RPMLimit  *int64   `json:"rpm_limit"`
	}
	if err := decodeJSON(r, &req); err != nil || req.TeamID == "" {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "team_id required", Type: "invalid_request_error"},
		})
		return
	}

	if _, err := h.DB.UpdateTeam(r.Context(), db.UpdateTeamParams{
		TeamID:    req.TeamID,
		TeamAlias: req.TeamAlias,
		MaxBudget: req.MaxBudget,
		Models:    req.Models,
		TpmLimit:  req.TPMLimit,
		RpmLimit:  req.RPMLimit,
	}); err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "update team: " + err.Error(), Type: "internal_error"},
		})
		return
	}

	h.createAuditLog(r.Context(), "updated", "TeamTable", req.TeamID, "", "", nil, req)
	h.dispatchEvent(r.Context(), "team_updated", req.TeamID, req)
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated", "team_id": req.TeamID})
}

// TeamMemberAdd handles POST /team/member/add.
func (h *Handlers) TeamMemberAdd(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	var req struct {
		TeamID string `json:"team_id"`
		UserID string `json:"user_id"`
	}
	if err := decodeJSON(r, &req); err != nil || req.TeamID == "" || req.UserID == "" {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "team_id and user_id required", Type: "invalid_request_error"},
		})
		return
	}

	if err := h.DB.AddTeamMember(r.Context(), db.AddTeamMemberParams{
		TeamID:      req.TeamID,
		ArrayAppend: req.UserID,
	}); err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "add member: " + err.Error(), Type: "internal_error"},
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "added", "team_id": req.TeamID, "user_id": req.UserID})
}

// TeamMemberDelete handles POST /team/member/delete.
func (h *Handlers) TeamMemberDelete(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	var req struct {
		TeamID string `json:"team_id"`
		UserID string `json:"user_id"`
	}
	if err := decodeJSON(r, &req); err != nil || req.TeamID == "" || req.UserID == "" {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "team_id and user_id required", Type: "invalid_request_error"},
		})
		return
	}

	if err := h.DB.RemoveTeamMember(r.Context(), db.RemoveTeamMemberParams{
		TeamID:      req.TeamID,
		ArrayRemove: req.UserID,
	}); err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "remove member: " + err.Error(), Type: "internal_error"},
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "removed", "team_id": req.TeamID, "user_id": req.UserID})
}
