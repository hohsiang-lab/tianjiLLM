package ui

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/praxisllmlab/tianjiLLM/internal/db"
	"github.com/praxisllmlab/tianjiLLM/internal/ui/components/toast"
	"github.com/praxisllmlab/tianjiLLM/internal/ui/pages"
)

// loadTeamDetailData builds TeamDetailData from the DB for the given teamID.
func (h *UIHandler) loadTeamDetailData(r *http.Request, teamID string) (pages.TeamDetailData, bool) {
	if h.DB == nil {
		return pages.TeamDetailData{}, false
	}

	ctx := r.Context()

	t, err := h.DB.GetTeam(ctx, teamID)
	if err != nil {
		return pages.TeamDetailData{}, false
	}

	alias := ""
	if t.TeamAlias != nil {
		alias = *t.TeamAlias
	}
	orgID := ""
	if t.OrganizationID != nil {
		orgID = *t.OrganizationID
	}

	// Look up org alias
	orgAlias := ""
	if orgID != "" {
		if org, err := h.DB.GetOrganization(ctx, orgID); err == nil {
			if org.OrganizationAlias != nil {
				orgAlias = *org.OrganizationAlias
			} else {
				orgAlias = org.OrganizationID
			}
		}
	}

	// Parse members_with_roles JSONB
	members, _ := parseMembersWithRoles(t.MembersWithRoles)
	memberRows := make([]pages.TeamMemberRow, 0, len(members))
	for _, m := range members {
		memberRows = append(memberRows, pages.TeamMemberRow{
			UserID: m.UserID,
			Role:   m.Role,
		})
	}

	// Pretty-print metadata
	metadataStr := "{}"
	if len(t.Metadata) > 0 {
		var v any
		if json.Unmarshal(t.Metadata, &v) == nil {
			b, _ := json.MarshalIndent(v, "", "  ")
			metadataStr = string(b)
		} else {
			metadataStr = string(t.Metadata)
		}
	}

	row := pages.TeamRow{
		TeamID:    t.TeamID,
		TeamAlias: alias,
		OrgID:     orgID,
		OrgAlias:  orgAlias,
		Spend:     t.Spend,
		MaxBudget: t.MaxBudget,
		Models:    t.Models,
		TPMLimit:  t.TpmLimit,
		RPMLimit:  t.RpmLimit,
		Blocked:   t.Blocked,
	}
	if t.CreatedAt.Valid {
		row.CreatedAt = t.CreatedAt.Time
	}

	return pages.TeamDetailData{
		Team:             row,
		MembersWithRoles: memberRows,
		AvailableModels:  h.loadAvailableModelNames(ctx),
		Metadata:         metadataStr,
		OrgAlias:         orgAlias,
	}, true
}

func (h *UIHandler) handleTeamDetail(w http.ResponseWriter, r *http.Request) {
	teamID := chi.URLParam(r, "team_id")

	data, ok := h.loadTeamDetailData(r, teamID)
	if !ok {
		http.Redirect(w, r, "/ui/teams", http.StatusSeeOther)
		return
	}

	render(r.Context(), w, pages.TeamDetailPage(data))
}

func (h *UIHandler) handleTeamUpdate(w http.ResponseWriter, r *http.Request) {
	teamID := chi.URLParam(r, "team_id")

	if h.DB == nil {
		http.Error(w, "database not configured", http.StatusServiceUnavailable)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	teamAlias := strings.TrimSpace(r.FormValue("team_alias"))
	var teamAliasPtr *string
	if teamAlias != "" {
		teamAliasPtr = &teamAlias
	}

	maxBudget := parseOptionalFloat(r.FormValue("max_budget"))
	tpmLimit := parseOptionalInt64(r.FormValue("tpm_limit"))
	rpmLimit := parseOptionalInt64(r.FormValue("rpm_limit"))

	// Fetch current team to preserve blocked state
	current, err := h.DB.GetTeam(r.Context(), teamID)
	if err != nil {
		http.Error(w, "team not found", http.StatusNotFound)
		return
	}

	params := db.UpdateTeamParams{
		TeamID:    teamID,
		TeamAlias: teamAliasPtr,
		MaxBudget: maxBudget,
		Models:    current.Models, // preserve existing models list
		Blocked:   current.Blocked,
		TpmLimit:  tpmLimit,
		RpmLimit:  rpmLimit,
		UpdatedBy: "admin",
	}

	if _, err := h.DB.UpdateTeam(r.Context(), params); err != nil {
		data, ok := h.loadTeamDetailData(r, teamID)
		if !ok {
			http.Error(w, "team not found", http.StatusNotFound)
			return
		}
		render(r.Context(), w, pages.TeamDetailHeaderWithToast(data, "Failed to update team: "+err.Error(), toast.VariantError))
		return
	}

	data, ok := h.loadTeamDetailData(r, teamID)
	if !ok {
		http.Error(w, "team not found", http.StatusNotFound)
		return
	}
	render(r.Context(), w, pages.TeamDetailHeaderWithToast(data, "Team updated successfully", toast.VariantSuccess))
}

func (h *UIHandler) handleTeamMemberAdd(w http.ResponseWriter, r *http.Request) {
	teamID := chi.URLParam(r, "team_id")

	if h.DB == nil {
		http.Error(w, "database not configured", http.StatusServiceUnavailable)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	userID := strings.TrimSpace(r.FormValue("user_id"))
	if userID == "" {
		data, ok := h.loadTeamDetailData(r, teamID)
		if !ok {
			http.Error(w, "team not found", http.StatusNotFound)
			return
		}
		renderMembersWithToast(w, r, data, "User ID is required", toast.VariantError)
		return
	}

	role := r.FormValue("role")
	if role == "" {
		role = "member"
	}

	ctx := r.Context()

	// Check if already a member
	current, err := h.DB.GetTeam(ctx, teamID)
	if err != nil {
		http.Error(w, "team not found", http.StatusNotFound)
		return
	}

	existingMembers, _ := parseMembersWithRoles(current.MembersWithRoles)
	for _, m := range existingMembers {
		if m.UserID == userID {
			data, _ := h.loadTeamDetailData(r, teamID)
			renderMembersWithToast(w, r, data, "User is already a member", toast.VariantError)
			return
		}
	}

	// Add to members array
	if err := h.DB.AddTeamMember(ctx, db.AddTeamMemberParams{
		TeamID:      teamID,
		ArrayAppend: userID,
	}); err != nil {
		data, _ := h.loadTeamDetailData(r, teamID)
		renderMembersWithToast(w, r, data, "Failed to add member: "+err.Error(), toast.VariantError)
		return
	}

	// Update members_with_roles JSONB
	updatedMembers := append(existingMembers, memberWithRole{UserID: userID, Role: role})
	membersJSON, _ := json.Marshal(updatedMembers)
	_ = h.DB.UpdateTeamMemberRole(ctx, db.UpdateTeamMemberRoleParams{
		TeamID:           teamID,
		MembersWithRoles: membersJSON,
	})

	data, _ := h.loadTeamDetailData(r, teamID)
	renderMembersWithToast(w, r, data, "Member added successfully", toast.VariantSuccess)
}

func (h *UIHandler) handleTeamMemberRemove(w http.ResponseWriter, r *http.Request) {
	teamID := chi.URLParam(r, "team_id")

	if h.DB == nil {
		http.Error(w, "database not configured", http.StatusServiceUnavailable)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	userID := strings.TrimSpace(r.FormValue("user_id"))
	if userID == "" {
		http.Error(w, "user_id required", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	if err := h.DB.RemoveTeamMember(ctx, db.RemoveTeamMemberParams{
		TeamID:      teamID,
		ArrayRemove: userID,
	}); err != nil {
		data, _ := h.loadTeamDetailData(r, teamID)
		renderMembersWithToast(w, r, data, "Failed to remove member: "+err.Error(), toast.VariantError)
		return
	}

	// Update members_with_roles JSONB: remove this user
	current, err := h.DB.GetTeam(ctx, teamID)
	if err == nil {
		existingMembers, _ := parseMembersWithRoles(current.MembersWithRoles)
		updatedMembers := make([]memberWithRole, 0, len(existingMembers))
		for _, m := range existingMembers {
			if m.UserID != userID {
				updatedMembers = append(updatedMembers, m)
			}
		}
		membersJSON, _ := json.Marshal(updatedMembers)
		_ = h.DB.UpdateTeamMemberRole(ctx, db.UpdateTeamMemberRoleParams{
			TeamID:           teamID,
			MembersWithRoles: membersJSON,
		})
	}

	data, _ := h.loadTeamDetailData(r, teamID)
	renderMembersWithToast(w, r, data, "Member removed successfully", toast.VariantSuccess)
}

func (h *UIHandler) handleTeamModelAdd(w http.ResponseWriter, r *http.Request) {
	teamID := chi.URLParam(r, "team_id")

	if h.DB == nil {
		http.Error(w, "database not configured", http.StatusServiceUnavailable)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	modelName := strings.TrimSpace(r.FormValue("model_name"))
	if modelName == "" {
		data, _ := h.loadTeamDetailData(r, teamID)
		renderModelsWithToast(w, r, data, "Model name is required", toast.VariantError)
		return
	}

	if err := h.DB.AddTeamModel(r.Context(), db.AddTeamModelParams{
		TeamID:      teamID,
		ArrayAppend: modelName,
	}); err != nil {
		data, _ := h.loadTeamDetailData(r, teamID)
		renderModelsWithToast(w, r, data, "Failed to add model: "+err.Error(), toast.VariantError)
		return
	}

	data, _ := h.loadTeamDetailData(r, teamID)
	renderModelsWithToast(w, r, data, "Model added successfully", toast.VariantSuccess)
}

func (h *UIHandler) handleTeamModelRemove(w http.ResponseWriter, r *http.Request) {
	teamID := chi.URLParam(r, "team_id")

	if h.DB == nil {
		http.Error(w, "database not configured", http.StatusServiceUnavailable)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	modelName := strings.TrimSpace(r.FormValue("model_name"))
	if modelName == "" {
		http.Error(w, "model_name required", http.StatusBadRequest)
		return
	}

	if err := h.DB.RemoveTeamModel(r.Context(), db.RemoveTeamModelParams{
		TeamID:      teamID,
		ArrayRemove: modelName,
	}); err != nil {
		data, _ := h.loadTeamDetailData(r, teamID)
		renderModelsWithToast(w, r, data, "Failed to remove model: "+err.Error(), toast.VariantError)
		return
	}

	data, _ := h.loadTeamDetailData(r, teamID)
	renderModelsWithToast(w, r, data, "Model removed successfully", toast.VariantSuccess)
}

// renderMembersWithToast renders the TeamMembersTablePartial with an OOB toast.
func renderMembersWithToast(w http.ResponseWriter, r *http.Request, data pages.TeamDetailData, msg string, variant toast.Variant) {
	render(r.Context(), w, pages.TeamMembersWithToast(data, msg, variant))
}

// renderModelsWithToast renders the TeamModelsListPartial with an OOB toast.
func renderModelsWithToast(w http.ResponseWriter, r *http.Request, data pages.TeamDetailData, msg string, variant toast.Variant) {
	render(r.Context(), w, pages.TeamModelsWithToast(data, msg, variant))
}
