package handler

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/praxisllmlab/tianjiLLM/internal/db"
	"github.com/praxisllmlab/tianjiLLM/internal/model"
)

// TeamInfo handles GET /team/info/{team_id}.
func (h *Handlers) TeamInfo(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	teamID := chi.URLParam(r, "team_id")
	if teamID == "" {
		teamID = r.URL.Query().Get("team_id")
	}
	if teamID == "" {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "team_id required", Type: "invalid_request_error"},
		})
		return
	}

	team, err := h.DB.GetTeam(r.Context(), teamID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "team not found", Type: "not_found"},
		})
		return
	}

	writeJSON(w, http.StatusOK, team)
}

// TeamBlock handles POST /team/block.
func (h *Handlers) TeamBlock(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	var req struct {
		TeamID string `json:"team_id"`
	}
	if err := decodeJSON(r, &req); err != nil || req.TeamID == "" {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "team_id required", Type: "invalid_request_error"},
		})
		return
	}

	if err := h.DB.BlockTeam(r.Context(), req.TeamID); err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "block team: " + err.Error(), Type: "internal_error"},
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "blocked", "team_id": req.TeamID})
}

// TeamUnblock handles POST /team/unblock.
func (h *Handlers) TeamUnblock(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	var req struct {
		TeamID string `json:"team_id"`
	}
	if err := decodeJSON(r, &req); err != nil || req.TeamID == "" {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "team_id required", Type: "invalid_request_error"},
		})
		return
	}

	if err := h.DB.UnblockTeam(r.Context(), req.TeamID); err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "unblock team: " + err.Error(), Type: "internal_error"},
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "unblocked", "team_id": req.TeamID})
}

// TeamDailyActivity handles GET /team/daily_activity.
func (h *Handlers) TeamDailyActivity(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	teamID := r.URL.Query().Get("team_id")
	if teamID == "" {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "team_id required", Type: "invalid_request_error"},
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

	result, err := h.DB.GetTeamDailyActivity(r.Context(), db.GetTeamDailyActivityParams{
		TeamID:      &teamID,
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

// TeamModelAdd handles POST /team/model/add.
func (h *Handlers) TeamModelAdd(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	var req struct {
		TeamID string `json:"team_id"`
		Model  string `json:"model"`
	}
	if err := decodeJSON(r, &req); err != nil || req.TeamID == "" || req.Model == "" {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "team_id and model required", Type: "invalid_request_error"},
		})
		return
	}

	if err := h.DB.AddTeamModel(r.Context(), db.AddTeamModelParams{
		TeamID:      req.TeamID,
		ArrayAppend: req.Model,
	}); err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "add model: " + err.Error(), Type: "internal_error"},
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "added", "team_id": req.TeamID, "model": req.Model})
}

// TeamModelRemove handles POST /team/model/remove.
func (h *Handlers) TeamModelRemove(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	var req struct {
		TeamID string `json:"team_id"`
		Model  string `json:"model"`
	}
	if err := decodeJSON(r, &req); err != nil || req.TeamID == "" || req.Model == "" {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "team_id and model required", Type: "invalid_request_error"},
		})
		return
	}

	if err := h.DB.RemoveTeamModel(r.Context(), db.RemoveTeamModelParams{
		TeamID:      req.TeamID,
		ArrayRemove: req.Model,
	}); err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "remove model: " + err.Error(), Type: "internal_error"},
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "removed", "team_id": req.TeamID, "model": req.Model})
}

// TeamMemberUpdate handles POST /team/member_update.
func (h *Handlers) TeamMemberUpdate(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	var req struct {
		TeamID           string `json:"team_id"`
		MembersWithRoles []byte `json:"members_with_roles"`
	}
	if err := decodeJSON(r, &req); err != nil || req.TeamID == "" {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "team_id required", Type: "invalid_request_error"},
		})
		return
	}

	if err := h.DB.UpdateTeamMemberRole(r.Context(), db.UpdateTeamMemberRoleParams{
		TeamID:           req.TeamID,
		MembersWithRoles: req.MembersWithRoles,
	}); err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "update member: " + err.Error(), Type: "internal_error"},
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "updated", "team_id": req.TeamID})
}

// TeamAvailable handles GET /team/available.
func (h *Handlers) TeamAvailable(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	teams, err := h.DB.ListAvailableTeams(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "list available teams: " + err.Error(), Type: "internal_error"},
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"teams": teams})
}

// TeamPermissionsList handles GET /team/permissions.
func (h *Handlers) TeamPermissionsList(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	teamID := r.URL.Query().Get("team_id")
	if teamID == "" {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "team_id required", Type: "invalid_request_error"},
		})
		return
	}

	metadata, err := h.DB.GetTeamPermissions(r.Context(), teamID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "team not found", Type: "not_found"},
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"team_id": teamID, "metadata": metadata})
}

// TeamPermissionsUpdate handles POST /team/permissions.
func (h *Handlers) TeamPermissionsUpdate(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	var req struct {
		TeamID      string `json:"team_id"`
		Permissions []byte `json:"permissions"`
	}
	if err := decodeJSON(r, &req); err != nil || req.TeamID == "" {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "team_id required", Type: "invalid_request_error"},
		})
		return
	}

	if err := h.DB.SetTeamPermissions(r.Context(), db.SetTeamPermissionsParams{
		TeamID:  req.TeamID,
		Column2: req.Permissions,
	}); err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "set permissions: " + err.Error(), Type: "internal_error"},
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "updated", "team_id": req.TeamID})
}

// TeamCallbackSet handles POST /team/callback.
func (h *Handlers) TeamCallbackSet(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	var req struct {
		TeamID           string `json:"team_id"`
		CallbackSettings []byte `json:"callback_settings"`
	}
	if err := decodeJSON(r, &req); err != nil || req.TeamID == "" {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "team_id required", Type: "invalid_request_error"},
		})
		return
	}

	if err := h.DB.SetTeamCallback(r.Context(), db.SetTeamCallbackParams{
		TeamID:  req.TeamID,
		Column2: req.CallbackSettings,
	}); err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "set callback: " + err.Error(), Type: "internal_error"},
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "updated", "team_id": req.TeamID})
}

// TeamCallbackGet handles GET /team/callback.
func (h *Handlers) TeamCallbackGet(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	teamID := r.URL.Query().Get("team_id")
	if teamID == "" {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "team_id required", Type: "invalid_request_error"},
		})
		return
	}

	settings, err := h.DB.GetTeamCallback(r.Context(), teamID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "team not found", Type: "not_found"},
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"team_id": teamID, "callback_settings": settings})
}

// ResetTeamSpend handles POST /team/{team_id}/reset_spend.
func (h *Handlers) ResetTeamSpend(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	teamID := chi.URLParam(r, "team_id")
	if teamID == "" {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "team_id required", Type: "invalid_request_error"},
		})
		return
	}

	if err := h.DB.ResetTeamSpend(r.Context(), teamID); err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "reset spend: " + err.Error(), Type: "internal_error"},
		})
		return
	}

	h.createAuditLog(r.Context(), "reset_spend", "TeamTable", teamID, "", "", nil, map[string]any{"spend": 0})
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "team_id": teamID})
}
