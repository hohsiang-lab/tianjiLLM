package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/praxisllmlab/tianjiLLM/internal/callback"
	"github.com/praxisllmlab/tianjiLLM/internal/db"
	"github.com/praxisllmlab/tianjiLLM/internal/security/redact"
)

const (
	openAISubscriptionLifecycleActionTest    = "test"
	openAISubscriptionLifecycleActionRefresh = "refresh"
	openAISubscriptionLifecycleActionDisable = "disable"
	openAISubscriptionLifecycleActionEnable  = "enable"
	openAISubscriptionOperatorDisabledReason = "operator_disabled"
	defaultOpenAIModelsBaseURL               = "https://api.openai.com/v1"
)

type openAISubscriptionLifecycleResponse struct {
	CredentialID  string `json:"credential_id"`
	Action        string `json:"action"`
	Status        string `json:"status"`
	ReasonCode    string `json:"reason_code,omitempty"`
	ModelsCount   *int   `json:"models_count,omitempty"`
	LastRefreshAt string `json:"last_refresh_at,omitempty"`
}

func (h *Handlers) OpenAISubscriptionCredentialTest(w http.ResponseWriter, r *http.Request) {
	credentialID := chi.URLParam(r, "credential_id")
	bundle, err := h.resolveUsableOpenAISubscriptionBundle(r.Context(), credentialID)
	if err != nil {
		h.writeOpenAISubscriptionLifecycleError(w, r.Context(), openAISubscriptionLifecycleActionTest, credentialID, err)
		return
	}

	modelsCount, err := h.testOpenAISubscriptionModels(r.Context(), bundle.AccessToken)
	if err != nil {
		reasonCode := openAISubscriptionLifecycleUpstreamReason(err)
		h.auditOpenAISubscriptionLifecycle(r.Context(), callback.OpenAISubscriptionAttribution{
			CredentialID: credentialID,
			Provider:     "openai",
			Action:       openAISubscriptionLifecycleActionTest,
			Status:       "failure",
			ReasonCode:   reasonCode,
		})
		writeJSON(w, http.StatusBadGateway, openAISubscriptionLifecycleResponse{
			CredentialID: credentialID,
			Action:       openAISubscriptionLifecycleActionTest,
			Status:       "error",
			ReasonCode:   reasonCode,
		})
		return
	}

	h.auditOpenAISubscriptionLifecycle(r.Context(), callback.OpenAISubscriptionAttribution{
		CredentialID: credentialID,
		Provider:     "openai",
		Action:       openAISubscriptionLifecycleActionTest,
		Status:       "success",
		Metadata:     map[string]any{"models_count": modelsCount},
	})
	writeJSON(w, http.StatusOK, openAISubscriptionLifecycleResponse{
		CredentialID: credentialID,
		Action:       openAISubscriptionLifecycleActionTest,
		Status:       "ok",
		ModelsCount:  &modelsCount,
	})
}

func (h *Handlers) OpenAISubscriptionCredentialRefresh(w http.ResponseWriter, r *http.Request) {
	credentialID := chi.URLParam(r, "credential_id")
	if _, err := h.forceRefreshOpenAISubscriptionCredential(r.Context(), credentialID); err != nil {
		h.writeOpenAISubscriptionLifecycleError(w, r.Context(), openAISubscriptionLifecycleActionRefresh, credentialID, err)
		return
	}

	loaded, err := h.loadOpenAISubscriptionCredential(r.Context(), credentialID)
	if err != nil {
		h.writeOpenAISubscriptionLifecycleError(w, r.Context(), openAISubscriptionLifecycleActionRefresh, credentialID, err)
		return
	}
	lastRefreshAt := ""
	if loaded.Info.LastRefreshAt != nil {
		lastRefreshAt = loaded.Info.LastRefreshAt.UTC().Format(time.RFC3339)
	}
	writeJSON(w, http.StatusOK, openAISubscriptionLifecycleResponse{
		CredentialID:  credentialID,
		Action:        openAISubscriptionLifecycleActionRefresh,
		Status:        "ok",
		LastRefreshAt: lastRefreshAt,
	})
}

func (h *Handlers) OpenAISubscriptionCredentialDisable(w http.ResponseWriter, r *http.Request) {
	credentialID := chi.URLParam(r, "credential_id")
	cred, info, err := h.loadOpenAISubscriptionCredentialMetadata(r.Context(), credentialID)
	if err != nil {
		h.writeOpenAISubscriptionLifecycleError(w, r.Context(), openAISubscriptionLifecycleActionDisable, credentialID, err)
		return
	}

	info.Status = "disabled"
	info.DisabledReason = openAISubscriptionOperatorDisabledReason
	info.LastError = redact.String(info.LastError)
	infoJSON, err := marshalSafeCredentialInfo(info)
	if err != nil {
		h.writeOpenAISubscriptionLifecycleError(w, r.Context(), openAISubscriptionLifecycleActionDisable, credentialID, h.openAISubscriptionError(credentialID, OpenAISubscriptionCredentialMalformed, err.Error()))
		return
	}
	if err := h.DB.UpdateCredentialInfo(r.Context(), db.UpdateCredentialInfoParams{
		CredentialID:   credentialID,
		CredentialInfo: infoJSON,
	}); err != nil {
		h.writeOpenAISubscriptionLifecycleError(w, r.Context(), openAISubscriptionLifecycleActionDisable, credentialID, h.openAISubscriptionError(credentialID, OpenAISubscriptionCredentialLookupErr, "disable credential metadata update failed"))
		return
	}
	h.invalidateOpenAISubscriptionCodexCatalog(r.Context(), credentialID)
	h.auditOpenAISubscriptionLifecycle(r.Context(), callback.OpenAISubscriptionAttribution{
		CredentialID:   credentialID,
		Provider:       "openai",
		OrganizationID: openAISubscriptionCredentialOrgID(cred.OrganizationID),
		Action:         openAISubscriptionLifecycleActionDisable,
		Status:         "success",
		Metadata:       map[string]any{"disabled_reason": openAISubscriptionOperatorDisabledReason},
	})
	writeJSON(w, http.StatusOK, openAISubscriptionLifecycleResponse{
		CredentialID: credentialID,
		Action:       openAISubscriptionLifecycleActionDisable,
		Status:       "ok",
	})
}

func (h *Handlers) OpenAISubscriptionCredentialEnable(w http.ResponseWriter, r *http.Request) {
	credentialID := chi.URLParam(r, "credential_id")
	cred, info, err := h.loadOpenAISubscriptionCredentialMetadata(r.Context(), credentialID)
	if err != nil {
		h.writeOpenAISubscriptionLifecycleError(w, r.Context(), openAISubscriptionLifecycleActionEnable, credentialID, err)
		return
	}

	if !openAISubscriptionCredentialCanEnable(info) {
		h.writeOpenAISubscriptionLifecycleError(w, r.Context(), openAISubscriptionLifecycleActionEnable, credentialID, h.openAISubscriptionError(credentialID, OpenAISubscriptionCredentialDisabled, "credential disabled by non-operator reason"))
		return
	}

	info.Status = "active"
	info.DisabledReason = ""
	info.LastError = redact.String(info.LastError)
	infoJSON, err := marshalSafeCredentialInfo(info)
	if err != nil {
		h.writeOpenAISubscriptionLifecycleError(w, r.Context(), openAISubscriptionLifecycleActionEnable, credentialID, h.openAISubscriptionError(credentialID, OpenAISubscriptionCredentialMalformed, err.Error()))
		return
	}
	if err := h.DB.UpdateCredentialInfo(r.Context(), db.UpdateCredentialInfoParams{
		CredentialID:   credentialID,
		CredentialInfo: infoJSON,
	}); err != nil {
		h.writeOpenAISubscriptionLifecycleError(w, r.Context(), openAISubscriptionLifecycleActionEnable, credentialID, h.openAISubscriptionError(credentialID, OpenAISubscriptionCredentialLookupErr, "enable credential metadata update failed"))
		return
	}
	h.invalidateOpenAISubscriptionCodexCatalog(r.Context(), credentialID)
	h.auditOpenAISubscriptionLifecycle(r.Context(), callback.OpenAISubscriptionAttribution{
		CredentialID:   credentialID,
		Provider:       "openai",
		OrganizationID: openAISubscriptionCredentialOrgID(cred.OrganizationID),
		Action:         openAISubscriptionLifecycleActionEnable,
		Status:         "success",
		Metadata:       map[string]any{"previous_disabled_reason": openAISubscriptionOperatorDisabledReason, "status": "active"},
	})
	writeJSON(w, http.StatusOK, openAISubscriptionLifecycleResponse{
		CredentialID: credentialID,
		Action:       openAISubscriptionLifecycleActionEnable,
		Status:       "ok",
	})
}

func openAISubscriptionCredentialCanEnable(info OpenAISubscriptionCredentialInfo) bool {
	status := strings.TrimSpace(strings.ToLower(info.Status))
	reason := strings.TrimSpace(info.DisabledReason)
	if status != "disabled" && reason == "" {
		return true
	}
	return reason == openAISubscriptionOperatorDisabledReason
}

func (h *Handlers) loadOpenAISubscriptionCredentialMetadata(ctx context.Context, credentialID string) (db.CredentialTable, OpenAISubscriptionCredentialInfo, error) {
	if h == nil || h.DB == nil {
		return db.CredentialTable{}, OpenAISubscriptionCredentialInfo{}, errors.New("OpenAI subscription credential lifecycle failed: database not configured")
	}
	cred, err := h.DB.GetCredential(ctx, credentialID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.CredentialTable{}, OpenAISubscriptionCredentialInfo{}, h.openAISubscriptionError(credentialID, OpenAISubscriptionCredentialMissing, "credential missing")
		}
		return db.CredentialTable{}, OpenAISubscriptionCredentialInfo{}, h.openAISubscriptionError(credentialID, OpenAISubscriptionCredentialLookupErr, "credential lookup failed")
	}
	if cred.CredentialType != CredentialTypeOpenAISubscription {
		return db.CredentialTable{}, OpenAISubscriptionCredentialInfo{}, h.openAISubscriptionError(credentialID, OpenAISubscriptionCredentialWrongType, "credential wrong type")
	}
	var info OpenAISubscriptionCredentialInfo
	if len(cred.CredentialInfo) > 0 {
		if err := json.Unmarshal(cred.CredentialInfo, &info); err != nil {
			return db.CredentialTable{}, OpenAISubscriptionCredentialInfo{}, h.openAISubscriptionError(credentialID, OpenAISubscriptionCredentialMalformed, "invalid credential metadata")
		}
	}
	return cred, info, nil
}

func (h *Handlers) testOpenAISubscriptionModels(ctx context.Context, bearerToken string) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(h.openAIUpstreamBaseURL(), "/")+"/models", nil)
	if err != nil {
		return 0, fmt.Errorf("create models request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+bearerToken)
	req.Header.Set("Accept", "application/json")

	resp, err := h.openAIUpstreamHTTPClient().Do(req)
	if err != nil {
		return 0, fmt.Errorf("models request failed: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return 0, &openAISubscriptionLifecycleUpstreamError{StatusCode: resp.StatusCode, Body: redact.String(string(body))}
	}

	var payload struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return 0, fmt.Errorf("parse models response: %w", err)
	}
	return len(payload.Data), nil
}

func (h *Handlers) openAIUpstreamHTTPClient() *http.Client {
	if h != nil && h.OpenAIUpstreamHTTPClient != nil {
		return h.OpenAIUpstreamHTTPClient
	}
	return http.DefaultClient
}

func (h *Handlers) openAIUpstreamBaseURL() string {
	if h != nil && h.OpenAIUpstreamBaseURL != "" {
		return h.OpenAIUpstreamBaseURL
	}
	return defaultOpenAIModelsBaseURL
}

type openAISubscriptionLifecycleUpstreamError struct {
	StatusCode int
	Body       string
}

func (e *openAISubscriptionLifecycleUpstreamError) Error() string {
	return fmt.Sprintf("upstream_%d: %s", e.StatusCode, redact.String(e.Body))
}

func openAISubscriptionLifecycleUpstreamReason(err error) string {
	var upstreamErr *openAISubscriptionLifecycleUpstreamError
	if errors.As(err, &upstreamErr) {
		return fmt.Sprintf("upstream_%d", upstreamErr.StatusCode)
	}
	return "upstream_request_failed"
}

func (h *Handlers) writeOpenAISubscriptionLifecycleError(w http.ResponseWriter, ctx context.Context, action, credentialID string, err error) {
	code := openAISubscriptionErrorCode(err)
	if code == "" {
		code = OpenAISubscriptionCredentialLookupErr
	}
	status := http.StatusBadRequest
	if code == OpenAISubscriptionCredentialLookupErr {
		status = http.StatusInternalServerError
	}
	if code == OpenAISubscriptionCredentialMissing {
		status = http.StatusNotFound
	}
	if action != openAISubscriptionLifecycleActionRefresh || code != OpenAISubscriptionCredentialRefreshErr {
		h.auditOpenAISubscriptionLifecycle(ctx, callback.OpenAISubscriptionAttribution{
			CredentialID: credentialID,
			Provider:     "openai",
			Action:       action,
			Status:       "failure",
			ReasonCode:   string(code),
		})
	}
	writeJSON(w, status, openAISubscriptionLifecycleResponse{
		CredentialID: credentialID,
		Action:       action,
		Status:       "error",
		ReasonCode:   string(code),
	})
}
