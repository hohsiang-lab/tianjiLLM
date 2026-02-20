package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/praxisllmlab/tianjiLLM/internal/db"
	"github.com/praxisllmlab/tianjiLLM/internal/model"
)

type guardrailRequest struct {
	GuardrailName string          `json:"guardrail_name"`
	GuardrailType string          `json:"guardrail_type"`
	Config        json.RawMessage `json:"config"`
	FailurePolicy string          `json:"failure_policy"`
	Enabled       bool            `json:"enabled"`
}

// GuardrailCreate handles POST /guardrails.
func (h *Handlers) GuardrailCreate(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	var req guardrailRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "invalid JSON: " + err.Error(), Type: "invalid_request_error"},
		})
		return
	}

	result, err := h.DB.CreateGuardrailConfig(r.Context(), db.CreateGuardrailConfigParams{
		GuardrailName: req.GuardrailName,
		GuardrailType: req.GuardrailType,
		Config:        req.Config,
		FailurePolicy: req.FailurePolicy,
		Enabled:       req.Enabled,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "create guardrail: " + err.Error(), Type: "internal_error"},
		})
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

// GuardrailGet handles GET /guardrails/{id}.
func (h *Handlers) GuardrailGet(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	id := chi.URLParam(r, "id")
	result, err := h.DB.GetGuardrailConfig(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "guardrail not found", Type: "not_found"},
		})
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// GuardrailList handles GET /guardrails/list.
func (h *Handlers) GuardrailList(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	result, err := h.DB.ListGuardrailConfigs(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "list guardrails: " + err.Error(), Type: "internal_error"},
		})
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// GuardrailUpdate handles PUT /guardrails/{id}.
func (h *Handlers) GuardrailUpdate(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	id := chi.URLParam(r, "id")
	var req guardrailRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "invalid JSON: " + err.Error(), Type: "invalid_request_error"},
		})
		return
	}

	result, err := h.DB.UpdateGuardrailConfig(r.Context(), db.UpdateGuardrailConfigParams{
		ID:            id,
		GuardrailName: req.GuardrailName,
		GuardrailType: req.GuardrailType,
		Config:        req.Config,
		FailurePolicy: req.FailurePolicy,
		Enabled:       req.Enabled,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "update guardrail: " + err.Error(), Type: "internal_error"},
		})
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// GuardrailDelete handles DELETE /guardrails/{id}.
func (h *Handlers) GuardrailDelete(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	id := chi.URLParam(r, "id")
	if err := h.DB.DeleteGuardrailConfig(r.Context(), id); err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "delete guardrail: " + err.Error(), Type: "internal_error"},
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
