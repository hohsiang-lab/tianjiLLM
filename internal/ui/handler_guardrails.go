package ui

import (
	"encoding/json"
	"net/http"
	"slices"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/praxisllmlab/tianjiLLM/internal/db"
	"github.com/praxisllmlab/tianjiLLM/internal/guardrail"
	"github.com/praxisllmlab/tianjiLLM/internal/model"
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

// --- Policy Binding handlers ---

func (h *UIHandler) loadGuardrailBindingsData(r *http.Request, guardrailID string) pages.GuardrailBindingsData {
	if h.DB == nil {
		return pages.GuardrailBindingsData{GuardrailID: guardrailID, Error: "database not configured"}
	}

	ctx := r.Context()

	g, err := h.DB.GetGuardrailConfig(ctx, guardrailID)
	if err != nil {
		return pages.GuardrailBindingsData{GuardrailID: guardrailID, Error: "Guardrail not found: " + err.Error()}
	}

	policies, err := h.DB.ListPolicies(ctx)
	if err != nil {
		return pages.GuardrailBindingsData{GuardrailID: guardrailID, GuardrailName: g.GuardrailName, Error: "Failed to load policies: " + err.Error()}
	}

	var bound, unbound []pages.GuardrailBindingPolicy
	for _, p := range policies {
		info := pages.GuardrailBindingPolicy{ID: p.ID, Name: p.Name}
		attachments, _ := h.DB.ListPolicyAttachmentsByPolicy(ctx, p.Name)
		for _, a := range attachments {
			info.Teams = append(info.Teams, a.Teams...)
			info.Keys = append(info.Keys, a.Keys...)
		}
		if slices.Contains(p.GuardrailsAdd, g.GuardrailName) {
			bound = append(bound, info)
		} else {
			unbound = append(unbound, info)
		}
	}

	return pages.GuardrailBindingsData{
		GuardrailID:     guardrailID,
		GuardrailName:   g.GuardrailName,
		BoundPolicies:   bound,
		UnboundPolicies: unbound,
	}
}

func (h *UIHandler) handleGuardrailBindings(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	data := h.loadGuardrailBindingsData(r, id)
	render(r.Context(), w, pages.GuardrailBindingsPartial(data))
}

func (h *UIHandler) handleGuardrailBindingAdd(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		http.Error(w, "database not configured", http.StatusServiceUnavailable)
		return
	}

	id := chi.URLParam(r, "id")
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	policyID := r.FormValue("policy_id")

	g, err := h.DB.GetGuardrailConfig(r.Context(), id)
	if err != nil {
		data := pages.GuardrailBindingsData{GuardrailID: id, Error: "Guardrail not found"}
		render(r.Context(), w, pages.GuardrailBindingsPartial(data))
		return
	}

	p, err := h.DB.GetPolicy(r.Context(), policyID)
	if err != nil {
		data := h.loadGuardrailBindingsData(r, id)
		data.Error = "Policy not found"
		render(r.Context(), w, pages.GuardrailBindingsPartial(data))
		return
	}

	if !slices.Contains(p.GuardrailsAdd, g.GuardrailName) {
		newGuardrails := append(p.GuardrailsAdd, g.GuardrailName)
		_, err = h.DB.UpdatePolicy(r.Context(), db.UpdatePolicyParams{
			ID:               p.ID,
			Name:             p.Name,
			ParentID:         p.ParentID,
			Conditions:       p.Conditions,
			GuardrailsAdd:    newGuardrails,
			GuardrailsRemove: p.GuardrailsRemove,
			Pipeline:         p.Pipeline,
			Description:      p.Description,
		})
		if err != nil {
			data := h.loadGuardrailBindingsData(r, id)
			data.Error = "Failed to add binding: " + err.Error()
			render(r.Context(), w, pages.GuardrailBindingsPartial(data))
			return
		}
	}

	data := h.loadGuardrailBindingsData(r, id)
	render(r.Context(), w, pages.GuardrailBindingsPartial(data))
}

func (h *UIHandler) handleGuardrailBindingRemove(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		http.Error(w, "database not configured", http.StatusServiceUnavailable)
		return
	}

	id := chi.URLParam(r, "id")
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	policyID := r.FormValue("policy_id")

	g, err := h.DB.GetGuardrailConfig(r.Context(), id)
	if err != nil {
		data := pages.GuardrailBindingsData{GuardrailID: id, Error: "Guardrail not found"}
		render(r.Context(), w, pages.GuardrailBindingsPartial(data))
		return
	}

	p, err := h.DB.GetPolicy(r.Context(), policyID)
	if err != nil {
		data := h.loadGuardrailBindingsData(r, id)
		data.Error = "Policy not found"
		render(r.Context(), w, pages.GuardrailBindingsPartial(data))
		return
	}

	newGuardrails := make([]string, 0, len(p.GuardrailsAdd))
	for _, v := range p.GuardrailsAdd {
		if v != g.GuardrailName {
			newGuardrails = append(newGuardrails, v)
		}
	}

	_, err = h.DB.UpdatePolicy(r.Context(), db.UpdatePolicyParams{
		ID:               p.ID,
		Name:             p.Name,
		ParentID:         p.ParentID,
		Conditions:       p.Conditions,
		GuardrailsAdd:    newGuardrails,
		GuardrailsRemove: p.GuardrailsRemove,
		Pipeline:         p.Pipeline,
		Description:      p.Description,
	})
	if err != nil {
		data := h.loadGuardrailBindingsData(r, id)
		data.Error = "Failed to remove binding: " + err.Error()
		render(r.Context(), w, pages.GuardrailBindingsPartial(data))
		return
	}

	data := h.loadGuardrailBindingsData(r, id)
	render(r.Context(), w, pages.GuardrailBindingsPartial(data))
}

// --- Test handler ---

func (h *UIHandler) handleGuardrailTest(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		render(r.Context(), w, pages.GuardrailTestResultPartial(true, false, "", "database not configured"))
		return
	}

	id := chi.URLParam(r, "id")
	if err := r.ParseForm(); err != nil {
		render(r.Context(), w, pages.GuardrailTestResultPartial(true, false, "", "bad request"))
		return
	}

	testText := strings.TrimSpace(r.FormValue("test_text"))
	if testText == "" {
		render(r.Context(), w, pages.GuardrailTestResultPartial(true, false, "", "Test text is required"))
		return
	}

	g, err := h.DB.GetGuardrailConfig(r.Context(), id)
	if err != nil {
		render(r.Context(), w, pages.GuardrailTestResultPartial(true, false, "", "Guardrail not found: "+err.Error()))
		return
	}

	var params map[string]any
	if jsonErr := json.Unmarshal(g.Config, &params); jsonErr != nil || params == nil {
		params = map[string]any{}
	}
	params["mode"] = g.GuardrailType

	gc := config.GuardrailConfig{
		GuardrailName: g.GuardrailName,
		TianjiParams:  params,
		FailurePolicy: g.FailurePolicy,
	}

	guardrailInst, err := guardrail.NewFromConfig(gc)
	if err != nil {
		render(r.Context(), w, pages.GuardrailTestResultPartial(true, false, "", "Test not supported for this guardrail type: "+err.Error()))
		return
	}

	req := &model.ChatCompletionRequest{
		Messages: []model.Message{{Role: "user", Content: testText}},
	}

	result, err := guardrailInst.Run(r.Context(), guardrail.HookPreCall, req, nil)
	if err != nil {
		render(r.Context(), w, pages.GuardrailTestResultPartial(true, false, "", "Test error: "+err.Error()))
		return
	}

	render(r.Context(), w, pages.GuardrailTestResultPartial(true, result.Passed, result.Message, ""))
}
