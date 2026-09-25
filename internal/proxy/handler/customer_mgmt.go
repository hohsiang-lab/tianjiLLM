package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/praxisllmlab/tianjiLLM/internal/db"
	"github.com/praxisllmlab/tianjiLLM/internal/model"
)

// EndUserNew handles POST /end_user/new.
func (h *Handlers) EndUserNew(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	var req struct {
		EndUserID          string   `json:"end_user_id"`
		Alias              *string  `json:"alias"`
		AllowedModelRegion *string  `json:"allowed_model_region"`
		DefaultModel       *string  `json:"default_model"`
		Budget             *float64 `json:"budget"`
		Blocked            bool     `json:"blocked"`
		Metadata           []byte   `json:"metadata"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "invalid request: " + err.Error(), Type: "invalid_request_error"},
		})
		return
	}

	result, err := h.DB.CreateEndUser(r.Context(), db.CreateEndUserParams{
		EndUserID:          req.EndUserID,
		Alias:              req.Alias,
		AllowedModelRegion: req.AllowedModelRegion,
		DefaultModel:       req.DefaultModel,
		Budget:             req.Budget,
		Blocked:            req.Blocked,
		Metadata:           req.Metadata,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "create end user: " + err.Error(), Type: "internal_error"},
		})
		return
	}

	writeJSON(w, http.StatusCreated, result)
}

// EndUserInfo handles GET /end_user/info/{id}.
func (h *Handlers) EndUserInfo(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	id := chi.URLParam(r, "id")
	result, err := h.DB.GetEndUser(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "end user not found", Type: "not_found"},
		})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// EndUserList handles GET /end_user/list.
func (h *Handlers) EndUserList(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	result, err := h.DB.ListEndUsers(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "list end users: " + err.Error(), Type: "internal_error"},
		})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// EndUserUpdate handles POST /end_user/update.
func (h *Handlers) EndUserUpdate(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	var req struct {
		ID                 string   `json:"id"`
		Alias              *string  `json:"alias"`
		AllowedModelRegion *string  `json:"allowed_model_region"`
		DefaultModel       *string  `json:"default_model"`
		Budget             *float64 `json:"budget"`
		Metadata           []byte   `json:"metadata"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "invalid request: " + err.Error(), Type: "invalid_request_error"},
		})
		return
	}

	result, err := h.DB.UpdateEndUser(r.Context(), db.UpdateEndUserParams{
		ID:                 req.ID,
		Alias:              req.Alias,
		AllowedModelRegion: req.AllowedModelRegion,
		DefaultModel:       req.DefaultModel,
		Budget:             req.Budget,
		Metadata:           req.Metadata,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "update end user: " + err.Error(), Type: "internal_error"},
		})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// EndUserDelete handles POST /end_user/delete.
func (h *Handlers) EndUserDelete(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		var req struct {
			ID string `json:"id"`
		}
		if err := decodeJSON(r, &req); err == nil {
			id = req.ID
		}
	}

	if err := h.DB.DeleteEndUser(r.Context(), id); err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "delete end user: " + err.Error(), Type: "internal_error"},
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// EndUserBlock handles POST /end_user/block.
func (h *Handlers) EndUserBlock(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	var req struct {
		ID string `json:"id"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "invalid request: " + err.Error(), Type: "invalid_request_error"},
		})
		return
	}

	result, err := h.DB.BlockEndUser(r.Context(), req.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "block end user: " + err.Error(), Type: "internal_error"},
		})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// EndUserUnblock handles POST /end_user/unblock.
func (h *Handlers) EndUserUnblock(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	var req struct {
		ID string `json:"id"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "invalid request: " + err.Error(), Type: "invalid_request_error"},
		})
		return
	}

	result, err := h.DB.UnblockEndUser(r.Context(), req.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "unblock end user: " + err.Error(), Type: "internal_error"},
		})
		return
	}

	writeJSON(w, http.StatusOK, result)
}
