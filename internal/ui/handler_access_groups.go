package ui

import (
	"log"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/praxisllmlab/tianjiLLM/internal/db"
	"github.com/praxisllmlab/tianjiLLM/internal/ui/components/toast"
	"github.com/praxisllmlab/tianjiLLM/internal/ui/pages"
)

func (h *UIHandler) handleAccessGroups(w http.ResponseWriter, r *http.Request) {
	data := h.loadAccessGroupsPageData(r)
	render(r.Context(), w, pages.AccessGroupsPage(data))
}

func (h *UIHandler) handleAccessGroupsTable(w http.ResponseWriter, r *http.Request) {
	data := h.loadAccessGroupsPageData(r)
	render(r.Context(), w, pages.AccessGroupsTablePartial(data))
}

func (h *UIHandler) loadAccessGroupsPageData(r *http.Request) pages.AccessGroupsPageData {
	page := parsePage(r.URL.Query().Get("page"))
	search := r.URL.Query().Get("search")

	data := pages.AccessGroupsPageData{
		Page:    page,
		PerPage: 20,
		Search:  search,
	}

	if h.DB == nil {
		return data
	}

	ctx := r.Context()

	data.AvailableModels = h.loadAvailableModelNames(ctx)

	orgs, err := h.DB.ListOrganizations(ctx)
	if err != nil {
		log.Printf("ui: failed to list organizations: %v", err)
	}
	orgAliasMap := map[string]string{}
	for _, o := range orgs {
		opt := pages.OrgOption{ID: o.OrganizationID}
		if o.OrganizationAlias != nil {
			opt.Alias = *o.OrganizationAlias
			orgAliasMap[o.OrganizationID] = *o.OrganizationAlias
		}
		data.Orgs = append(data.Orgs, opt)
	}

	all, err := h.DB.ListAccessGroups(ctx)
	if err != nil {
		return data
	}

	// Load all keys for the "add key" dropdown
	allTokens, _ := h.DB.ListAllKeySummaries(ctx)
	for _, t := range allTokens {
		opt := pages.KeyMemberOption{Token: t.Token}
		if t.KeyName != nil {
			opt.KeyName = *t.KeyName
		}
		if t.KeyAlias != nil {
			opt.KeyAlias = *t.KeyAlias
		}
		data.AllKeys = append(data.AllKeys, opt)
	}

	var filtered []pages.AccessGroupRow
	for _, g := range all {
		alias := ""
		if g.GroupAlias != nil {
			alias = *g.GroupAlias
		}
		if search != "" && !strings.Contains(strings.ToLower(alias), strings.ToLower(search)) {
			continue
		}
		orgID := ""
		if g.OrganizationID != nil {
			orgID = *g.OrganizationID
		}
		row := pages.AccessGroupRow{
			GroupID:    g.GroupID,
			GroupAlias: alias,
			Models:     g.Models,
			OrgID:      orgID,
			OrgAlias:   orgAliasMap[orgID],
		}
		if g.CreatedAt.Valid {
			row.CreatedAt = g.CreatedAt.Time
		}

		// TODO: batch query to avoid N+1 when group count grows
		// Load key members for this access group
		members, err := h.DB.ListKeysByAccessGroup(ctx, g.GroupID)
		if err == nil {
			for _, m := range members {
				km := pages.KeyMemberOption{Token: m.Token}
				if m.KeyName != nil {
					km.KeyName = *m.KeyName
				}
				if m.KeyAlias != nil {
					km.KeyAlias = *m.KeyAlias
				}
				row.Members = append(row.Members, km)
			}
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
	data.Groups = filtered[offset:end]

	return data
}

func (h *UIHandler) handleAccessGroupCreate(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		http.Error(w, "database not configured", http.StatusServiceUnavailable)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	groupAlias := strings.TrimSpace(r.FormValue("group_alias"))
	if groupAlias == "" {
		data := h.loadAccessGroupsPageData(r)
		render(r.Context(), w, pages.AccessGroupsTableWithToast(data, "Group alias is required", toast.VariantError))
		return
	}

	orgID := r.FormValue("organization_id")
	var orgIDPtr *string
	if orgID != "" {
		orgIDPtr = &orgID
	}

	models := parseModelSelection(r.FormValue("all_models"), r.Form["models"])
	if models == nil {
		models = []string{}
	}

	groupID := uuid.New().String()

	params := db.CreateAccessGroupParams{
		GroupID:        groupID,
		GroupAlias:     &groupAlias,
		Models:         models,
		OrganizationID: orgIDPtr,
		CreatedBy:      "admin",
	}

	_, err := h.DB.CreateAccessGroup(r.Context(), params)
	if err != nil {
		data := h.loadAccessGroupsPageData(r)
		render(r.Context(), w, pages.AccessGroupsTableWithToast(data, "Failed to create access group: "+err.Error(), toast.VariantError))
		return
	}

	data := h.loadAccessGroupsPageData(r)
	render(r.Context(), w, pages.AccessGroupsTableWithToast(data, "Access group created successfully", toast.VariantSuccess))
}

func (h *UIHandler) handleAccessGroupUpdate(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		http.Error(w, "database not configured", http.StatusServiceUnavailable)
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "id required", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	groupAlias := strings.TrimSpace(r.FormValue("group_alias"))
	var groupAliasPtr *string
	if groupAlias != "" {
		groupAliasPtr = &groupAlias
	}

	models := parseModelSelection(r.FormValue("all_models"), r.Form["models"])
	if models == nil {
		models = []string{}
	}

	params := db.UpdateAccessGroupParams{
		GroupID:    id,
		GroupAlias: groupAliasPtr,
		Models:     models,
	}

	if err := h.DB.UpdateAccessGroup(r.Context(), params); err != nil {
		data := h.loadAccessGroupsPageData(r)
		render(r.Context(), w, pages.AccessGroupsTableWithToast(data, "Failed to update access group: "+err.Error(), toast.VariantError))
		return
	}

	data := h.loadAccessGroupsPageData(r)
	render(r.Context(), w, pages.AccessGroupsTableWithToast(data, "Access group updated successfully", toast.VariantSuccess))
}

func (h *UIHandler) handleAccessGroupDelete(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		http.Error(w, "database not configured", http.StatusServiceUnavailable)
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "id required", http.StatusBadRequest)
		return
	}

	// Clean up access_group_ids references from all keys before deleting the group
	if err := h.DB.RemoveAccessGroupFromAllKeys(r.Context(), id); err != nil {
		data := h.loadAccessGroupsPageData(r)
		render(r.Context(), w, pages.AccessGroupsTableWithToast(data, "Failed to clean up key references: "+err.Error(), toast.VariantError))
		return
	}

	if err := h.DB.DeleteAccessGroup(r.Context(), id); err != nil {
		data := h.loadAccessGroupsPageData(r)
		render(r.Context(), w, pages.AccessGroupsTableWithToast(data, "Failed to delete access group: "+err.Error(), toast.VariantError))
		return
	}

	data := h.loadAccessGroupsPageData(r)
	render(r.Context(), w, pages.AccessGroupsTableWithToast(data, "Access group deleted successfully", toast.VariantSuccess))
}

func (h *UIHandler) handleAccessGroupAddKey(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		http.Error(w, "database not configured", http.StatusServiceUnavailable)
		return
	}

	groupID := chi.URLParam(r, "id")
	if groupID == "" {
		http.Error(w, "id required", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	token := r.FormValue("token")
	if token == "" {
		data := h.loadAccessGroupsPageData(r)
		render(r.Context(), w, pages.AccessGroupsTableWithToast(data, "Key token is required", toast.VariantError))
		return
	}

	params := db.AddKeyToAccessGroupParams{
		GroupID: groupID,
		Token:   token,
	}
	if err := h.DB.AddKeyToAccessGroup(r.Context(), params); err != nil {
		data := h.loadAccessGroupsPageData(r)
		render(r.Context(), w, pages.AccessGroupsTableWithToast(data, "Failed to add key: "+err.Error(), toast.VariantError))
		return
	}

	data := h.loadAccessGroupsPageData(r)
	render(r.Context(), w, pages.AccessGroupsTableWithToast(data, "Key added to access group", toast.VariantSuccess))
}

func (h *UIHandler) handleAccessGroupRemoveKey(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		http.Error(w, "database not configured", http.StatusServiceUnavailable)
		return
	}

	groupID := chi.URLParam(r, "id")
	if groupID == "" {
		http.Error(w, "id required", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	token := r.FormValue("token")
	if token == "" {
		data := h.loadAccessGroupsPageData(r)
		render(r.Context(), w, pages.AccessGroupsTableWithToast(data, "Key token is required", toast.VariantError))
		return
	}

	params := db.RemoveKeyFromAccessGroupParams{
		GroupID: groupID,
		Token:   token,
	}
	if err := h.DB.RemoveKeyFromAccessGroup(r.Context(), params); err != nil {
		data := h.loadAccessGroupsPageData(r)
		render(r.Context(), w, pages.AccessGroupsTableWithToast(data, "Failed to remove key: "+err.Error(), toast.VariantError))
		return
	}

	data := h.loadAccessGroupsPageData(r)
	render(r.Context(), w, pages.AccessGroupsTableWithToast(data, "Key removed from access group", toast.VariantSuccess))
}
