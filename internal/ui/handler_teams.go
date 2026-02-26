package ui

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/praxisllmlab/tianjiLLM/internal/db"
	"github.com/praxisllmlab/tianjiLLM/internal/ui/components/toast"
	"github.com/praxisllmlab/tianjiLLM/internal/ui/pages"
)

func (h *UIHandler) handleTeams(w http.ResponseWriter, r *http.Request) {
	data := h.loadTeamsPageData(r)
	render(r.Context(), w, pages.TeamsPage(data))
}

func (h *UIHandler) handleTeamsTable(w http.ResponseWriter, r *http.Request) {
	data := h.loadTeamsPageData(r)
	render(r.Context(), w, pages.TeamsTablePartial(data))
}

func (h *UIHandler) loadTeamsPageData(r *http.Request) pages.TeamsPageData {
	page := parsePage(r.URL.Query().Get("page"))
	search := r.URL.Query().Get("search")
	filterOrgID := r.URL.Query().Get("filter_org_id")

	data := pages.TeamsPageData{
		Page:        page,
		PerPage:     50,
		Search:      search,
		FilterOrgID: filterOrgID,
	}

	if h.DB == nil {
		return data
	}

	ctx := r.Context()

	// Load available models for create form
	data.AvailableModels = h.loadAvailableModelNames(ctx)

	// Load orgs for filter dropdown
	orgs, _ := h.DB.ListOrganizations(ctx)
	for _, o := range orgs {
		opt := pages.OrgOption{ID: o.OrganizationID}
		if o.OrganizationAlias != nil {
			opt.Alias = *o.OrganizationAlias
		}
		data.Orgs = append(data.Orgs, opt)
	}

	// Build org alias map for display
	orgAliasMap := map[string]string{}
	for _, o := range orgs {
		if o.OrganizationAlias != nil {
			orgAliasMap[o.OrganizationID] = *o.OrganizationAlias
		}
	}

	// Load all teams
	allTeams, err := h.DB.ListTeams(ctx)
	if err != nil {
		return data
	}

	// Filter in Go
	var filtered []pages.TeamRow
	for _, t := range allTeams {
		alias := ""
		if t.TeamAlias != nil {
			alias = *t.TeamAlias
		}
		if search != "" && !strings.Contains(strings.ToLower(alias), strings.ToLower(search)) {
			continue
		}
		orgID := ""
		if t.OrganizationID != nil {
			orgID = *t.OrganizationID
		}
		if filterOrgID != "" && orgID != filterOrgID {
			continue
		}

		row := pages.TeamRow{
			TeamID:      t.TeamID,
			TeamAlias:   alias,
			OrgID:       orgID,
			OrgAlias:    orgAliasMap[orgID],
			MemberCount: len(t.Members),
			Spend:       t.Spend,
			MaxBudget:   t.MaxBudget,
			ModelCount:  len(t.Models),
			Models:      t.Models,
			TPMLimit:    t.TpmLimit,
			RPMLimit:    t.RpmLimit,
			Blocked:     t.Blocked,
		}
		if t.CreatedAt.Valid {
			row.CreatedAt = t.CreatedAt.Time
		}
		filtered = append(filtered, row)
	}

	data.TotalCount = len(filtered)
	data.TotalPages = (data.TotalCount + data.PerPage - 1) / data.PerPage
	if data.TotalPages < 1 {
		data.TotalPages = 1
	}

	// Paginate
	offset := (page - 1) * data.PerPage
	if offset > len(filtered) {
		offset = len(filtered)
	}
	end := offset + data.PerPage
	if end > len(filtered) {
		end = len(filtered)
	}
	data.Teams = filtered[offset:end]

	return data
}

func (h *UIHandler) handleTeamCreate(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		http.Error(w, "database not configured", http.StatusServiceUnavailable)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	teamAlias := strings.TrimSpace(r.FormValue("team_alias"))
	if teamAlias == "" {
		data := h.loadTeamsPageData(r)
		render(r.Context(), w, pages.TeamsTableWithToast(data, "Team alias is required", toast.VariantError))
		return
	}

	// Check alias uniqueness
	existing, err := h.DB.GetTeamByAlias(r.Context(), &teamAlias)
	if err == nil && existing.TeamID != "" {
		data := h.loadTeamsPageData(r)
		render(r.Context(), w, pages.TeamsTableWithToast(data, "Team alias already exists", toast.VariantError))
		return
	}

	orgID := r.FormValue("organization_id")
	var orgIDPtr *string
	if orgID != "" {
		orgIDPtr = &orgID
	}

	maxBudget := parseOptionalFloat(r.FormValue("max_budget"))
	tpmLimit := parseOptionalInt64(r.FormValue("tpm_limit"))
	rpmLimit := parseOptionalInt64(r.FormValue("rpm_limit"))
	budgetDuration := r.FormValue("budget_duration")
	var budgetDurationPtr *string
	if budgetDuration != "" {
		budgetDurationPtr = &budgetDuration
	}

	models := parseModelSelection(r.FormValue("all_models"), r.Form["models"])

	teamID := uuid.New().String()

	params := db.CreateTeamParams{
		TeamID:         teamID,
		TeamAlias:      &teamAlias,
		OrganizationID: orgIDPtr,
		Admins:         []string{},
		Members:        []string{},
		MaxBudget:      maxBudget,
		Models:         models,
		TpmLimit:       tpmLimit,
		RpmLimit:       rpmLimit,
		BudgetDuration: budgetDurationPtr,
		CreatedBy:      "admin",
	}

	_, err = h.DB.CreateTeam(r.Context(), params)
	if err != nil {
		data := h.loadTeamsPageData(r)
		render(r.Context(), w, pages.TeamsTableWithToast(data, "Failed to create team: "+err.Error(), toast.VariantError))
		return
	}

	data := h.loadTeamsPageData(r)
	render(r.Context(), w, pages.TeamsTableWithToast(data, "Team created successfully", toast.VariantSuccess))
}

func (h *UIHandler) handleTeamBlock(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		http.Error(w, "database not configured", http.StatusServiceUnavailable)
		return
	}

	teamID := chi.URLParam(r, "team_id")
	if teamID == "" {
		http.Error(w, "team_id required", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	if err := h.DB.BlockTeam(r.Context(), teamID); err != nil {
		data := h.loadTeamsPageData(r)
		render(r.Context(), w, pages.TeamsTableWithToast(data, "Failed to block team: "+err.Error(), toast.VariantError))
		return
	}

	if r.FormValue("return_to") == "detail" {
		http.Redirect(w, r, "/ui/teams/"+teamID, http.StatusSeeOther)
		return
	}

	data := h.loadTeamsPageData(r)
	render(r.Context(), w, pages.TeamsTableWithToast(data, "Team blocked successfully", toast.VariantSuccess))
}

func (h *UIHandler) handleTeamUnblock(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		http.Error(w, "database not configured", http.StatusServiceUnavailable)
		return
	}

	teamID := chi.URLParam(r, "team_id")
	if teamID == "" {
		http.Error(w, "team_id required", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	if err := h.DB.UnblockTeam(r.Context(), teamID); err != nil {
		data := h.loadTeamsPageData(r)
		render(r.Context(), w, pages.TeamsTableWithToast(data, "Failed to unblock team: "+err.Error(), toast.VariantError))
		return
	}

	if r.FormValue("return_to") == "detail" {
		http.Redirect(w, r, "/ui/teams/"+teamID, http.StatusSeeOther)
		return
	}

	data := h.loadTeamsPageData(r)
	render(r.Context(), w, pages.TeamsTableWithToast(data, "Team unblocked successfully", toast.VariantSuccess))
}

func (h *UIHandler) handleTeamDelete(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		http.Error(w, "database not configured", http.StatusServiceUnavailable)
		return
	}

	teamID := chi.URLParam(r, "team_id")
	if teamID == "" {
		http.Error(w, "team_id required", http.StatusBadRequest)
		return
	}

	if err := h.DB.DeleteTeam(r.Context(), teamID); err != nil {
		data := h.loadTeamsPageData(r)
		render(r.Context(), w, pages.TeamsTableWithToast(data, "Failed to delete team: "+err.Error(), toast.VariantError))
		return
	}

	w.Header().Set("HX-Redirect", "/ui/teams")
	w.WriteHeader(http.StatusOK)
}

// parsePage parses the page query param. Returns 1 for any invalid or out-of-range value.
func parsePage(s string) int {
	if s == "" {
		return 1
	}
	p, err := strconv.Atoi(s)
	if err != nil || p < 1 {
		return 1
	}
	return p
}

// parseMembersWithRoles parses the members_with_roles JSONB column.
// Returns an empty slice (not nil) on null/empty input. Returns an error for malformed JSON.
func parseMembersWithRoles(data []byte) ([]memberWithRole, error) {
	if len(data) == 0 {
		return []memberWithRole{}, nil
	}
	var result []memberWithRole
	if err := json.Unmarshal(data, &result); err != nil {
		return []memberWithRole{}, err
	}
	if result == nil {
		return []memberWithRole{}, nil
	}
	return result, nil
}

type memberWithRole struct {
	UserID string `json:"user_id"`
	Role   string `json:"role"`
}
