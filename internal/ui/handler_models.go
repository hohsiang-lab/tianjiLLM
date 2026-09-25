package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/a-h/templ"
	"github.com/google/uuid"

	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/praxisllmlab/tianjiLLM/internal/db"
	"github.com/praxisllmlab/tianjiLLM/internal/pricing"
	"github.com/praxisllmlab/tianjiLLM/internal/provider"
	"github.com/praxisllmlab/tianjiLLM/internal/ui/components/toast"
	"github.com/praxisllmlab/tianjiLLM/internal/ui/pages"
)

const modelsPerPage = 20

const defaultUpstreamURL = "https://raw.githubusercontent.com/BerriAI/litellm/main/model_prices_and_context_window.json"

const createdByUI = "ui"

// handleSyncPricing handles POST /ui/models/sync-pricing.
// Uses TryLock to prevent concurrent syncs; reads PRICING_UPSTREAM_URL env var.
func (h *UIHandler) handleSyncPricing(w http.ResponseWriter, r *http.Request) {
	if !h.syncPricingMu.TryLock() {
		w.WriteHeader(http.StatusConflict)
		render(r.Context(), w, syncPricingToast("Sync already in progress, please wait", toast.VariantWarning))
		return
	}
	defer h.syncPricingMu.Unlock()

	if h.DB == nil || h.Pool == nil {
		render(r.Context(), w, syncPricingToast("Database not configured", toast.VariantError))
		return
	}

	upstreamURL := os.Getenv("PRICING_UPSTREAM_URL")
	if upstreamURL == "" {
		upstreamURL = defaultUpstreamURL
	}

	openRouterURL := os.Getenv("OPENROUTER_PRICING_URL")
	if openRouterURL == "" {
		openRouterURL = "https://openrouter.ai/api/v1/models"
	}

	count, err := pricing.SyncFromUpstream(r.Context(), h.Pool, h.DB, h.Pricing, upstreamURL, openRouterURL)
	if err != nil {
		render(r.Context(), w, syncPricingToast("Sync failed: "+err.Error(), toast.VariantError))
		return
	}

	render(r.Context(), w, syncPricingToast(fmt.Sprintf("Synced %d models successfully", count), toast.VariantSuccess))
}

// handleModels renders the full Models management page.
func (h *UIHandler) handleModels(w http.ResponseWriter, r *http.Request) {
	data := h.loadModelsPageData(r)
	render(r.Context(), w, pages.ModelsPage(data))
}

// handleModelsTable renders only the table partial (HTMX swap target).
func (h *UIHandler) handleModelsTable(w http.ResponseWriter, r *http.Request) {
	data := h.loadModelsPageData(r)
	render(r.Context(), w, pages.ModelsTable(data))
}

// loadModelsPageData builds the view model for models listing.
// When DB is nil, falls back to reading from config (no pagination/search).
func (h *UIHandler) loadModelsPageData(r *http.Request) pages.ModelsPageData {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	search := r.URL.Query().Get("search")

	data := pages.ModelsPageData{
		Page:   page,
		Search: search,
	}

	if h.DB == nil {
		// Fallback: read from YAML config, no pagination or search.
		for _, m := range h.Config.ModelList {
			row := buildModelRowFromConfig(m)
			row.Source = "config"
			data.Models = append(data.Models, row)
		}
		data.TotalPages = 1
		data.DBAvailable = false
		return data
	}

	data.DBAvailable = true

	var searchPtr *string
	if search != "" {
		searchPtr = &search
	}

	totalCount, err := h.DB.CountProxyModels(r.Context(), searchPtr)
	if err != nil {
		return data
	}
	data.TotalPages = int(totalCount+int64(modelsPerPage)-1) / modelsPerPage
	if data.TotalPages < 1 {
		data.TotalPages = 1
	}

	models, err := h.DB.ListProxyModelsPage(r.Context(), db.ListProxyModelsPageParams{
		Search:     searchPtr,
		PageOffset: int32((page - 1) * modelsPerPage),
		PageLimit:  modelsPerPage,
	})
	if err != nil {
		return data
	}

	for _, m := range models {
		row := buildModelRow(m)
		row.Source = "db"
		data.Models = append(data.Models, row)
	}

	// Append YAML config models (read-only, not in DB)
	dbNames := make(map[string]bool, len(data.Models))
	for _, m := range data.Models {
		dbNames[m.ModelName] = true
	}
	for _, m := range h.Config.ModelList {
		if dbNames[m.ModelName] {
			continue
		}
		if search != "" && !strings.Contains(strings.ToLower(m.ModelName), strings.ToLower(search)) {
			continue
		}
		row := buildModelRowFromConfig(m)
		row.Source = "config"
		data.Models = append(data.Models, row)
	}

	return data
}

// handleModelCreate handles POST to create a new model.
func (h *UIHandler) handleModelCreate(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		http.Error(w, "database not configured", http.StatusServiceUnavailable)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	modelName := strings.TrimSpace(r.FormValue("model_name"))
	model := strings.TrimSpace(r.FormValue("model")) // provider/model format
	apiBase := strings.TrimSpace(r.FormValue("api_base"))
	apiKey := strings.TrimSpace(r.FormValue("api_key"))
	tpmStr := r.FormValue("tpm")
	rpmStr := r.FormValue("rpm")

	// Validate required fields.
	if modelName == "" {
		data := h.loadModelsPageData(r)
		render(r.Context(), w, pages.ModelsTableWithToast(data, "Model name is required", toast.VariantError))
		return
	}
	if !strings.Contains(model, "/") {
		data := h.loadModelsPageData(r)
		render(r.Context(), w, pages.ModelsTableWithToast(data, "Model must be in provider/model format (e.g. openai/gpt-4o)", toast.VariantError))
		return
	}

	// Check duplicate by name.
	if _, err := h.DB.GetProxyModelByName(r.Context(), modelName); err == nil {
		data := h.loadModelsPageData(r)
		render(r.Context(), w, pages.ModelsTableWithToast(data, "A model with this name already exists", toast.VariantError))
		return
	}

	// Build tianji_params JSON.
	tp := map[string]any{"model": model}
	if apiBase != "" {
		tp["api_base"] = apiBase
	}
	if apiKey != "" {
		tp["api_key"] = apiKey
	}
	tp = normalizeOfficialOpenAIUIParams(tp)
	if tpm := parseInt64(tpmStr); tpm > 0 {
		tp["tpm"] = tpm
	}
	if rpm := parseInt64(rpmStr); rpm > 0 {
		tp["rpm"] = rpm
	}

	tianjiJSON, err := json.Marshal(tp)
	if err != nil {
		data := h.loadModelsPageData(r)
		render(r.Context(), w, pages.ModelsTableWithToast(data, "Internal error: failed to encode params", toast.VariantError))
		return
	}

	modelInfoJSON := []byte("{}")

	_, err = h.DB.CreateProxyModel(r.Context(), db.CreateProxyModelParams{
		ModelID:      uuid.New().String(),
		ModelName:    modelName,
		TianjiParams: tianjiJSON,
		ModelInfo:    modelInfoJSON,
		CreatedBy:    createdByUI,
	})
	if err != nil {
		data := h.loadModelsPageData(r)
		render(r.Context(), w, pages.ModelsTableWithToast(data, "Failed to create model: "+err.Error(), toast.VariantError))
		return
	}
	if err := h.refreshRuntimeModels(r.Context()); err != nil {
		data := h.loadModelsPageData(r)
		render(r.Context(), w, pages.ModelsTableWithToast(data, "Model saved, but runtime refresh failed", toast.VariantWarning))
		return
	}

	w.Header().Set("HX-Trigger", "models-changed")
	data := h.loadModelsPageData(r)
	render(r.Context(), w, pages.ModelsTableWithToastAndClose(data, "Model created successfully", toast.VariantSuccess))
}

// handleModelEdit returns a pre-filled edit form for a model (GET /models/edit?model_id=X).
func (h *UIHandler) handleModelEdit(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		http.Error(w, "database not configured", http.StatusServiceUnavailable)
		return
	}

	modelID := r.URL.Query().Get("model_id")
	if modelID == "" {
		http.Error(w, "model_id required", http.StatusBadRequest)
		return
	}

	m, err := h.DB.GetProxyModel(r.Context(), modelID)
	if err != nil {
		http.Error(w, "model not found", http.StatusNotFound)
		return
	}

	// Build row with UNMASKED api_key for hidden form field.
	row := buildModelRowUnmasked(m)
	render(r.Context(), w, pages.EditModelForm(row))
}

// handleModelUpdate handles POST to update an existing model (model_id in form body).
func (h *UIHandler) handleModelUpdate(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		http.Error(w, "database not configured", http.StatusServiceUnavailable)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	modelID := r.FormValue("model_id")
	if modelID == "" {
		http.Error(w, "model_id required", http.StatusBadRequest)
		return
	}

	// Read existing model to merge values.
	existing, err := h.DB.GetProxyModel(r.Context(), modelID)
	if err != nil {
		http.Error(w, "model not found", http.StatusNotFound)
		return
	}

	existingTP := parseTianjiParams(existing.TianjiParams)

	modelName := strings.TrimSpace(r.FormValue("model_name"))
	model := strings.TrimSpace(r.FormValue("model"))
	apiBase := strings.TrimSpace(r.FormValue("api_base"))
	apiKey := strings.TrimSpace(r.FormValue("api_key"))
	tpmStr := r.FormValue("tpm")
	rpmStr := r.FormValue("rpm")

	// Merge editable fields while preserving the existing API key when left empty.
	if modelName == "" {
		modelName = existing.ModelName
	}
	if model != "" {
		existingTP["model"] = model
	}
	if _, ok := r.Form["api_base"]; ok {
		if apiBase != "" {
			existingTP["api_base"] = apiBase
		} else {
			delete(existingTP, "api_base")
		}
	}
	// Empty api_key means keep existing; non-empty replaces.
	if apiKey != "" {
		existingTP["api_key"] = apiKey
	}
	if tpm := parseInt64(tpmStr); tpm > 0 {
		existingTP["tpm"] = tpm
	}
	if rpm := parseInt64(rpmStr); rpm > 0 {
		existingTP["rpm"] = rpm
	}

	existingTP = normalizeOfficialOpenAIUIParams(existingTP)
	tianjiJSON, err := json.Marshal(existingTP)
	if err != nil {
		data := h.loadModelsPageData(r)
		render(r.Context(), w, pages.ModelsTableWithToast(data, "Internal error: failed to encode params", toast.VariantError))
		return
	}

	// Preserve supported model metadata and drop the removed legacy ACL field.
	var existingInfo map[string]any
	if len(existing.ModelInfo) > 0 {
		if unmarshalErr := json.Unmarshal(existing.ModelInfo, &existingInfo); unmarshalErr != nil {
			slog.Warn("failed to unmarshal existing model_info, treating as empty", "model_id", modelID, "err", unmarshalErr)
		}
	}
	if existingInfo == nil {
		existingInfo = map[string]any{}
	}
	delete(existingInfo, "access_control")
	modelInfoJSON, err := json.Marshal(existingInfo)
	if err != nil {
		data := h.loadModelsPageData(r)
		render(r.Context(), w, pages.ModelsTableWithToast(data, "Internal error: failed to encode model info", toast.VariantError))
		return
	}

	_, err = h.DB.UpdateProxyModel(r.Context(), db.UpdateProxyModelParams{
		ModelID:      modelID,
		ModelName:    modelName,
		TianjiParams: tianjiJSON,
		ModelInfo:    modelInfoJSON,
		UpdatedBy:    createdByUI,
	})
	if err != nil {
		data := h.loadModelsPageData(r)
		render(r.Context(), w, pages.ModelsTableWithToast(data, "Failed to update model: "+err.Error(), toast.VariantError))
		return
	}
	if err := h.refreshRuntimeModels(r.Context()); err != nil {
		data := h.loadModelsPageData(r)
		render(r.Context(), w, pages.ModelsTableWithToast(data, "Model saved, but runtime refresh failed", toast.VariantWarning))
		return
	}

	w.Header().Set("HX-Trigger", "models-changed")
	data := h.loadModelsPageData(r)
	render(r.Context(), w, pages.ModelsTableWithToastAndClose(data, "Model updated successfully", toast.VariantSuccess))
}

// handleModelDelete handles POST to delete a model.
func (h *UIHandler) handleModelDelete(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		http.Error(w, "database not configured", http.StatusServiceUnavailable)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	modelID := r.FormValue("model_id")
	if modelID == "" {
		http.Error(w, "model_id required", http.StatusBadRequest)
		return
	}

	if err := h.DB.DeleteProxyModel(r.Context(), modelID); err != nil {
		data := h.loadModelsPageData(r)
		render(r.Context(), w, pages.ModelsTableWithToast(data, "Failed to delete model: "+err.Error(), toast.VariantError))
		return
	}
	if err := h.refreshRuntimeModels(r.Context()); err != nil {
		data := h.loadModelsPageData(r)
		render(r.Context(), w, pages.ModelsTableWithToast(data, "Model deleted, but runtime refresh failed", toast.VariantWarning))
		return
	}

	w.Header().Set("HX-Trigger", "models-changed")
	data := h.loadModelsPageData(r)
	render(r.Context(), w, pages.ModelsTableWithToastAndClose(data, "Model deleted successfully", toast.VariantSuccess))
}

// --- helpers ---

func (h *UIHandler) refreshRuntimeModels(ctx context.Context) error {
	if h.RefreshRuntimeModels == nil {
		return nil
	}
	return h.RefreshRuntimeModels(ctx)
}

// maskAPIKey returns a truncated display: first 8 chars + "..." + last 4 chars, or "" if empty.
func maskAPIKey(key string) string {
	switch {
	case len(key) == 0:
		return ""
	case len(key) <= 12:
		return key[:min(4, len(key))] + "..." + key[len(key)-min(4, len(key)):]
	default:
		return key[:8] + "..." + key[len(key)-4:]
	}
}

// parseTianjiParams unmarshals JSONB tianji_params into a map.
func parseTianjiParams(raw []byte) map[string]any {
	m := map[string]any{}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &m)
	}
	return m
}

// normalizeOfficialOpenAIUIParams removes legacy per-model OpenAI credentials from
// model JSON while leaving generic compatibility-provider fields untouched.
func normalizeOfficialOpenAIUIParams(params map[string]any) map[string]any {
	modelName, _ := params["model"].(string)
	if strings.TrimSpace(modelName) == "" {
		if legacyModel, ok := params["Model"].(string); ok {
			modelName = legacyModel
			params["model"] = legacyModel
			delete(params, "Model")
		}
	}
	providerName, _ := provider.ParseModelName(strings.TrimSpace(modelName))
	if providerName != "openai" || strings.TrimSpace(modelName) == "" {
		return params
	}
	for _, key := range []string{
		"api_key", "api_base",
		"openai_subscription_credential_ids", "openai_subscription_transport",
		"APIKey", "APIBase",
		"OpenAISubscriptionCredentialIDs", "OpenAISubscriptionTransport",
	} {
		delete(params, key)
	}
	params["openai_subscription_transport"] = config.OpenAISubscriptionTransportChatGPTCodexBackend
	return params
}

// buildModelRow converts a DB row into the view model with masked API key.
func buildModelRow(m db.ProxyModelTable) pages.ModelRow {
	return buildModelRowFromDB(m, true)
}

// buildModelRowUnmasked is like buildModelRow but exposes the raw API key for edit forms.
func buildModelRowUnmasked(m db.ProxyModelTable) pages.ModelRow {
	return buildModelRowFromDB(m, false)
}

// buildModelRowFromDB is the shared implementation for building a ModelRow from a DB row.
// When maskKey is true, the API key is masked for display; when false, the raw key is exposed.
func buildModelRowFromDB(m db.ProxyModelTable, maskKey bool) pages.ModelRow {
	tp := normalizeOfficialOpenAIUIParams(parseTianjiParams(m.TianjiParams))

	provider, providerModel := splitProviderModel(str(tp, "model"))

	apiKey := str(tp, "api_key")
	if maskKey {
		apiKey = maskAPIKey(apiKey)
	}

	row := pages.ModelRow{
		ID:            m.ModelID,
		ModelName:     m.ModelName,
		Provider:      provider,
		ProviderModel: providerModel,
		APIBase:       str(tp, "api_base"),
		APIKey:        apiKey,
		TPM:           int64Val(tp, "tpm"),
		RPM:           int64Val(tp, "rpm"),
	}
	return row
}

// buildModelRowFromConfig converts a YAML config model entry into the view model.
func buildModelRowFromConfig(m config.ModelConfig) pages.ModelRow {
	provider, providerModel := splitProviderModel(m.TianjiParams.Model)

	var apiBase string
	if !isOfficialOpenAIUIModel(m.TianjiParams.Model) && m.TianjiParams.APIBase != nil {
		apiBase = *m.TianjiParams.APIBase
	}
	var apiKey string
	if !isOfficialOpenAIUIModel(m.TianjiParams.Model) && m.TianjiParams.APIKey != nil {
		apiKey = maskAPIKey(*m.TianjiParams.APIKey)
	}
	var tpm, rpm int64
	if m.TianjiParams.TPM != nil {
		tpm = *m.TianjiParams.TPM
	}
	if m.TianjiParams.RPM != nil {
		rpm = *m.TianjiParams.RPM
	}

	row := pages.ModelRow{
		ModelName:     m.ModelName,
		Provider:      provider,
		ProviderModel: providerModel,
		APIBase:       apiBase,
		APIKey:        apiKey,
		TPM:           tpm,
		RPM:           rpm,
	}
	return row
}

func isOfficialOpenAIUIModel(modelName string) bool {
	modelName = strings.TrimSpace(modelName)
	if modelName == "" {
		return false
	}
	providerName, _ := provider.ParseModelName(modelName)
	return providerName == "openai"
}

// splitProviderModel splits a model using the canonical provider parser.
func splitProviderModel(s string) (providerName, modelName string) {
	return provider.ParseModelName(strings.TrimSpace(s))
}

// str extracts a string value from a map.
func str(m map[string]any, key string) string {
	v, _ := m[key].(string)
	return v
}

// parseInt64 parses a string to int64, returning 0 on failure.
func parseInt64(s string) int64 {
	v, _ := strconv.ParseInt(s, 10, 64)
	return v
}

// int64Val extracts a numeric value from a map as int64.
func int64Val(m map[string]any, key string) int64 {
	switch v := m[key].(type) {
	case float64:
		return int64(v)
	case json.Number:
		n, _ := v.Int64()
		return n
	case int64:
		return v
	}
	return 0
}

// syncPricingToast returns a templ component that renders a toast notification.
// Rendered response is designed to be appended to body via HTMX (hx-swap="beforeend").
func syncPricingToast(msg string, variant toast.Variant) templ.Component {
	return toast.Toast(toast.Props{
		Title:       msg,
		Variant:     variant,
		Dismissible: true,
		Duration:    5000,
	})
}
