package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/praxisllmlab/tianjiLLM/internal/auth"
	"github.com/praxisllmlab/tianjiLLM/internal/callback"
	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/praxisllmlab/tianjiLLM/internal/db"
	"github.com/praxisllmlab/tianjiLLM/internal/provider/openai"
	"github.com/praxisllmlab/tianjiLLM/internal/security/redact"
)

const (
	openAISubscriptionRefreshBuffer          = 5 * time.Minute
	openAISubscriptionProactiveRefreshBuffer = 30 * time.Minute
	openAISubscriptionReconnectRequiredMsg   = "OpenAI session ended; reconnect required"
)

type OpenAISubscriptionCredentialErrorCode string

const (
	OpenAISubscriptionCredentialMissing    OpenAISubscriptionCredentialErrorCode = "credential_missing"
	OpenAISubscriptionCredentialWrongType  OpenAISubscriptionCredentialErrorCode = "credential_wrong_type"
	OpenAISubscriptionCredentialDisabled   OpenAISubscriptionCredentialErrorCode = "credential_disabled"
	OpenAISubscriptionCredentialMalformed  OpenAISubscriptionCredentialErrorCode = "credential_malformed"
	OpenAISubscriptionCredentialExpired    OpenAISubscriptionCredentialErrorCode = "credential_expired"
	OpenAISubscriptionCredentialLookupErr  OpenAISubscriptionCredentialErrorCode = "credential_lookup_failed"
	OpenAISubscriptionCredentialRefreshErr OpenAISubscriptionCredentialErrorCode = "refresh_failed"
	OpenAISubscriptionCredentialAuthErr    OpenAISubscriptionCredentialErrorCode = "auth_failed_after_refresh"
	OpenAISubscriptionCredentialReconnect  OpenAISubscriptionCredentialErrorCode = "refresh_token_invalidated"
)

type OpenAISubscriptionCredentialError struct {
	CredentialID        string
	Code                OpenAISubscriptionCredentialErrorCode
	Message             string
	NonSelectableReason string
}

func (e *OpenAISubscriptionCredentialError) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("OpenAI subscription credential resolution failed for %q: %s", e.CredentialID, redact.String(e.Message))
}

func (h *Handlers) resolveUsableOpenAISubscriptionBundle(ctx context.Context, credentialID string) (OpenAISubscriptionTokenBundle, error) {
	cred, err := h.loadOpenAISubscriptionCredential(ctx, credentialID)
	if err != nil {
		return OpenAISubscriptionTokenBundle{}, err
	}
	if h.openAISubscriptionTokenIsFresh(cred.Bundle) {
		return cred.Bundle, nil
	}

	value, err, _ := h.openAIRefreshGroup.Do(credentialID, func() (any, error) {
		latest, loadErr := h.loadOpenAISubscriptionCredential(ctx, credentialID)
		if loadErr != nil {
			return nil, loadErr
		}
		if h.openAISubscriptionTokenIsFresh(latest.Bundle) {
			return latest.Bundle, nil
		}
		return h.refreshOpenAISubscriptionCredential(ctx, latest, true)
	})
	if err != nil {
		return OpenAISubscriptionTokenBundle{}, err
	}
	bundle, ok := value.(OpenAISubscriptionTokenBundle)
	if !ok {
		return OpenAISubscriptionTokenBundle{}, h.openAISubscriptionError(credentialID, OpenAISubscriptionCredentialMalformed, "refresh returned malformed credential")
	}
	return bundle, nil
}

func (h *Handlers) forceRefreshOpenAISubscriptionCredential(ctx context.Context, credentialID string) (resolvedOpenAISubscriptionCredential, error) {
	if h == nil || h.DB == nil {
		return resolvedOpenAISubscriptionCredential{}, errors.New("OpenAI subscription credential resolution failed: database not configured")
	}

	value, err, _ := h.openAIRefreshGroup.Do(credentialID, func() (any, error) {
		latest, loadErr := h.loadOpenAISubscriptionCredential(ctx, credentialID)
		if loadErr != nil {
			return nil, loadErr
		}
		return h.refreshOpenAISubscriptionCredential(ctx, latest, true)
	})
	if err != nil {
		return resolvedOpenAISubscriptionCredential{}, err
	}
	bundle, ok := value.(OpenAISubscriptionTokenBundle)
	if !ok {
		return resolvedOpenAISubscriptionCredential{}, h.openAISubscriptionError(credentialID, OpenAISubscriptionCredentialMalformed, "refresh returned malformed credential")
	}
	return resolvedOpenAISubscriptionCredential{
		CredentialID: credentialID,
		BearerToken:  bundle.AccessToken,
		AccountID:    bundle.AccountID,
	}, nil
}

func (h *Handlers) refreshOpenAISubscriptionCredentialForUsageSnapshot(ctx context.Context, credentialID string) (resolvedOpenAISubscriptionCredential, error) {
	if h == nil || h.DB == nil {
		return resolvedOpenAISubscriptionCredential{}, errors.New("OpenAI subscription credential resolution failed: database not configured")
	}

	value, err, _ := h.openAIRefreshGroup.Do(credentialID, func() (any, error) {
		latest, loadErr := h.loadOpenAISubscriptionCredential(ctx, credentialID)
		if loadErr != nil {
			return nil, loadErr
		}
		return h.refreshOpenAISubscriptionCredential(ctx, latest, false)
	})
	if err != nil {
		return resolvedOpenAISubscriptionCredential{}, err
	}
	bundle, ok := value.(OpenAISubscriptionTokenBundle)
	if !ok {
		return resolvedOpenAISubscriptionCredential{}, h.openAISubscriptionError(credentialID, OpenAISubscriptionCredentialMalformed, "refresh returned malformed credential")
	}
	return resolvedOpenAISubscriptionCredential{
		CredentialID: credentialID,
		BearerToken:  bundle.AccessToken,
		AccountID:    bundle.AccountID,
	}, nil
}

func (h *Handlers) resolveOpenAISubscriptionCredentialForUsageSnapshot(ctx context.Context, credentialID string) (resolvedOpenAISubscriptionCredential, error) {
	if h == nil || h.DB == nil {
		return resolvedOpenAISubscriptionCredential{}, errors.New("OpenAI subscription credential resolution failed: database not configured")
	}

	loaded, err := h.loadOpenAISubscriptionCredential(ctx, credentialID)
	if err != nil {
		return resolvedOpenAISubscriptionCredential{}, err
	}
	if h.openAISubscriptionTokenIsFresh(loaded.Bundle) {
		return resolvedOpenAISubscriptionCredential{
			CredentialID: credentialID,
			BearerToken:  loaded.Bundle.AccessToken,
			AccountID:    loaded.Bundle.AccountID,
		}, nil
	}
	return h.refreshOpenAISubscriptionCredentialForUsageSnapshot(ctx, credentialID)
}

type loadedOpenAISubscriptionCredential struct {
	Credential db.CredentialTable
	Info       OpenAISubscriptionCredentialInfo
	Bundle     OpenAISubscriptionTokenBundle
}

func (h *Handlers) loadOpenAISubscriptionCredential(ctx context.Context, credentialID string) (loadedOpenAISubscriptionCredential, error) {
	cred, err := h.DB.GetCredential(ctx, credentialID)
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return loadedOpenAISubscriptionCredential{}, h.openAISubscriptionError(credentialID, OpenAISubscriptionCredentialLookupErr, "credential lookup failed")
		}
		return loadedOpenAISubscriptionCredential{}, h.openAISubscriptionError(credentialID, OpenAISubscriptionCredentialMissing, "credential missing")
	}
	if cred.CredentialType != CredentialTypeOpenAISubscription {
		return loadedOpenAISubscriptionCredential{}, h.openAISubscriptionError(credentialID, OpenAISubscriptionCredentialWrongType, "credential wrong type")
	}

	var info OpenAISubscriptionCredentialInfo
	if len(cred.CredentialInfo) > 0 {
		if unmarshalErr := json.Unmarshal(cred.CredentialInfo, &info); unmarshalErr != nil {
			return loadedOpenAISubscriptionCredential{}, h.openAISubscriptionError(credentialID, OpenAISubscriptionCredentialMalformed, "invalid credential metadata")
		}
	}
	if strings.TrimSpace(info.Status) == "disabled" {
		return loadedOpenAISubscriptionCredential{}, h.openAISubscriptionErrorWithNonSelectableReason(credentialID, OpenAISubscriptionCredentialDisabled, "credential disabled", info.DisabledReason)
	}
	if !openAISubscriptionCredentialInfoSelectable(info) {
		return loadedOpenAISubscriptionCredential{}, h.openAISubscriptionErrorWithNonSelectableReason(credentialID, OpenAISubscriptionCredentialRefreshErr, openAISubscriptionNonSelectableReason(info), info.DisabledReason)
	}

	plaintext, err := auth.Decrypt(cred.CredentialValue, h.getMasterKey())
	if err != nil {
		return loadedOpenAISubscriptionCredential{}, h.openAISubscriptionError(credentialID, OpenAISubscriptionCredentialMalformed, "decrypt credential")
	}
	var bundle OpenAISubscriptionTokenBundle
	if err := json.Unmarshal([]byte(plaintext), &bundle); err != nil {
		return loadedOpenAISubscriptionCredential{}, h.openAISubscriptionError(credentialID, OpenAISubscriptionCredentialMalformed, "malformed credential bundle")
	}
	if err := validateOpenAITokenBundle(bundle, false); err != nil {
		return loadedOpenAISubscriptionCredential{}, h.openAISubscriptionError(credentialID, OpenAISubscriptionCredentialMalformed, redact.String(err.Error()))
	}
	return loadedOpenAISubscriptionCredential{Credential: cred, Info: info, Bundle: bundle}, nil
}

func (h *Handlers) refreshOpenAISubscriptionCredential(ctx context.Context, loaded loadedOpenAISubscriptionCredential, recordFailure bool) (OpenAISubscriptionTokenBundle, error) {
	credentialID := loaded.Credential.CredentialID
	if loaded.Bundle.RefreshToken == "" {
		return OpenAISubscriptionTokenBundle{}, h.openAISubscriptionError(credentialID, OpenAISubscriptionCredentialExpired, "refresh_token required")
	}

	refreshed, err := openai.RefreshToken(ctx, h.openAIOAuthHTTPClient(), h.openAIOAuthConfig(), loaded.Bundle.RefreshToken)
	if err != nil {
		if !recordFailure {
			reason := ""
			if openAISubscriptionRefreshTokenInvalidated(err.Error()) {
				reason = string(OpenAISubscriptionCredentialReconnect)
			}
			return OpenAISubscriptionTokenBundle{}, h.openAISubscriptionErrorWithNonSelectableReason(
				credentialID,
				OpenAISubscriptionCredentialRefreshErr,
				redactOpenAISubscriptionCredentialError(err.Error(), loaded.Bundle),
				reason,
			)
		}
		return OpenAISubscriptionTokenBundle{}, h.recordOpenAISubscriptionRefreshFailure(ctx, loaded, err.Error())
	}

	expiresAt := time.Time{}
	if refreshed.ExpiresIn > 0 {
		expiresAt = h.openAISubscriptionNowUTC().Add(time.Duration(refreshed.ExpiresIn) * time.Second)
	}
	bundle := OpenAISubscriptionTokenBundle{
		AccessToken:  refreshed.AccessToken,
		RefreshToken: refreshed.RefreshToken,
		ExpiresAt:    expiresAt,
		AccountID:    openAISubscriptionAccountID(refreshed),
	}
	if bundle.AccountID == "" {
		bundle.AccountID = loaded.Bundle.AccountID
	}
	if err := validateOpenAITokenBundle(bundle, false); err != nil {
		if !recordFailure {
			return OpenAISubscriptionTokenBundle{}, h.openAISubscriptionError(credentialID, OpenAISubscriptionCredentialRefreshErr, redact.String(err.Error()))
		}
		return OpenAISubscriptionTokenBundle{}, h.recordOpenAISubscriptionRefreshFailure(ctx, loaded, err.Error())
	}

	now := h.openAISubscriptionNowUTC()
	info := OpenAISubscriptionCredentialInfo{
		Email:         loaded.Info.Email,
		Scopes:        loaded.Info.Scopes,
		Status:        "active",
		LastRefreshAt: &now,
	}
	if refreshed.Scope != "" {
		info.Scopes = strings.Fields(refreshed.Scope)
	}
	if err := h.UpdateOpenAISubscriptionCredential(ctx, credentialID, bundle, info); err != nil {
		return OpenAISubscriptionTokenBundle{}, h.openAISubscriptionError(credentialID, OpenAISubscriptionCredentialRefreshErr, redact.String(err.Error()))
	}
	h.auditOpenAISubscriptionLifecycle(ctx, callback.OpenAISubscriptionAttribution{
		CredentialID:   credentialID,
		Provider:       "openai",
		OrganizationID: openAISubscriptionCredentialOrgID(loaded.Credential.OrganizationID),
		Action:         "refresh",
		Status:         "success",
		Metadata:       map[string]any{"last_refresh_at": now.Format(time.RFC3339)},
	})
	if bundle.RefreshToken == "" {
		bundle.RefreshToken = loaded.Bundle.RefreshToken
	}
	return bundle, nil
}

type OpenAISubscriptionProactiveRefreshResult struct {
	Scanned           int
	Eligible          int
	Refreshed         int
	Skipped           int
	Failed            int
	ReconnectRequired int
}

func (h *Handlers) ProactiveRefreshOpenAISubscriptionCredentials(ctx context.Context) (OpenAISubscriptionProactiveRefreshResult, error) {
	var result OpenAISubscriptionProactiveRefreshResult
	if h == nil || h.DB == nil {
		return result, errors.New("OpenAI subscription proactive refresh failed: database not configured")
	}

	rows, err := h.DB.ListOpenAISubscriptionRefreshCandidates(ctx)
	if err != nil {
		return result, err
	}
	result.Scanned = len(rows)

	for _, row := range rows {
		loaded, loadErr := h.loadOpenAISubscriptionCredentialFromRow(row)
		if loadErr != nil {
			result.Skipped++
			continue
		}
		if h.openAISubscriptionTokenOutsideProactiveBuffer(loaded.Bundle) {
			result.Skipped++
			continue
		}
		result.Eligible++

		_, refreshErr, _ := h.openAIRefreshGroup.Do(row.CredentialID, func() (any, error) {
			latest, latestErr := h.loadOpenAISubscriptionCredential(ctx, row.CredentialID)
			if latestErr != nil {
				return nil, latestErr
			}
			if h.openAISubscriptionTokenOutsideProactiveBuffer(latest.Bundle) {
				return latest.Bundle, nil
			}
			return h.refreshOpenAISubscriptionCredential(ctx, latest, true)
		})
		if refreshErr != nil {
			result.Failed++
			if openAISubscriptionErrorCode(refreshErr) == OpenAISubscriptionCredentialReconnect {
				result.ReconnectRequired++
			}
			continue
		}
		result.Refreshed++
	}
	return result, nil
}

func (h *Handlers) loadOpenAISubscriptionCredentialFromRow(cred db.CredentialTable) (loadedOpenAISubscriptionCredential, error) {
	credentialID := cred.CredentialID
	if cred.CredentialType != CredentialTypeOpenAISubscription {
		return loadedOpenAISubscriptionCredential{}, h.openAISubscriptionError(credentialID, OpenAISubscriptionCredentialWrongType, "credential wrong type")
	}
	var info OpenAISubscriptionCredentialInfo
	if len(cred.CredentialInfo) > 0 {
		if err := json.Unmarshal(cred.CredentialInfo, &info); err != nil {
			return loadedOpenAISubscriptionCredential{}, h.openAISubscriptionError(credentialID, OpenAISubscriptionCredentialMalformed, "invalid credential metadata")
		}
	}
	if strings.TrimSpace(info.Status) == "disabled" {
		return loadedOpenAISubscriptionCredential{}, h.openAISubscriptionErrorWithNonSelectableReason(credentialID, OpenAISubscriptionCredentialDisabled, "credential disabled", info.DisabledReason)
	}
	if !openAISubscriptionCredentialInfoSelectable(info) {
		return loadedOpenAISubscriptionCredential{}, h.openAISubscriptionErrorWithNonSelectableReason(credentialID, OpenAISubscriptionCredentialRefreshErr, openAISubscriptionNonSelectableReason(info), info.DisabledReason)
	}
	plaintext, err := auth.Decrypt(cred.CredentialValue, h.getMasterKey())
	if err != nil {
		return loadedOpenAISubscriptionCredential{}, h.openAISubscriptionError(credentialID, OpenAISubscriptionCredentialMalformed, "decrypt credential")
	}
	var bundle OpenAISubscriptionTokenBundle
	if err := json.Unmarshal([]byte(plaintext), &bundle); err != nil {
		return loadedOpenAISubscriptionCredential{}, h.openAISubscriptionError(credentialID, OpenAISubscriptionCredentialMalformed, "malformed credential bundle")
	}
	if err := validateOpenAITokenBundle(bundle, false); err != nil {
		return loadedOpenAISubscriptionCredential{}, h.openAISubscriptionError(credentialID, OpenAISubscriptionCredentialMalformed, redact.String(err.Error()))
	}
	return loadedOpenAISubscriptionCredential{Credential: cred, Info: info, Bundle: bundle}, nil
}

func (h *Handlers) recordOpenAISubscriptionRefreshFailure(ctx context.Context, loaded loadedOpenAISubscriptionCredential, message string) error {
	credentialID := loaded.Credential.CredentialID
	safeErr := redactOpenAISubscriptionCredentialError(message, loaded.Bundle)
	now := h.openAISubscriptionNowUTC()
	reconnectRequired := openAISubscriptionRefreshTokenInvalidated(message)
	reasonCode := string(OpenAISubscriptionCredentialRefreshErr)
	status := "refresh_failed"
	operatorMessage := ""
	if reconnectRequired {
		reasonCode = string(OpenAISubscriptionCredentialReconnect)
		safeErr = openAISubscriptionReconnectRequiredMsg
		operatorMessage = openAISubscriptionReconnectRequiredMsg
	}
	firstFailedAt := loaded.Info.FirstRefreshFailedAt
	if firstFailedAt == nil {
		firstFailedAt = &now
	}
	updateErr := h.UpdateOpenAISubscriptionCredentialFailure(ctx, credentialID, OpenAISubscriptionCredentialInfo{
		Email:                loaded.Info.Email,
		Scopes:               loaded.Info.Scopes,
		Status:               status,
		LastRefreshAt:        loaded.Info.LastRefreshAt,
		FirstRefreshFailedAt: firstFailedAt,
		LastRefreshFailedAt:  &now,
		LastError:            safeErr,
		DisabledReason:       reasonCode,
		OperatorMessage:      operatorMessage,
	})
	if updateErr != nil {
		h.auditOpenAISubscriptionLifecycle(ctx, callback.OpenAISubscriptionAttribution{
			CredentialID:   credentialID,
			Provider:       "openai",
			OrganizationID: openAISubscriptionCredentialOrgID(loaded.Credential.OrganizationID),
			Action:         "refresh",
			Status:         "failure",
			ReasonCode:     reasonCode,
			Metadata:       map[string]any{"last_error": safeErr, "persist_failed": true},
		})
		return h.openAISubscriptionError(credentialID, OpenAISubscriptionCredentialRefreshErr, fmt.Sprintf("refresh failed at %s: %s; persist failure metadata failed", now.Format(time.RFC3339), safeErr))
	}
	h.auditOpenAISubscriptionLifecycle(ctx, callback.OpenAISubscriptionAttribution{
		CredentialID:   credentialID,
		Provider:       "openai",
		OrganizationID: openAISubscriptionCredentialOrgID(loaded.Credential.OrganizationID),
		Action:         "refresh",
		Status:         "failure",
		ReasonCode:     reasonCode,
		Metadata:       map[string]any{"last_error": safeErr},
	})
	if reconnectRequired {
		return h.openAISubscriptionError(credentialID, OpenAISubscriptionCredentialReconnect, fmt.Sprintf("refresh failed at %s: %s", now.Format(time.RFC3339), safeErr))
	}
	return h.openAISubscriptionError(credentialID, OpenAISubscriptionCredentialRefreshErr, fmt.Sprintf("refresh failed at %s: %s", now.Format(time.RFC3339), safeErr))
}

func redactOpenAISubscriptionCredentialError(message string, bundle OpenAISubscriptionTokenBundle) string {
	safe := redact.String(message)
	for _, secret := range []string{bundle.AccessToken, bundle.RefreshToken} {
		if secret != "" {
			safe = strings.ReplaceAll(safe, secret, "[REDACTED]")
		}
	}
	return redact.String(safe)
}

func (h *Handlers) recordOpenAISubscriptionAuthFailureAfterRefresh(ctx context.Context, credentialID string, message string) error {
	loaded, err := h.loadOpenAISubscriptionCredential(ctx, credentialID)
	if err != nil {
		return err
	}
	safeErr := redact.String(message)
	updateErr := h.UpdateOpenAISubscriptionCredentialFailure(ctx, credentialID, OpenAISubscriptionCredentialInfo{
		Email:          loaded.Info.Email,
		Scopes:         loaded.Info.Scopes,
		Status:         "disabled",
		LastRefreshAt:  loaded.Info.LastRefreshAt,
		LastError:      "OpenAI authentication failed after forced refresh: " + safeErr,
		DisabledReason: string(OpenAISubscriptionCredentialAuthErr),
	})
	if updateErr != nil {
		return h.openAISubscriptionError(credentialID, OpenAISubscriptionCredentialAuthErr, "persist auth failure metadata failed")
	}
	h.auditOpenAISubscriptionLifecycle(ctx, callback.OpenAISubscriptionAttribution{
		CredentialID:   credentialID,
		Provider:       "openai",
		OrganizationID: openAISubscriptionCredentialOrgID(loaded.Credential.OrganizationID),
		Action:         "disable",
		Status:         "failure",
		ReasonCode:     string(OpenAISubscriptionCredentialAuthErr),
		Metadata:       map[string]any{"last_error": "OpenAI authentication failed after forced refresh: " + safeErr},
	})
	return h.openAISubscriptionError(credentialID, OpenAISubscriptionCredentialAuthErr, "OpenAI authentication failed after forced refresh")
}

func (h *Handlers) openAISubscriptionTokenIsFresh(bundle OpenAISubscriptionTokenBundle) bool {
	return bundle.ExpiresAt.After(h.openAISubscriptionNowUTC().Add(openAISubscriptionRefreshBuffer))
}

func (h *Handlers) openAISubscriptionTokenOutsideProactiveBuffer(bundle OpenAISubscriptionTokenBundle) bool {
	return bundle.ExpiresAt.After(h.openAISubscriptionNowUTC().Add(openAISubscriptionProactiveRefreshBuffer))
}

func openAISubscriptionCredentialInfoSelectable(info OpenAISubscriptionCredentialInfo) bool {
	status := strings.TrimSpace(info.Status)
	if status == "" {
		status = "active"
	}
	if status == "disabled" || status == "refresh_failed" {
		return false
	}
	switch strings.TrimSpace(info.DisabledReason) {
	case "", "none":
		return true
	case "refresh_failed", "refresh_token_invalidated", "auth_failed_after_refresh", "operator_disabled":
		return false
	default:
		return true
	}
}

func openAISubscriptionNonSelectableReason(info OpenAISubscriptionCredentialInfo) string {
	if strings.TrimSpace(info.OperatorMessage) != "" {
		return info.OperatorMessage
	}
	if strings.TrimSpace(info.DisabledReason) != "" {
		return "credential non-selectable: " + info.DisabledReason
	}
	if strings.TrimSpace(info.Status) != "" {
		return "credential non-selectable: " + info.Status
	}
	return "credential non-selectable"
}

func openAISubscriptionRefreshTokenInvalidated(message string) bool {
	lower := strings.ToLower(message)
	return strings.Contains(lower, "refresh_token_invalidated") || strings.Contains(lower, "your session has ended")
}

func (h *Handlers) openAISubscriptionNowUTC() time.Time {
	if h.openAISubscriptionNow != nil {
		return h.openAISubscriptionNow().UTC()
	}
	return time.Now().UTC()
}

func (h *Handlers) openAIOAuthConfig() config.OpenAIOAuthConfig {
	if h.Config == nil {
		return config.OpenAIOAuthConfig{}
	}
	return h.Config.GeneralSettings.OpenAIOAuth
}

func (h *Handlers) openAISubscriptionError(credentialID string, code OpenAISubscriptionCredentialErrorCode, message string) error {
	return h.openAISubscriptionErrorWithNonSelectableReason(credentialID, code, message, "")
}

func (h *Handlers) openAISubscriptionErrorWithNonSelectableReason(credentialID string, code OpenAISubscriptionCredentialErrorCode, message, nonSelectableReason string) error {
	return &OpenAISubscriptionCredentialError{
		CredentialID:        credentialID,
		Code:                code,
		Message:             redact.String(message),
		NonSelectableReason: strings.TrimSpace(nonSelectableReason),
	}
}

func openAISubscriptionErrorCode(err error) OpenAISubscriptionCredentialErrorCode {
	var typed *OpenAISubscriptionCredentialError
	if errors.As(err, &typed) {
		return typed.Code
	}
	return ""
}
