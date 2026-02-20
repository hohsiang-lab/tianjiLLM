package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/praxisllmlab/tianjiLLM/internal/db"
	"github.com/praxisllmlab/tianjiLLM/internal/model"
)

// KeyRegenerate handles POST /key/regenerate.
func (h *Handlers) KeyRegenerate(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	var req struct {
		Key string `json:"key"`
	}
	if err := decodeJSON(r, &req); err != nil || req.Key == "" {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "key required", Type: "invalid_request_error"},
		})
		return
	}

	oldHash := hashKey(req.Key)
	newRaw := "sk-" + uuid.New().String()
	newHash := hashKey(newRaw)

	token, err := h.DB.RegenerateVerificationToken(r.Context(), db.RegenerateVerificationTokenParams{
		Token:   oldHash,
		Token_2: newHash,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "regenerate key: " + err.Error(), Type: "internal_error"},
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"key":      newRaw,
		"token":    token.Token,
		"key_name": token.KeyName,
		"expires":  token.Expires,
	})
}

// KeyBulkUpdate handles POST /key/bulk_update.
func (h *Handlers) KeyBulkUpdate(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	var req struct {
		Keys      []string `json:"keys"`
		MaxBudget *float64 `json:"max_budget"`
		TPMLimit  *int64   `json:"tpm_limit"`
		RPMLimit  *int64   `json:"rpm_limit"`
	}
	if err := decodeJSON(r, &req); err != nil || len(req.Keys) == 0 {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "keys required", Type: "invalid_request_error"},
		})
		return
	}

	hashed := make([]string, len(req.Keys))
	for i, k := range req.Keys {
		hashed[i] = hashKey(k)
	}

	if err := h.DB.BulkUpdateVerificationTokens(r.Context(), db.BulkUpdateVerificationTokensParams{
		Column1:   hashed,
		MaxBudget: req.MaxBudget,
		TpmLimit:  req.TPMLimit,
		RpmLimit:  req.RPMLimit,
	}); err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "bulk update: " + err.Error(), Type: "internal_error"},
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"status": "updated", "count": len(req.Keys)})
}

// KeyHealthCheck handles GET /key/health.
func (h *Handlers) KeyHealthCheck(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	key := r.URL.Query().Get("key")
	if key == "" {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "key parameter required", Type: "invalid_request_error"},
		})
		return
	}

	token, err := h.DB.GetVerificationToken(r.Context(), hashKey(key))
	if err != nil {
		writeJSON(w, http.StatusNotFound, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "key not found", Type: "not_found"},
		})
		return
	}

	blocked := token.Blocked != nil && *token.Blocked
	healthy := !blocked
	if token.MaxBudget != nil && token.Spend >= *token.MaxBudget {
		healthy = false
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"healthy":    healthy,
		"blocked":    blocked,
		"spend":      token.Spend,
		"max_budget": token.MaxBudget,
	})
}

// ServiceAccountKeyGenerate handles POST /key/service-account/generate.
func (h *Handlers) ServiceAccountKeyGenerate(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	var req keyGenerateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "invalid request: " + err.Error(), Type: "invalid_request_error"},
		})
		return
	}

	req.UserID = nil
	if req.TeamID == nil {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "team_id is required for service account keys", Type: "invalid_request_error"},
		})
		return
	}

	h.KeyGenerateHandler(w, r)
}

// ResetKeySpend handles POST /key/{key}/reset_spend.
func (h *Handlers) ResetKeySpend(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	keyParam := chi.URLParam(r, "key")
	if keyParam == "" {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "missing key parameter", Type: "invalid_request_error"},
		})
		return
	}

	err := h.DB.ResetVerificationTokenSpend(r.Context(), keyParam)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "reset spend: " + err.Error(), Type: "internal_error"},
		})
		return
	}

	h.createAuditLog(r.Context(), "reset_spend", "VerificationToken", keyParam, "", "", nil, map[string]any{"spend": 0})
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "key": keyParam})
}

// KeyAliases handles GET /key/aliases.
func (h *Handlers) KeyAliases(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	aliases, err := h.DB.ListDistinctKeyAliases(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "list aliases: " + err.Error(), Type: "internal_error"},
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"aliases": aliases})
}

// KeyInfoV2 handles POST /v2/key/info — batch key info lookup.
func (h *Handlers) KeyInfoV2(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	var req struct {
		Keys []string `json:"keys"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "invalid request: " + err.Error(), Type: "invalid_request_error"},
		})
		return
	}

	if len(req.Keys) == 0 {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "keys array is required", Type: "invalid_request_error"},
		})
		return
	}

	tokens, err := h.DB.GetVerificationTokenBatch(r.Context(), req.Keys)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "batch key info: " + err.Error(), Type: "internal_error"},
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": tokens})
}
