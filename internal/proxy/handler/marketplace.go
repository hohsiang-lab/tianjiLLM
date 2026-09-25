package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/praxisllmlab/tianjiLLM/internal/db"
	"github.com/praxisllmlab/tianjiLLM/internal/model"
)

// MarketplaceJSON handles GET /claude-code/marketplace.json (public, no auth).
func (h *Handlers) MarketplaceJSON(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	plugins, err := h.DB.ListEnabledPlugins(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "list plugins: " + err.Error(), Type: "internal_error"},
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"plugins": plugins})
}

// PluginCreate handles POST /claude-code/plugins.
func (h *Handlers) PluginCreate(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	var req struct {
		Name        string `json:"name"`
		Version     string `json:"version"`
		Description string `json:"description"`
		Manifest    any    `json:"manifest"`
		Files       any    `json:"files"`
		Source      string `json:"source"`
		SourceURL   string `json:"source_url"`
		CreatedBy   string `json:"created_by"`
	}
	if err := decodeJSON(r, &req); err != nil || req.Name == "" {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "name required", Type: "invalid_request_error"},
		})
		return
	}

	manifestJSON, _ := json.Marshal(req.Manifest)
	filesJSON, _ := json.Marshal(req.Files)

	plugin, err := h.DB.CreatePlugin(r.Context(), db.CreatePluginParams{
		Name:         req.Name,
		Version:      req.Version,
		Description:  req.Description,
		ManifestJson: manifestJSON,
		FilesJson:    filesJSON,
		Source:       req.Source,
		SourceUrl:    req.SourceURL,
		CreatedBy:    req.CreatedBy,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "create plugin: " + err.Error(), Type: "internal_error"},
		})
		return
	}

	writeJSON(w, http.StatusOK, plugin)
}

// PluginGet handles GET /claude-code/plugins/{name}.
func (h *Handlers) PluginGet(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	name := chi.URLParam(r, "name")
	plugin, err := h.DB.GetPlugin(r.Context(), name)
	if err != nil {
		writeJSON(w, http.StatusNotFound, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "plugin not found", Type: "not_found"},
		})
		return
	}

	writeJSON(w, http.StatusOK, plugin)
}

// PluginList handles GET /claude-code/plugins.
func (h *Handlers) PluginList(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	plugins, err := h.DB.ListPlugins(r.Context(), db.ListPluginsParams{
		Limit:  100,
		Offset: 0,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "list plugins: " + err.Error(), Type: "internal_error"},
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": plugins})
}

// PluginEnable handles POST /claude-code/plugins/{name}/enable.
func (h *Handlers) PluginEnable(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	name := chi.URLParam(r, "name")
	if err := h.DB.EnablePlugin(r.Context(), name); err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "enable plugin: " + err.Error(), Type: "internal_error"},
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "enabled", "name": name})
}

// PluginDisable handles POST /claude-code/plugins/{name}/disable.
func (h *Handlers) PluginDisable(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	name := chi.URLParam(r, "name")
	if err := h.DB.DisablePlugin(r.Context(), name); err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "disable plugin: " + err.Error(), Type: "internal_error"},
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "disabled", "name": name})
}

// PluginDelete handles DELETE /claude-code/plugins/{name}.
func (h *Handlers) PluginDelete(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	name := chi.URLParam(r, "name")
	if err := h.DB.DeletePlugin(r.Context(), name); err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "delete plugin: " + err.Error(), Type: "internal_error"},
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted", "name": name})
}
