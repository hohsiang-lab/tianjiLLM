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

// loadOrgDetailData builds OrgDetailData from the DB for the given orgID.
func (h *UIHandler) loadOrgDetailData(r *http.Request, orgID string) (pages.OrgDetailData, bool) {
	if h.DB == nil {
		return pages.OrgDetailData{}, false
	}

	ctx := r.Context()

	o, err := h.DB.GetOrganization(ctx, orgID)
	if err != nil {
		return pages.OrgDetailData{}, false
	}

	alias := ""
	if o.OrganizationAlias != nil {
		alias = *o.OrganizationAlias
	}

	models := o.Models
	if models == nil {
		models = []string{}
	}

	orgRow := pages.OrgRow{
		OrgID:      o.OrganizationID,
		OrgAlias:   alias,
		Spend:      o.Spend,
		MaxBudget:  o.MaxBudget,
		ModelCount: len(models),
		Models:     models,
		TpmLimit:   o.TpmLimit,
		RpmLimit:   o.RpmLimit,
	}
	if o.CreatedAt.Valid {
		orgRow.CreatedAt = o.CreatedAt.Time
	}

	// Load members
	memberships, _ := h.DB.ListOrgMembers(ctx, orgID)
	memberRows := make([]pages.OrgMemberRow, 0, len(memberships))
	for _, m := range memberships {
		role := ""
		if m.UserRole != nil {
			role = *m.UserRole
		}
		row := pages.OrgMemberRow{
			UserID: m.UserID,
			Role:   role,
			Spend:  m.Spend,
		}
		if m.CreatedAt.Valid {
			row.JoinedAt = m.CreatedAt.Time
		}
		memberRows = append(memberRows, row)
	}

	// Load teams in this org
	teams, _ := h.DB.ListTeamsByOrganization(ctx, &orgID)
	teamRows := make([]pages.OrgTeamRow, 0, len(teams))
	for _, t := range teams {
		teamAlias := ""
		if t.TeamAlias != nil {
			teamAlias = *t.TeamAlias
		}
		teamRows = append(teamRows, pages.OrgTeamRow{
			TeamID:      t.TeamID,
			TeamAlias:   teamAlias,
			MemberCount: len(t.Members),
		})
	}

	// Pretty-print metadata
	metadataStr := "{}"
	if len(o.Metadata) > 0 {
		var v any
		if json.Unmarshal(o.Metadata, &v) == nil {
			b, _ := json.MarshalIndent(v, "", "  ")
			metadataStr = string(b)
		} else {
			metadataStr = string(o.Metadata)
		}
	}

	return pages.OrgDetailData{
		Org:             orgRow,
		Members:         memberRows,
		Teams:           teamRows,
		AvailableModels: h.loadAvailableModelNames(ctx),
		Metadata:        metadataStr,
	}, true
}

func (h *UIHandler) handleOrgDetail(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "org_id")

	data, ok := h.loadOrgDetailData(r, orgID)
	if !ok {
		http.Redirect(w, r, "/ui/orgs", http.StatusSeeOther)
		return
	}

	render(r.Context(), w, pages.OrgDetailPage(data))
}

func (h *UIHandler) handleOrgUpdate(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "org_id")

	if h.DB == nil {
		http.Error(w, "database not configured", http.StatusServiceUnavailable)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	orgAlias := strings.TrimSpace(r.FormValue("org_alias"))
	var orgAliasPtr *string
	if orgAlias != "" {
		orgAliasPtr = &orgAlias
	}

	maxBudget := parseOptionalFloat(r.FormValue("max_budget"))

	params := db.UpdateOrganizationParams{
		OrganizationID:    orgID,
		OrganizationAlias: orgAliasPtr,
		MaxBudget:         maxBudget,
	}

	if _, err := h.DB.UpdateOrganization(r.Context(), params); err != nil {
		data, ok := h.loadOrgDetailData(r, orgID)
		if !ok {
			http.Error(w, "organization not found", http.StatusNotFound)
			return
		}
		render(r.Context(), w, pages.OrgDetailHeaderWithToast(data, "Failed to update organization: "+err.Error(), toast.VariantError))
		return
	}

	data, ok := h.loadOrgDetailData(r, orgID)
	if !ok {
		http.Error(w, "organization not found", http.StatusNotFound)
		return
	}
	render(r.Context(), w, pages.OrgDetailHeaderWithToast(data, "Organization updated successfully", toast.VariantSuccess))
}

func (h *UIHandler) handleOrgDelete(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		http.Error(w, "database not configured", http.StatusServiceUnavailable)
		return
	}

	orgID := chi.URLParam(r, "org_id")
	if orgID == "" {
		http.Error(w, "org_id required", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	// Prevent deletion if org has teams
	teams, err := h.DB.ListTeamsByOrganization(ctx, &orgID)
	if err == nil && len(teams) > 0 {
		// Check if request is from list page (HX-Target=body or orgs-table) vs detail page
		hxTarget := r.Header.Get("HX-Target")
		if hxTarget == "body" || hxTarget == "orgs-table" {
			data := h.loadOrgsPageData(r)
			render(r.Context(), w, pages.OrgsTableWithToast(data, "Cannot delete organization with teams. Remove all teams first.", toast.VariantError))
			return
		}
		data, ok := h.loadOrgDetailData(r, orgID)
		if ok {
			render(r.Context(), w, pages.OrgDetailHeaderWithToast(data, "Cannot delete organization with teams. Remove all teams first.", toast.VariantError))
			return
		}
		http.Error(w, "cannot delete organization with teams", http.StatusConflict)
		return
	}

	if err := h.DB.DeleteOrganization(ctx, orgID); err != nil {
		hxTarget := r.Header.Get("HX-Target")
		if hxTarget == "orgs-table" {
			data := h.loadOrgsPageData(r)
			render(r.Context(), w, pages.OrgsTableWithToast(data, "Failed to delete organization: "+err.Error(), toast.VariantError))
			return
		}
		data, ok := h.loadOrgDetailData(r, orgID)
		if ok {
			render(r.Context(), w, pages.OrgDetailHeaderWithToast(data, "Failed to delete organization: "+err.Error(), toast.VariantError))
			return
		}
		http.Error(w, "failed to delete organization: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// From list page: return updated table; from detail page: redirect
	hxTarget := r.Header.Get("HX-Target")
	if hxTarget == "orgs-table" {
		data := h.loadOrgsPageData(r)
		render(r.Context(), w, pages.OrgsTableWithToast(data, "Organization deleted successfully", toast.VariantSuccess))
		return
	}
	w.Header().Set("HX-Redirect", "/ui/orgs")
	w.WriteHeader(http.StatusOK)
}

func (h *UIHandler) handleOrgMemberAdd(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "org_id")

	if h.DB == nil {
		http.Error(w, "database not configured", http.StatusServiceUnavailable)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	userID := strings.TrimSpace(r.FormValue("user_id"))
	userRole := strings.TrimSpace(r.FormValue("user_role"))

	if userID == "" || userRole == "" {
		data, ok := h.loadOrgDetailData(r, orgID)
		if !ok {
			http.Error(w, "organization not found", http.StatusNotFound)
			return
		}
		render(r.Context(), w, pages.OrgMembersWithToast(data, "User ID and role are required", toast.VariantError))
		return
	}

	if _, err := h.DB.AddOrgMember(r.Context(), db.AddOrgMemberParams{
		UserID:         userID,
		OrganizationID: orgID,
		UserRole:       &userRole,
		BudgetID:       nil,
	}); err != nil {
		data, _ := h.loadOrgDetailData(r, orgID)
		render(r.Context(), w, pages.OrgMembersWithToast(data, "Failed to add member: "+err.Error(), toast.VariantError))
		return
	}

	data, _ := h.loadOrgDetailData(r, orgID)
	render(r.Context(), w, pages.OrgMembersWithToast(data, "Member added successfully", toast.VariantSuccess))
}

func (h *UIHandler) handleOrgMemberUpdate(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "org_id")

	if h.DB == nil {
		http.Error(w, "database not configured", http.StatusServiceUnavailable)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	userID := strings.TrimSpace(r.FormValue("user_id"))
	userRole := strings.TrimSpace(r.FormValue("user_role"))

	if _, err := h.DB.UpdateOrgMember(r.Context(), db.UpdateOrgMemberParams{
		UserID:         userID,
		OrganizationID: orgID,
		UserRole:       &userRole,
		BudgetID:       nil,
	}); err != nil {
		data, _ := h.loadOrgDetailData(r, orgID)
		render(r.Context(), w, pages.OrgMembersWithToast(data, "Failed to update member role: "+err.Error(), toast.VariantError))
		return
	}

	data, _ := h.loadOrgDetailData(r, orgID)
	render(r.Context(), w, pages.OrgMembersWithToast(data, "Member role updated", toast.VariantSuccess))
}

func (h *UIHandler) handleOrgMemberRemove(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "org_id")

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

	if err := h.DB.DeleteOrgMember(r.Context(), db.DeleteOrgMemberParams{
		UserID:         userID,
		OrganizationID: orgID,
	}); err != nil {
		data, _ := h.loadOrgDetailData(r, orgID)
		render(r.Context(), w, pages.OrgMembersWithToast(data, "Failed to remove member: "+err.Error(), toast.VariantError))
		return
	}

	data, _ := h.loadOrgDetailData(r, orgID)
	render(r.Context(), w, pages.OrgMembersWithToast(data, "Member removed successfully", toast.VariantSuccess))
}
