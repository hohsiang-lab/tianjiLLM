package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/praxisllmlab/tianjiLLM/internal/auth"
	"github.com/praxisllmlab/tianjiLLM/internal/callback"
	"github.com/praxisllmlab/tianjiLLM/internal/db"
	"github.com/praxisllmlab/tianjiLLM/internal/model"
	"github.com/praxisllmlab/tianjiLLM/internal/openaioauth"
	"github.com/praxisllmlab/tianjiLLM/internal/security/redact"
)

const CredentialTypeOpenAISubscription = "openai_subscription"

type OpenAISubscriptionTokenBundle struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	ExpiresAt    time.Time `json:"expires_at"`
	AccountID    string    `json:"account_id"`
}

type OpenAISubscriptionCredentialInfo struct {
	Email                string     `json:"email,omitempty"`
	Scopes               []string   `json:"scopes,omitempty"`
	Status               string     `json:"status,omitempty"`
	LastRefreshAt        *time.Time `json:"last_refresh_at,omitempty"`
	FirstRefreshFailedAt *time.Time `json:"first_refresh_failed_at,omitempty"`
	LastRefreshFailedAt  *time.Time `json:"last_refresh_failed_at,omitempty"`
	LastError            string     `json:"last_error,omitempty"`
	DisabledReason       string     `json:"disabled_reason,omitempty"`
	OperatorMessage      string     `json:"operator_message,omitempty"`
}

type redactedCredentialResponse struct {
	CredentialID   string         `json:"credential_id"`
	CredentialName string         `json:"credential_name"`
	CredentialType string         `json:"credential_type"`
	CredentialInfo map[string]any `json:"credential_info"`
	OrganizationID *string        `json:"organization_id"`
	CreatedAt      any            `json:"created_at"`
	UpdatedAt      any            `json:"updated_at"`
}

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
	if req.CredentialType == "" {
		req.CredentialType = "api_key"
	}

	credInfo, err := sanitizeRequestCredentialInfoJSON(req.CredentialInfo)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "invalid credential_info: " + err.Error(), Type: "invalid_request_error"},
		})
		return
	}
	if req.CredentialType == CredentialTypeOpenAISubscription {
		req.CredentialValue, err = normalizeOpenAISubscriptionCredentialValue(req.CredentialValue)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
				Error: model.ErrorDetail{Message: err.Error(), Type: "invalid_request_error"},
			})
			return
		}
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

	writeJSON(w, http.StatusOK, redactCredential(cred))
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

	redacted := make([]redactedCredentialResponse, 0, len(creds))
	for _, cred := range creds {
		redacted = append(redacted, redactCredential(cred))
	}
	writeJSON(w, http.StatusOK, map[string]any{"credentials": redacted})
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

	writeJSON(w, http.StatusOK, redactCredential(cred))
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
		CredentialID    string          `json:"credential_id"`
		CredentialType  string          `json:"credential_type"`
		CredentialValue string          `json:"credential_value"`
		CredentialInfo  json.RawMessage `json:"credential_info"`
	}
	if err := decodeJSON(r, &req); err != nil || req.CredentialID == "" || (req.CredentialValue == "" && len(req.CredentialInfo) == 0) {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "credential_id and credential_value or credential_info required", Type: "invalid_request_error"},
		})
		return
	}

	cred, err := h.DB.GetCredential(r.Context(), req.CredentialID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "credential not found", Type: "invalid_request_error"},
		})
		return
	}

	if req.CredentialType == CredentialTypeOpenAISubscription && cred.CredentialType != CredentialTypeOpenAISubscription {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "credential type mismatch", Type: "invalid_request_error"},
		})
		return
	}
	if req.CredentialType != "" && req.CredentialType != cred.CredentialType {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "credential_type cannot be changed", Type: "invalid_request_error"},
		})
		return
	}

	updated := cred
	updatedInfo := cred.CredentialInfo
	if len(req.CredentialInfo) > 0 {
		updatedInfo, err = sanitizeRequestCredentialInfoJSON(req.CredentialInfo)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
				Error: model.ErrorDetail{Message: "invalid credential_info: " + err.Error(), Type: "invalid_request_error"},
			})
			return
		}
		updated.CredentialInfo = updatedInfo
	}

	if req.CredentialValue != "" {
		if cred.CredentialType == CredentialTypeOpenAISubscription {
			req.CredentialValue, err = normalizeOpenAISubscriptionCredentialValue(req.CredentialValue)
			if err != nil {
				writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
					Error: model.ErrorDetail{Message: err.Error(), Type: "invalid_request_error"},
				})
				return
			}
		}

		masterKey := h.getMasterKey()
		encrypted, err := auth.Encrypt(req.CredentialValue, masterKey)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
				Error: model.ErrorDetail{Message: "encrypt credential: " + err.Error(), Type: "internal_error"},
			})
			return
		}
		updated.CredentialValue = encrypted

		if len(req.CredentialInfo) > 0 {
			if err := h.DB.UpdateCredentialValueAndInfo(r.Context(), db.UpdateCredentialValueAndInfoParams{
				CredentialID:    req.CredentialID,
				CredentialValue: encrypted,
				CredentialInfo:  updatedInfo,
			}); err != nil {
				writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
					Error: model.ErrorDetail{Message: "update credential: " + err.Error(), Type: "internal_error"},
				})
				return
			}
		} else if err := h.DB.UpdateCredential(r.Context(), db.UpdateCredentialParams{
			CredentialID:    req.CredentialID,
			CredentialValue: encrypted,
		}); err != nil {
			writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
				Error: model.ErrorDetail{Message: "update credential: " + err.Error(), Type: "internal_error"},
			})
			return
		}
	} else if err := h.DB.UpdateCredentialInfo(r.Context(), db.UpdateCredentialInfoParams{
		CredentialID:   req.CredentialID,
		CredentialInfo: updatedInfo,
	}); err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "update credential: " + err.Error(), Type: "internal_error"},
		})
		return
	}

	if cred.CredentialType == CredentialTypeOpenAISubscription {
		h.invalidateOpenAISubscriptionCodexCatalog(r.Context(), req.CredentialID)
	}

	writeJSON(w, http.StatusOK, redactCredential(updated))
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

	cred, getErr := h.DB.GetCredential(r.Context(), credID)
	if err := h.DB.DeleteCredential(r.Context(), credID); err != nil {
		if getErr == nil && cred.CredentialType == CredentialTypeOpenAISubscription {
			h.auditOpenAISubscriptionLifecycle(r.Context(), callback.OpenAISubscriptionAttribution{
				CredentialID:   credID,
				Provider:       "openai",
				OrganizationID: openAISubscriptionCredentialOrgID(cred.OrganizationID),
				Action:         "delete",
				Status:         "failure",
				ReasonCode:     "delete_failed",
				Metadata:       map[string]any{"error": err.Error()},
			})
		}
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "delete credential: " + err.Error(), Type: "internal_error"},
		})
		return
	}
	if getErr == nil && cred.CredentialType == CredentialTypeOpenAISubscription {
		h.invalidateOpenAISubscriptionCodexCatalog(r.Context(), credID)
		h.auditOpenAISubscriptionLifecycle(r.Context(), callback.OpenAISubscriptionAttribution{
			CredentialID:   credID,
			Provider:       "openai",
			OrganizationID: openAISubscriptionCredentialOrgID(cred.OrganizationID),
			Action:         "delete",
			Status:         "success",
		})
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

func normalizeOpenAISubscriptionCredentialValue(raw string) (string, error) {
	var bundle OpenAISubscriptionTokenBundle
	if err := json.Unmarshal([]byte(raw), &bundle); err != nil {
		return "", fmt.Errorf("invalid openai subscription credential_value: %w", err)
	}
	if err := validateOpenAITokenBundle(bundle, true); err != nil {
		return "", err
	}
	normalizedBundle, err := json.Marshal(bundle)
	if err != nil {
		return "", fmt.Errorf("marshal openai subscription credential_value: %w", err)
	}
	return string(normalizedBundle), nil
}

func (h *Handlers) SaveOpenAISubscriptionCredential(ctx context.Context, name string, bundle OpenAISubscriptionTokenBundle, info OpenAISubscriptionCredentialInfo, organizationID *string) (db.CredentialTable, error) {
	return h.saveOpenAISubscriptionCredential(ctx, h.DB, uuid.New().String(), name, bundle, info, organizationID)
}

// SaveOpenAISubscriptionCredentialForFlow consumes the caller's attempt exactly
// once. The returned owner is usable for completion, never a new admission.
func (h *Handlers) SaveOpenAISubscriptionCredentialForFlow(ctx context.Context, expected *openaioauth.DeviceAuthRecord, name string, bundle OpenAISubscriptionTokenBundle, info OpenAISubscriptionCredentialInfo) (db.CredentialTable, error) {
	if expected == nil || expected.FlowID == "" {
		return db.CredentialTable{}, openaioauth.ErrDeviceAuthInvalid
	}
	store := openaioauth.NewDeviceStore(h.Cache)
	var credential db.CredentialTable
	err := h.withDeviceAuthTx(ctx, expected.FlowID, func(ctx context.Context, q *db.Queries) error {
		current, err := store.Get(ctx, expected.FlowID)
		if err != nil && !errors.Is(err, openaioauth.ErrDeviceAuthExpired) {
			return err
		}
		if !sameDeviceAuthIdentity(current, *expected) {
			return openaioauth.ErrDeviceAuthOwnership
		}
		existing, found, err := deviceAuthCredential(ctx, q, current)
		if err != nil {
			return err
		}
		if found {
			if !strings.HasPrefix(current.Generation, "admitted:") {
				return openaioauth.ErrDeviceAuthInvalid
			}
			credential = existing
			return nil
		}
		now := time.Now().UTC()
		if current.Terminal() || current.Status != openaioauth.DeviceAuthStatusExchanging || expected.Status != openaioauth.DeviceAuthStatusExchanging ||
			current.Generation != expected.Generation || !strings.HasPrefix(expected.Generation, "attempt:") ||
			!now.Before(current.ExpiresAt) || !now.Before(current.UpdatedAt.Add(deviceAuthExchangeLease)) {
			return openaioauth.ErrDeviceAuthOwnership
		}
		admitted := current
		admitted.Generation = "admitted:" + uuid.NewString()
		applied, err := store.SaveIfCurrent(ctx, current, admitted)
		if err != nil {
			return err
		}
		if !applied {
			return openaioauth.ErrDeviceAuthOwnership
		}
		// No retry may adopt this generation after transaction loss or row deletion.
		// A delayed CAS can succeed after PG disconnects, but only THIS tx may INSERT.
		*expected = admitted
		var orgID *string
		if current.OrgID != "" {
			orgID = &current.OrgID
		}
		credential, err = h.saveOpenAISubscriptionCredential(ctx, q, openAIDeviceCredentialID(current.FlowID), name, bundle, info, orgID)
		return err
	})
	if err != nil {
		return db.CredentialTable{}, err
	}
	return credential, nil
}

func (h *Handlers) saveOpenAISubscriptionCredential(ctx context.Context, writer db.Store, credentialID, name string, bundle OpenAISubscriptionTokenBundle, info OpenAISubscriptionCredentialInfo, organizationID *string) (db.CredentialTable, error) {
	if h.DB == nil {
		return db.CredentialTable{}, errors.New("database not configured")
	}
	if credentialID == "" {
		return db.CredentialTable{}, errors.New("credential_id required")
	}
	if name == "" {
		return db.CredentialTable{}, errors.New("credential_name required")
	}
	if err := validateOpenAITokenBundle(bundle, true); err != nil {
		return db.CredentialTable{}, err
	}
	encrypted, err := h.encryptOpenAITokenBundle(bundle)
	if err != nil {
		return db.CredentialTable{}, err
	}
	infoJSON, err := marshalSafeCredentialInfo(info)
	if err != nil {
		return db.CredentialTable{}, err
	}
	return writer.CreateCredential(ctx, db.CreateCredentialParams{
		CredentialID:    credentialID,
		CredentialName:  name,
		CredentialType:  CredentialTypeOpenAISubscription,
		CredentialValue: encrypted,
		CredentialInfo:  infoJSON,
		OrganizationID:  organizationID,
	})
}

func (h *Handlers) UpdateOpenAISubscriptionCredential(ctx context.Context, credentialID string, refreshed OpenAISubscriptionTokenBundle, info OpenAISubscriptionCredentialInfo) error {
	if h.DB == nil {
		return errors.New("database not configured")
	}
	if credentialID == "" {
		return errors.New("credential_id required")
	}
	if err := validateOpenAITokenBundle(refreshed, false); err != nil {
		return err
	}

	existing, err := h.getOpenAISubscriptionCredential(ctx, credentialID)
	if err != nil {
		return err
	}
	if refreshed.RefreshToken == "" {
		var plaintext string
		plaintext, err = auth.Decrypt(existing.CredentialValue, h.getMasterKey())
		if err != nil {
			return fmt.Errorf("decrypt existing credential: %w", err)
		}
		var existingBundle OpenAISubscriptionTokenBundle
		err = json.Unmarshal([]byte(plaintext), &existingBundle)
		if err != nil {
			return fmt.Errorf("parse existing credential bundle: %w", err)
		}
		refreshed.RefreshToken = existingBundle.RefreshToken
	}
	err = validateOpenAITokenBundle(refreshed, true)
	if err != nil {
		return err
	}

	encrypted, err := h.encryptOpenAITokenBundle(refreshed)
	if err != nil {
		return err
	}
	infoJSON, err := marshalSafeCredentialInfo(info)
	if err != nil {
		return err
	}
	if err := h.DB.UpdateCredentialValueAndInfo(ctx, db.UpdateCredentialValueAndInfoParams{
		CredentialID:    credentialID,
		CredentialValue: encrypted,
		CredentialInfo:  infoJSON,
	}); err != nil {
		return err
	}
	h.invalidateOpenAISubscriptionCodexCatalog(ctx, credentialID)
	return nil
}

func (h *Handlers) UpdateOpenAISubscriptionCredentialFailure(ctx context.Context, credentialID string, info OpenAISubscriptionCredentialInfo) error {
	if h.DB == nil {
		return errors.New("database not configured")
	}
	if credentialID == "" {
		return errors.New("credential_id required")
	}
	if _, err := h.getOpenAISubscriptionCredential(ctx, credentialID); err != nil {
		return err
	}
	infoJSON, err := marshalSafeCredentialInfo(info)
	if err != nil {
		return err
	}
	if err := h.DB.UpdateCredentialInfo(ctx, db.UpdateCredentialInfoParams{
		CredentialID:   credentialID,
		CredentialInfo: infoJSON,
	}); err != nil {
		return err
	}
	h.invalidateOpenAISubscriptionCodexCatalog(ctx, credentialID)
	return nil
}

func (h *Handlers) getOpenAISubscriptionCredential(ctx context.Context, credentialID string) (db.CredentialTable, error) {
	existing, err := h.DB.GetCredential(ctx, credentialID)
	if err != nil {
		return db.CredentialTable{}, fmt.Errorf("get existing credential: %w", err)
	}
	if existing.CredentialType != CredentialTypeOpenAISubscription {
		return db.CredentialTable{}, fmt.Errorf("credential %s is not %s", credentialID, CredentialTypeOpenAISubscription)
	}
	return existing, nil
}

func (h *Handlers) encryptOpenAITokenBundle(bundle OpenAISubscriptionTokenBundle) (string, error) {
	plaintext, err := json.Marshal(bundle)
	if err != nil {
		return "", fmt.Errorf("marshal openai token bundle: %w", err)
	}
	encrypted, err := auth.Encrypt(string(plaintext), h.getMasterKey())
	if err != nil {
		return "", fmt.Errorf("encrypt openai token bundle: %w", err)
	}
	return encrypted, nil
}

func validateOpenAITokenBundle(bundle OpenAISubscriptionTokenBundle, requireRefreshToken bool) error {
	switch {
	case bundle.AccessToken == "":
		return errors.New("access_token required")
	case requireRefreshToken && bundle.RefreshToken == "":
		return errors.New("refresh_token required")
	case bundle.ExpiresAt.IsZero():
		return errors.New("expires_at required")
	case bundle.AccountID == "":
		return errors.New("account_id required")
	default:
		return nil
	}
}

func marshalSafeCredentialInfo(info OpenAISubscriptionCredentialInfo) ([]byte, error) {
	payload, err := json.Marshal(info)
	if err != nil {
		return nil, fmt.Errorf("marshal credential_info: %w", err)
	}
	return sanitizeCredentialInfoJSON(payload)
}

func sanitizeCredentialInfoJSON(raw json.RawMessage) ([]byte, error) {
	if len(raw) == 0 {
		return []byte("{}"), nil
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}
	if key, ok := findSecretCredentialInfoField(payload); ok {
		return nil, fmt.Errorf("%s is secret material and cannot be stored in credential_info", key)
	}
	return json.Marshal(redact.JSONValue(payload))
}

func sanitizeRequestCredentialInfoJSON(raw json.RawMessage) ([]byte, error) {
	if len(raw) == 0 {
		return []byte("{}"), nil
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}
	if key, ok := findSecretCredentialInfoField(payload); ok {
		return nil, fmt.Errorf("%s is secret material and cannot be stored in credential_info", key)
	}
	if secret, ok := findSecretCredentialInfoValue(payload); ok {
		return nil, fmt.Errorf("%s is secret material and cannot be stored in credential_info", secret)
	}
	return json.Marshal(redact.JSONValue(payload))
}

func redactCredential(cred db.CredentialTable) redactedCredentialResponse {
	info := map[string]any{}
	if len(cred.CredentialInfo) > 0 {
		_ = json.Unmarshal(cred.CredentialInfo, &info)
	}
	redactCredentialInfoFields(info)
	return redactedCredentialResponse{
		CredentialID:   cred.CredentialID,
		CredentialName: cred.CredentialName,
		CredentialType: cred.CredentialType,
		CredentialInfo: info,
		OrganizationID: cred.OrganizationID,
		CreatedAt:      cred.CreatedAt,
		UpdatedAt:      cred.UpdatedAt,
	}
}

func findSecretCredentialInfoField(value any) (string, bool) {
	switch typed := value.(type) {
	case map[string]any:
		for key, nested := range typed {
			if redact.IsSecretFieldName(key) {
				return key, true
			}
			if found, ok := findSecretCredentialInfoField(nested); ok {
				return found, true
			}
		}
	case []any:
		for _, item := range typed {
			if found, ok := findSecretCredentialInfoField(item); ok {
				return found, true
			}
		}
	}
	return "", false
}

func findSecretCredentialInfoValue(value any) (string, bool) {
	switch typed := value.(type) {
	case map[string]any:
		for _, nested := range typed {
			if found, ok := findSecretCredentialInfoValue(nested); ok {
				return found, true
			}
		}
	case []any:
		for _, item := range typed {
			if found, ok := findSecretCredentialInfoValue(item); ok {
				return found, true
			}
		}
	case string:
		if typed != redact.String(typed) {
			return "secret-looking value", true
		}
	}
	return "", false
}

func redactCredentialInfoFields(value any) {
	switch typed := value.(type) {
	case map[string]any:
		for key, nested := range typed {
			if redact.IsSecretFieldName(key) {
				delete(typed, key)
				continue
			}
			if str, ok := nested.(string); ok {
				typed[key] = redact.String(str)
				continue
			}
			redactCredentialInfoFields(nested)
		}
	case []any:
		for i, item := range typed {
			if str, ok := item.(string); ok {
				typed[i] = redact.String(str)
				continue
			}
			redactCredentialInfoFields(item)
		}
	}
}
