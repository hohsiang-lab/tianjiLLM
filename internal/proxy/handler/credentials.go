package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/praxisllmlab/tianjiLLM/internal/auth"
	"github.com/praxisllmlab/tianjiLLM/internal/db"
	"github.com/praxisllmlab/tianjiLLM/internal/model"
)

// CredentialNew handles POST /credentials/new.
func (h *Handlers) CredentialNew(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	var req struct {
		CredentialName  string          `json:"credential_name"`
		CredentialType  string          `json:"credential_type"`
		CredentialValue string          `json:"credential_value"`
		CredentialInfo  json.RawMessage `json:"credential_info"`
		OrganizationID  *string         `json:"organization_id"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "invalid request: " + err.Error(), Type: "invalid_request_error"},
		})
		return
	}
	if req.CredentialName == "" || req.CredentialValue == "" {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "credential_name and credential_value required", Type: "invalid_request_error"},
		})
		return
	}

	// Encrypt the credential value using NaCl SecretBox
	masterKey := h.getMasterKey()
	encrypted, err := auth.Encrypt(req.CredentialValue, masterKey)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "encrypt credential: " + err.Error(), Type: "internal_error"},
		})
		return
	}

	credInfo := []byte("{}")
	if req.CredentialInfo != nil {
		credInfo = []byte(req.CredentialInfo)
	}

	cred, err := h.DB.CreateCredential(r.Context(), db.CreateCredentialParams{
		CredentialID:    uuid.New().String(),
		CredentialName:  req.CredentialName,
		CredentialType:  req.CredentialType,
		CredentialValue: encrypted,
		CredentialInfo:  credInfo,
		OrganizationID:  req.OrganizationID,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "create credential: " + err.Error(), Type: "internal_error"},
		})
		return
	}

	// Don't return the encrypted value in the response
	writeJSON(w, http.StatusOK, map[string]any{
		"credential_id":   cred.CredentialID,
		"credential_name": cred.CredentialName,
		"credential_type": cred.CredentialType,
		"organization_id": cred.OrganizationID,
		"created_at":      cred.CreatedAt,
	})
}

// CredentialList handles GET /credentials/list.
func (h *Handlers) CredentialList(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	var creds []db.CredentialTable
	var err error
	if q := r.URL.Query().Get("organization_id"); q != "" {
		creds, err = h.DB.ListCredentialsByOrg(r.Context(), &q)
	} else {
		creds, err = h.DB.ListCredentials(r.Context())
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "list credentials: " + err.Error(), Type: "internal_error"},
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"credentials": creds})
}

// CredentialInfo handles GET /credentials/info/{credential_id}.
func (h *Handlers) CredentialInfo(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	credID := chi.URLParam(r, "credential_id")
	if credID == "" {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "credential_id required", Type: "invalid_request_error"},
		})
		return
	}

	cred, err := h.DB.GetCredential(r.Context(), credID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "credential not found", Type: "invalid_request_error"},
		})
		return
	}

	// Don't expose encrypted value — return metadata only
	writeJSON(w, http.StatusOK, map[string]any{
		"credential_id":   cred.CredentialID,
		"credential_name": cred.CredentialName,
		"credential_type": cred.CredentialType,
		"organization_id": cred.OrganizationID,
		"created_at":      cred.CreatedAt,
	})
}

// CredentialUpdate handles POST /credentials/update.
func (h *Handlers) CredentialUpdate(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	var req struct {
		CredentialID    string `json:"credential_id"`
		CredentialValue string `json:"credential_value"`
	}
	if err := decodeJSON(r, &req); err != nil || req.CredentialID == "" || req.CredentialValue == "" {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "credential_id and credential_value required", Type: "invalid_request_error"},
		})
		return
	}

	masterKey := h.getMasterKey()
	encrypted, err := auth.Encrypt(req.CredentialValue, masterKey)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "encrypt credential: " + err.Error(), Type: "internal_error"},
		})
		return
	}

	if err := h.DB.UpdateCredential(r.Context(), db.UpdateCredentialParams{
		CredentialID:    req.CredentialID,
		CredentialValue: encrypted,
	}); err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "update credential: " + err.Error(), Type: "internal_error"},
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "updated", "credential_id": req.CredentialID})
}

// CredentialDelete handles DELETE /credentials/delete/{credential_id}.
func (h *Handlers) CredentialDelete(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	credID := chi.URLParam(r, "credential_id")
	if credID == "" {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "credential_id required", Type: "invalid_request_error"},
		})
		return
	}

	if err := h.DB.DeleteCredential(r.Context(), credID); err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "delete credential: " + err.Error(), Type: "internal_error"},
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted", "credential_id": credID})
}

// getMasterKey returns the master key from config for encryption.
func (h *Handlers) getMasterKey() string {
	if h.Config != nil && h.Config.GeneralSettings.MasterKey != "" {
		return h.Config.GeneralSettings.MasterKey
	}
	return ""
}
