package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/praxisllmlab/tianjiLLM/internal/db"
	"github.com/praxisllmlab/tianjiLLM/internal/model"
)

type promptRequest struct {
	Name      string          `json:"name"`
	Template  string          `json:"template"`
	Variables []string        `json:"variables"`
	Model     *string         `json:"model,omitempty"`
	Metadata  json.RawMessage `json:"metadata,omitempty"`
}

// PromptCreate handles POST /prompts — auto-increments version per name.
func (h *Handlers) PromptCreate(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	var req promptRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "invalid JSON: " + err.Error(), Type: "invalid_request_error"},
		})
		return
	}

	nextVersion, err := h.DB.GetNextPromptVersion(r.Context(), req.Name)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "get next version: " + err.Error(), Type: "internal_error"},
		})
		return
	}

	result, err := h.DB.CreatePromptTemplate(r.Context(), db.CreatePromptTemplateParams{
		Name:      req.Name,
		Version:   nextVersion,
		Template:  req.Template,
		Variables: req.Variables,
		Model:     req.Model,
		Metadata:  req.Metadata,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "create prompt: " + err.Error(), Type: "internal_error"},
		})
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

// PromptGet handles GET /prompts/{id}.
func (h *Handlers) PromptGet(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	id := chi.URLParam(r, "id")
	result, err := h.DB.GetPromptTemplate(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "prompt not found", Type: "not_found"},
		})
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// PromptList handles GET /prompts.
func (h *Handlers) PromptList(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	result, err := h.DB.ListPromptTemplates(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "list prompts: " + err.Error(), Type: "internal_error"},
		})
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// PromptUpdate handles PUT /prompts/{id} — creates a new version.
func (h *Handlers) PromptUpdate(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	id := chi.URLParam(r, "id")
	existing, err := h.DB.GetPromptTemplate(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "prompt not found", Type: "not_found"},
		})
		return
	}

	var req promptRequest
	if err = decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "invalid JSON: " + err.Error(), Type: "invalid_request_error"},
		})
		return
	}

	nextVersion, err := h.DB.GetNextPromptVersion(r.Context(), existing.Name)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "get next version: " + err.Error(), Type: "internal_error"},
		})
		return
	}

	tmpl := req.Template
	if tmpl == "" {
		tmpl = existing.Template
	}
	vars := req.Variables
	if vars == nil {
		vars = existing.Variables
	}

	result, err := h.DB.CreatePromptTemplate(r.Context(), db.CreatePromptTemplateParams{
		Name:      existing.Name,
		Version:   nextVersion,
		Template:  tmpl,
		Variables: vars,
		Model:     req.Model,
		Metadata:  req.Metadata,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "create prompt version: " + err.Error(), Type: "internal_error"},
		})
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// PromptDelete handles DELETE /prompts/{id}.
func (h *Handlers) PromptDelete(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	id := chi.URLParam(r, "id")
	if err := h.DB.DeletePromptTemplate(r.Context(), id); err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "delete prompt: " + err.Error(), Type: "internal_error"},
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// PromptVersions handles GET /prompts/{id}/versions.
func (h *Handlers) PromptVersions(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	id := chi.URLParam(r, "id")
	// First get the prompt to find its name
	prompt, err := h.DB.GetPromptTemplate(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "prompt not found", Type: "not_found"},
		})
		return
	}

	versions, err := h.DB.GetPromptVersions(r.Context(), prompt.Name)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "list versions: " + err.Error(), Type: "internal_error"},
		})
		return
	}
	writeJSON(w, http.StatusOK, versions)
}

// PromptTest handles POST /prompts/test — resolves template variables without calling LLM.
func (h *Handlers) PromptTest(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Template  string            `json:"template"`
		Variables map[string]string `json:"variables"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "invalid JSON: " + err.Error(), Type: "invalid_request_error"},
		})
		return
	}

	resolved := req.Template
	for k, v := range req.Variables {
		resolved = strings.ReplaceAll(resolved, "{{"+k+"}}", v)
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"resolved": resolved,
	})
}
