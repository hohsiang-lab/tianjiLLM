package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/praxisllmlab/tianjiLLM/internal/db"
	"github.com/praxisllmlab/tianjiLLM/internal/model"
)

// ModelNew handles POST /model/new.
func (h *Handlers) ModelNew(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	var req struct {
		ModelID      string `json:"model_id"`
		ModelName    string `json:"model_name"`
		TianjiParams []byte `json:"tianji_params"`
		ModelInfo    []byte `json:"model_info"`
		CreatedBy    string `json:"created_by"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "invalid request: " + err.Error(), Type: "invalid_request_error"},
		})
		return
	}

	result, err := h.DB.CreateProxyModel(r.Context(), db.CreateProxyModelParams{
		ModelID:      req.ModelID,
		ModelName:    req.ModelName,
		TianjiParams: req.TianjiParams,
		ModelInfo:    req.ModelInfo,
		CreatedBy:    req.CreatedBy,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "create model: " + err.Error(), Type: "internal_error"},
		})
		return
	}

	writeJSON(w, http.StatusCreated, result)
}

// ModelInfo handles GET /model/info.
func (h *Handlers) ModelInfo(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	modelID := r.URL.Query().Get("model_id")
	if modelID != "" {
		result, err := h.DB.GetProxyModel(r.Context(), modelID)
		if err != nil {
			writeJSON(w, http.StatusNotFound, model.ErrorResponse{
				Error: model.ErrorDetail{Message: "model not found", Type: "not_found"},
			})
			return
		}
		writeJSON(w, http.StatusOK, result)
		return
	}

	result, err := h.DB.ListProxyModels(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "list models: " + err.Error(), Type: "internal_error"},
		})
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// ModelUpdate handles POST /model/update.
func (h *Handlers) ModelUpdate(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	var req struct {
		ModelID      string `json:"model_id"`
		ModelName    string `json:"model_name"`
		TianjiParams []byte `json:"tianji_params"`
		ModelInfo    []byte `json:"model_info"`
		UpdatedBy    string `json:"updated_by"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "invalid request: " + err.Error(), Type: "invalid_request_error"},
		})
		return
	}

	result, err := h.DB.UpdateProxyModel(r.Context(), db.UpdateProxyModelParams{
		ModelID:      req.ModelID,
		ModelName:    req.ModelName,
		TianjiParams: req.TianjiParams,
		ModelInfo:    req.ModelInfo,
		UpdatedBy:    req.UpdatedBy,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "update model: " + err.Error(), Type: "internal_error"},
		})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// ModelDelete handles POST /model/delete.
func (h *Handlers) ModelDelete(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	modelID := chi.URLParam(r, "model_id")
	if modelID == "" {
		var req struct {
			ModelID string `json:"model_id"`
		}
		if err := decodeJSON(r, &req); err == nil {
			modelID = req.ModelID
		}
	}

	if err := h.DB.DeleteProxyModel(r.Context(), modelID); err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "delete model: " + err.Error(), Type: "internal_error"},
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
