package ui

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/praxisllmlab/tianjiLLM/internal/db"
	"github.com/praxisllmlab/tianjiLLM/internal/ui/components/toast"
	"github.com/praxisllmlab/tianjiLLM/internal/ui/pages"
)

const orgsPerPage = 50

func (h *UIHandler) handleOrgs(w http.ResponseWriter, r *http.Request) {
	data := h.loadOrgsPageData(r)
	render(r.Context(), w, pages.OrgsPage(data))
}

func (h *UIHandler) handleOrgsTable(w http.ResponseWriter, r *http.Request) {
	data := h.loadOrgsPageData(r)
	render(r.Context(), w, pages.OrgsTablePartial(data))
}

func (h *UIHandler) loadOrgsPageData(r *http.Request) pages.OrgsPageData {
	page := parsePage(r.URL.Query().Get("page"))
	search := r.URL.Query().Get("search")

	data := pages.OrgsPageData{
		Page:    page,
		PerPage: orgsPerPage,
		Search:  search,
	}

	if h.DB == nil {
		return data
	}

	ctx := r.Context()

	data.AvailableModels = h.loadAvailableModelNames(ctx)

	allOrgs, err := h.DB.ListOrganizations(ctx)
	if err != nil {
		return data
	}

	// Build team count map
	teamCounts := map[string]int{}
	if rows, err := h.DB.CountTeamsPerOrganization(ctx); err == nil {
		for _, row := range rows {
			if row.OrganizationID != nil {
				teamCounts[*row.OrganizationID] = int(row.TeamCount)
			}
		}
	}

	// Build member count map
	memberCounts := map[string]int{}
	if rows, err := h.DB.CountMembersPerOrganization(ctx); err == nil {
		for _, row := range rows {
			memberCounts[row.OrganizationID] = int(row.MemberCount)
		}
	}

	// Filter and map to OrgRow
	var filtered []pages.OrgRow
	for _, o := range allOrgs {
		alias := ""
		if o.OrganizationAlias != nil {
			alias = *o.OrganizationAlias
		}
		if search != "" && !strings.Contains(strings.ToLower(alias), strings.ToLower(search)) {
			continue
		}

		row := pages.OrgRow{
			OrgID:       o.OrganizationID,
			OrgAlias:    alias,
			TeamCount:   teamCounts[o.OrganizationID],
			MemberCount: memberCounts[o.OrganizationID],
			Spend:       o.Spend,
			MaxBudget:   o.MaxBudget,
			ModelCount:  len(o.Models),
			Models:      o.Models,
			TpmLimit:    o.TpmLimit,
			RpmLimit:    o.RpmLimit,
		}
		if o.CreatedAt.Valid {
			row.CreatedAt = o.CreatedAt.Time
		}
		filtered = append(filtered, row)
	}

	data.TotalCount = len(filtered)
	data.TotalPages = (data.TotalCount + data.PerPage - 1) / data.PerPage
	if data.TotalPages < 1 {
		data.TotalPages = 1
	}

	offset := (page - 1) * data.PerPage
	if offset > len(filtered) {
		offset = len(filtered)
	}
	end := offset + data.PerPage
	if end > len(filtered) {
		end = len(filtered)
	}
	data.Orgs = filtered[offset:end]

	return data
}

func (h *UIHandler) handleOrgCreate(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		http.Error(w, "database not configured", http.StatusServiceUnavailable)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	orgAlias := strings.TrimSpace(r.FormValue("org_alias"))
	if orgAlias == "" {
		data := h.loadOrgsPageData(r)
		render(r.Context(), w, pages.OrgsTableWithToast(data, "Organization alias is required", toast.VariantError))
		return
	}

	maxBudget := parseOptionalFloat(r.FormValue("max_budget"))

	models := parseModelSelection(r.FormValue("all_models"), r.Form["models"])
	if models == nil {
		models = []string{}
	}

	orgID := uuid.New().String()

	params := db.CreateOrganizationParams{
		OrganizationID:    orgID,
		OrganizationAlias: &orgAlias,
		MaxBudget:         maxBudget,
		Models:            models,
		CreatedBy:         "admin",
	}

	_, err := h.DB.CreateOrganization(r.Context(), params)
	if err != nil {
		data := h.loadOrgsPageData(r)
		render(r.Context(), w, pages.OrgsTableWithToast(data, "Failed to create organization: "+err.Error(), toast.VariantError))
		return
	}

	data := h.loadOrgsPageData(r)
	render(r.Context(), w, pages.OrgsTableWithToast(data, "Organization created successfully", toast.VariantSuccess))
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

	if err := h.DB.DeleteOrganization(r.Context(), orgID); err != nil {
		data := h.loadOrgsPageData(r)
		render(r.Context(), w, pages.OrgsTableWithToast(data, "Failed to delete organization: "+err.Error(), toast.VariantError))
		return
	}

	w.Header().Set("HX-Redirect", "/ui/orgs")
	w.WriteHeader(http.StatusOK)
}
