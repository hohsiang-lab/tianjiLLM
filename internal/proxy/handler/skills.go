package handler

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/praxisllmlab/tianjiLLM/internal/db"
	"github.com/praxisllmlab/tianjiLLM/internal/model"
)

// SkillCreate handles POST /v1/skills.
func (h *Handlers) SkillCreate(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	var req struct {
		DisplayTitle  string `json:"display_title"`
		Description   string `json:"description"`
		Instructions  string `json:"instructions"`
		Source        string `json:"source"`
		LatestVersion string `json:"latest_version"`
		FileName      string `json:"file_name"`
		FileType      string `json:"file_type"`
		FileContent   []byte `json:"file_content"`
		Metadata      []byte `json:"metadata"`
		CreatedBy     string `json:"created_by"`
	}
	if err := decodeJSON(r, &req); err != nil || req.DisplayTitle == "" {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "display_title required", Type: "invalid_request_error"},
		})
		return
	}

	if req.Metadata == nil {
		req.Metadata = []byte("{}")
	}

	skill, err := h.DB.CreateSkill(r.Context(), db.CreateSkillParams{
		DisplayTitle:  req.DisplayTitle,
		Description:   req.Description,
		Instructions:  req.Instructions,
		Source:        req.Source,
		LatestVersion: req.LatestVersion,
		FileName:      req.FileName,
		FileType:      req.FileType,
		FileContent:   req.FileContent,
		Metadata:      req.Metadata,
		CreatedBy:     req.CreatedBy,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "create skill: " + err.Error(), Type: "internal_error"},
		})
		return
	}

	writeJSON(w, http.StatusOK, skill)
}

// SkillGet handles GET /v1/skills/{skill_id}.
func (h *Handlers) SkillGet(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	skillID := chi.URLParam(r, "skill_id")
	if skillID == "" {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "skill_id required", Type: "invalid_request_error"},
		})
		return
	}

	skill, err := h.DB.GetSkill(r.Context(), skillID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "skill not found", Type: "not_found"},
		})
		return
	}

	writeJSON(w, http.StatusOK, skill)
}

// SkillList handles GET /v1/skills.
func (h *Handlers) SkillList(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	limit := int32(50)
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = int32(n)
		}
	}
	offset := int32(0)
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = int32(n)
		}
	}

	skills, err := h.DB.ListSkills(r.Context(), db.ListSkillsParams{
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "list skills: " + err.Error(), Type: "internal_error"},
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": skills})
}

// SkillDelete handles DELETE /v1/skills/{skill_id}.
func (h *Handlers) SkillDelete(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	skillID := chi.URLParam(r, "skill_id")
	if skillID == "" {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "skill_id required", Type: "invalid_request_error"},
		})
		return
	}

	if err := h.DB.DeleteSkill(r.Context(), skillID); err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "delete skill: " + err.Error(), Type: "internal_error"},
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted", "skill_id": skillID})
}
