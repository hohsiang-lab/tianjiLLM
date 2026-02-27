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

func (h *UIHandler) handleGuardrails(w http.ResponseWriter, r *http.Request) {
	data := h.loadGuardrailsPageData(r)
	render(r.Context(), w, pages.GuardrailsPage(data))
}

func (h *UIHandler) handleGuardrailsTable(w http.ResponseWriter, r *http.Request) {
	data := h.loadGuardrailsPageData(r)
	render(r.Context(), w, pages.GuardrailsTablePartial(data))
}

func (h *UIHandler) loadGuardrailsPageData(r *http.Request) pages.GuardrailsPageData {
	page := parsePage(r.URL.Query().Get("page"))
	search := r.URL.Query().Get("search")

	data := pages.GuardrailsPageData{
		Page:    page,
		PerPage: 20,
		Search:  search,
	}

	if h.DB == nil {
		return data
	}

	ctx := r.Context()

	all, err := h.DB.ListGuardrailConfigs(ctx)
	if err != nil {
		return data
	}

	var filtered []pages.GuardrailRow
	for _, g := range all {
		if search != "" && !strings.Contains(strings.ToLower(g.GuardrailName), strings.ToLower(search)) {
			continue
		}
		row := pages.GuardrailRow{
			ID:            g.ID,
			Name:          g.GuardrailName,
			Type:          g.GuardrailType,
			FailurePolicy: g.FailurePolicy,
			Enabled:       g.Enabled,
			ConfigJSON:    string(g.Config),
		}
		if g.CreatedAt.Valid {
			row.CreatedAt = g.CreatedAt.Time
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
	data.Guardrails = filtered[offset:end]

	return data
}

func (h *UIHandler) handleGuardrailCreate(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		http.Error(w, "database not configured", http.StatusServiceUnavailable)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	name := strings.TrimSpace(r.FormValue("guardrail_name"))
	if name == "" {
		data := h.loadGuardrailsPageData(r)
		render(r.Context(), w, pages.GuardrailsTableWithToast(data, "Guardrail name is required", toast.VariantError))
		return
	}

	// Check name uniqueness
	existing, err := h.DB.GetGuardrailConfigByName(r.Context(), name)
	if err == nil && existing.ID != "" {
		data := h.loadGuardrailsPageData(r)
		render(r.Context(), w, pages.GuardrailsTableWithToast(data, "Guardrail name already exists", toast.VariantError))
		return
	}

	guardrailType := r.FormValue("guardrail_type")
	failurePolicy := r.FormValue("failure_policy")
	if failurePolicy == "" {
		failurePolicy = "fail_open"
	}
	configStr := strings.TrimSpace(r.FormValue("config"))
	if configStr == "" {
		configStr = "{}"
	}
	// Validate JSON
	if !json.Valid([]byte(configStr)) {
		data := h.loadGuardrailsPageData(r)
		render(r.Context(), w, pages.GuardrailsTableWithToast(data, "Config must be valid JSON", toast.VariantError))
		return
	}
	enabled := r.FormValue("enabled") == "on" || r.FormValue("enabled") == "true"

	params := db.CreateGuardrailConfigParams{
		GuardrailName: name,
		GuardrailType: guardrailType,
		Config:        []byte(configStr),
		FailurePolicy: failurePolicy,
		Enabled:       enabled,
	}

	_, err = h.DB.CreateGuardrailConfig(r.Context(), params)
	if err != nil {
		data := h.loadGuardrailsPageData(r)
		render(r.Context(), w, pages.GuardrailsTableWithToast(data, "Failed to create guardrail: "+err.Error(), toast.VariantError))
		return
	}

	data := h.loadGuardrailsPageData(r)
	render(r.Context(), w, pages.GuardrailsTableWithToast(data, "Guardrail created successfully", toast.VariantSuccess))
}

func (h *UIHandler) handleGuardrailUpdate(w http.ResponseWriter, r *http.Request) {
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

	name := strings.TrimSpace(r.FormValue("guardrail_name"))
	if name == "" {
		data := h.loadGuardrailsPageData(r)
		render(r.Context(), w, pages.GuardrailsTableWithToast(data, "Guardrail name is required", toast.VariantError))
		return
	}

	guardrailType := r.FormValue("guardrail_type")
	failurePolicy := r.FormValue("failure_policy")
	if failurePolicy == "" {
		failurePolicy = "fail_open"
	}
	configStr := strings.TrimSpace(r.FormValue("config"))
	if configStr == "" {
		configStr = "{}"
	}
	if !json.Valid([]byte(configStr)) {
		data := h.loadGuardrailsPageData(r)
		render(r.Context(), w, pages.GuardrailsTableWithToast(data, "Config must be valid JSON", toast.VariantError))
		return
	}
	enabled := r.FormValue("enabled") == "on" || r.FormValue("enabled") == "true"

	params := db.UpdateGuardrailConfigParams{
		ID:            id,
		GuardrailName: name,
		GuardrailType: guardrailType,
		Config:        []byte(configStr),
		FailurePolicy: failurePolicy,
		Enabled:       enabled,
	}

	_, err := h.DB.UpdateGuardrailConfig(r.Context(), params)
	if err != nil {
		data := h.loadGuardrailsPageData(r)
		render(r.Context(), w, pages.GuardrailsTableWithToast(data, "Failed to update guardrail: "+err.Error(), toast.VariantError))
		return
	}

	data := h.loadGuardrailsPageData(r)
	render(r.Context(), w, pages.GuardrailsTableWithToast(data, "Guardrail updated successfully", toast.VariantSuccess))
}

func (h *UIHandler) handleGuardrailToggle(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		http.Error(w, "database not configured", http.StatusServiceUnavailable)
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "id required", http.StatusBadRequest)
		return
	}

	existing, err := h.DB.GetGuardrailConfig(r.Context(), id)
	if err != nil {
		data := h.loadGuardrailsPageData(r)
		render(r.Context(), w, pages.GuardrailsTableWithToast(data, "Guardrail not found", toast.VariantError))
		return
	}

	params := db.UpdateGuardrailConfigParams{
		ID:            existing.ID,
		GuardrailName: existing.GuardrailName,
		GuardrailType: existing.GuardrailType,
		Config:        existing.Config,
		FailurePolicy: existing.FailurePolicy,
		Enabled:       !existing.Enabled,
	}

	_, err = h.DB.UpdateGuardrailConfig(r.Context(), params)
	if err != nil {
		data := h.loadGuardrailsPageData(r)
		render(r.Context(), w, pages.GuardrailsTableWithToast(data, "Failed to toggle guardrail: "+err.Error(), toast.VariantError))
		return
	}

	msg := "Guardrail enabled"
	if existing.Enabled {
		msg = "Guardrail disabled"
	}
	data := h.loadGuardrailsPageData(r)
	render(r.Context(), w, pages.GuardrailsTableWithToast(data, msg, toast.VariantSuccess))
}

func (h *UIHandler) handleGuardrailDelete(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		http.Error(w, "database not configured", http.StatusServiceUnavailable)
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "id required", http.StatusBadRequest)
		return
	}

	if err := h.DB.DeleteGuardrailConfig(r.Context(), id); err != nil {
		data := h.loadGuardrailsPageData(r)
		render(r.Context(), w, pages.GuardrailsTableWithToast(data, "Failed to delete guardrail: "+err.Error(), toast.VariantError))
		return
	}

	data := h.loadGuardrailsPageData(r)
	render(r.Context(), w, pages.GuardrailsTableWithToast(data, "Guardrail deleted successfully", toast.VariantSuccess))
}
