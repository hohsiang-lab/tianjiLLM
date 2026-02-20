package handler

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/praxisllmlab/tianjiLLM/internal/db"
	"github.com/praxisllmlab/tianjiLLM/internal/model"
)

type keyGenerateRequest struct {
	KeyName   *string  `json:"key_name"`
	KeyAlias  *string  `json:"key_alias"`
	MaxBudget *float64 `json:"max_budget"`
	Duration  *string  `json:"duration"`
	Models    []string `json:"models"`
	UserID    *string  `json:"user_id"`
	TeamID    *string  `json:"team_id"`
	TPMLimit  *int64   `json:"tpm_limit"`
	RPMLimit  *int64   `json:"rpm_limit"`
}

// KeyGenerateHandler handles POST /key/generate.
func (h *Handlers) KeyGenerateHandler(w http.ResponseWriter, r *http.Request) {
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

	// Generate key: sk-<uuid>
	rawKey := "sk-" + uuid.New().String()
	hashedKey := hashKey(rawKey)

	var expires *time.Time
	if req.Duration != nil {
		d, err := time.ParseDuration(*req.Duration)
		if err == nil {
			t := time.Now().Add(d)
			expires = &t
		}
	}

	var expiresTS pgtype.Timestamptz
	if expires != nil {
		expiresTS = pgtype.Timestamptz{Time: *expires, Valid: true}
	}

	token, err := h.DB.CreateVerificationToken(r.Context(), db.CreateVerificationTokenParams{
		Token:     hashedKey,
		KeyName:   req.KeyName,
		KeyAlias:  req.KeyAlias,
		MaxBudget: req.MaxBudget,
		Expires:   expiresTS,
		Models:    req.Models,
		UserID:    req.UserID,
		TeamID:    req.TeamID,
		TpmLimit:  req.TPMLimit,
		RpmLimit:  req.RPMLimit,
		Metadata:  []byte("{}"),
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "create key: " + err.Error(), Type: "internal_error"},
		})
		return
	}

	h.createAuditLog(r.Context(), "created", "VerificationToken", token.Token, "", "", nil, req)
	h.dispatchEvent(r.Context(), "key_created", token.Token, req)
	writeJSON(w, http.StatusOK, map[string]any{
		"key":        rawKey,
		"token":      token.Token,
		"key_name":   token.KeyName,
		"max_budget": token.MaxBudget,
		"expires":    token.Expires,
		"models":     token.Models,
		"user_id":    token.UserID,
		"team_id":    token.TeamID,
	})
}

// KeyInfo handles GET /key/info?key=...
func (h *Handlers) KeyInfo(w http.ResponseWriter, r *http.Request) {
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

	hashedKey := hashKey(key)
	token, err := h.DB.GetVerificationToken(r.Context(), hashedKey)
	if err != nil {
		writeJSON(w, http.StatusNotFound, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "key not found", Type: "invalid_request_error"},
		})
		return
	}

	writeJSON(w, http.StatusOK, token)
}

// KeyList handles GET /key/list
func (h *Handlers) KeyList(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	tokens, err := h.DB.ListVerificationTokens(r.Context(), db.ListVerificationTokensParams{
		Limit:  1000,
		Offset: 0,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "list keys: " + err.Error(), Type: "internal_error"},
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"keys": tokens})
}

// KeyDelete handles POST /key/delete
func (h *Handlers) KeyDelete(w http.ResponseWriter, r *http.Request) {
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

	for _, key := range req.Keys {
		if err := h.DB.DeleteVerificationToken(r.Context(), hashKey(key)); err != nil {
			writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
				Error: model.ErrorDetail{Message: fmt.Sprintf("delete key: %v", err), Type: "internal_error"},
			})
			return
		}
	}

	for _, key := range req.Keys {
		h.createAuditLog(r.Context(), "deleted", "VerificationToken", hashKey(key), "", "", nil, nil)
		h.dispatchEvent(r.Context(), "key_deleted", hashKey(key), nil)
	}
	writeJSON(w, http.StatusOK, map[string]any{"deleted_keys": req.Keys})
}

// KeyBlock handles POST /key/block
func (h *Handlers) KeyBlock(w http.ResponseWriter, r *http.Request) {
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

	if err := h.DB.BlockVerificationToken(r.Context(), hashKey(req.Key)); err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "block key: " + err.Error(), Type: "internal_error"},
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "blocked"})
}

// KeyUnblock handles POST /key/unblock
func (h *Handlers) KeyUnblock(w http.ResponseWriter, r *http.Request) {
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

	if err := h.DB.UnblockVerificationToken(r.Context(), hashKey(req.Key)); err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "unblock key: " + err.Error(), Type: "internal_error"},
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "unblocked"})
}

// KeyUpdate handles POST /key/update.
func (h *Handlers) KeyUpdate(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	var req struct {
		Key       string   `json:"key"`
		KeyName   *string  `json:"key_name"`
		KeyAlias  *string  `json:"key_alias"`
		MaxBudget *float64 `json:"max_budget"`
		Models    []string `json:"models"`
		TPMLimit  *int64   `json:"tpm_limit"`
		RPMLimit  *int64   `json:"rpm_limit"`
	}
	if err := decodeJSON(r, &req); err != nil || req.Key == "" {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "key required", Type: "invalid_request_error"},
		})
		return
	}

	if _, err := h.DB.UpdateVerificationToken(r.Context(), db.UpdateVerificationTokenParams{
		Token:     hashKey(req.Key),
		KeyName:   req.KeyName,
		KeyAlias:  req.KeyAlias,
		MaxBudget: req.MaxBudget,
		Models:    req.Models,
		TpmLimit:  req.TPMLimit,
		RpmLimit:  req.RPMLimit,
	}); err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "update key: " + err.Error(), Type: "internal_error"},
		})
		return
	}

	h.createAuditLog(r.Context(), "updated", "VerificationToken", hashKey(req.Key), "", "", nil, req)
	h.dispatchEvent(r.Context(), "key_updated", hashKey(req.Key), req)
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func hashKey(key string) string {
	h := sha256.Sum256([]byte(key))
	return hex.EncodeToString(h[:])
}
