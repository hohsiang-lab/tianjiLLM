package handler

import (
	"net/http"

	"github.com/praxisllmlab/tianjiLLM/internal/db"
	"github.com/praxisllmlab/tianjiLLM/internal/model"
)

// IPAdd handles POST /ip/add.
func (h *Handlers) IPAdd(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	var req struct {
		IPAddress   string `json:"ip_address"`
		Description string `json:"description"`
		CreatedBy   string `json:"created_by"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "invalid JSON: " + err.Error(), Type: "invalid_request_error"},
		})
		return
	}

	result, err := h.DB.CreateIPWhitelist(r.Context(), db.CreateIPWhitelistParams{
		IpAddress:   req.IPAddress,
		Description: &req.Description,
		CreatedBy:   req.CreatedBy,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "add IP: " + err.Error(), Type: "internal_error"},
		})
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

// IPDelete handles DELETE /ip/delete.
func (h *Handlers) IPDelete(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	var req struct {
		IPAddress string `json:"ip_address"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "invalid JSON: " + err.Error(), Type: "invalid_request_error"},
		})
		return
	}

	if err := h.DB.DeleteIPWhitelistByAddress(r.Context(), req.IPAddress); err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "delete IP: " + err.Error(), Type: "internal_error"},
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// IPList handles GET /ip/list.
func (h *Handlers) IPList(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	result, err := h.DB.ListIPWhitelist(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "list IPs: " + err.Error(), Type: "internal_error"},
		})
		return
	}
	writeJSON(w, http.StatusOK, result)
}
