package ui

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/praxisllmlab/tianjiLLM/internal/openaioauth"
	"github.com/praxisllmlab/tianjiLLM/internal/provider/openai"
	proxyhandler "github.com/praxisllmlab/tianjiLLM/internal/proxy/handler"
	"github.com/praxisllmlab/tianjiLLM/internal/ui/pages"
)

const openAIDeviceCSRFSessionKey = "openai_device_csrf_token"

func (h *UIHandler) handleOpenAIConnect(w http.ResponseWriter, r *http.Request) {
	setOpenAIDeviceNoStore(w)
	if h.Config == nil {
		http.Error(w, "OpenAI OAuth is not configured", http.StatusServiceUnavailable)
		return
	}
	if !h.Config.GeneralSettings.OpenAIOAuth.Enabled {
		http.Error(w, proxyhandler.ErrOpenAIConnectDisabled.Error(), http.StatusServiceUnavailable)
		return
	}
	orgID, err := openAIScopeOrganization(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	redirectURI, err := config.ResolveOpenAIOAuthRedirectURI(h.Config.GeneralSettings.OpenAIOAuth)
	if err != nil {
		http.Error(w, fmt.Sprintf("OpenAI OAuth redirect URI error: %v", err), http.StatusBadRequest)
		return
	}
	store := openaioauth.NewStateStore(h.Cache)
	record, err := store.Create(r.Context(), orgID, redirectURI, openaioauth.DefaultStateTTL)
	if err != nil {
		http.Error(w, fmt.Sprintf("OpenAI OAuth state error: %v", err), http.StatusServiceUnavailable)
		return
	}
	authorizeURL, err := openai.BuildAuthorizeURL(
		h.Config.GeneralSettings.OpenAIOAuth,
		record.RedirectURI,
		openai.ChallengeFromVerifier(record.CodeVerifier),
		record.State,
	)
	if err != nil {
		http.Error(w, fmt.Sprintf("OpenAI OAuth authorize URL error: %v", err), http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, authorizeURL, http.StatusSeeOther)
}

func (h *UIHandler) handleOpenAICallbackURL(w http.ResponseWriter, r *http.Request) {
	setOpenAIDeviceNoStore(w)
	_, err := h.openAIDeviceSessionBinding(r)
	if err != nil {
		http.Error(w, "session is not available", http.StatusUnauthorized)
		return
	}
	csrfToken, err := h.openAIDeviceCSRFToken(r)
	if err != nil {
		http.Error(w, "session is not available", http.StatusUnauthorized)
		return
	}
	if !validOpenAIDeviceCSRF(r, csrfToken) {
		http.Error(w, "invalid form session", http.StatusForbidden)
		return
	}
	h.proxyLifecycleHandlers().OpenAIOAuthPastedCallback(w, r)
}

func openAIScopeOrganization(r *http.Request) (string, error) {
	scope := strings.TrimSpace(r.FormValue("scope"))
	orgID := strings.TrimSpace(r.FormValue("org_id"))
	if orgID == "" {
		orgID = strings.TrimSpace(r.FormValue("organization_id"))
	}
	switch scope {
	case "":
		if orgID == "" {
			return "", errors.New("org_id is required")
		}
	case "global":
		if orgID != "" {
			return "", errors.New("global scope cannot include org_id")
		}
	default:
		return "", errors.New("unsupported OpenAI credential scope")
	}
	return orgID, nil
}

func (h *UIHandler) handleOpenAIDeviceStart(w http.ResponseWriter, r *http.Request) {
	setOpenAIDeviceNoStore(w)
	if h.Config == nil {
		h.renderOpenAIDeviceError(w, r, "OpenAI OAuth is not configured", http.StatusServiceUnavailable, "")
		return
	}
	orgID, err := openAIScopeOrganization(r)
	if err != nil {
		h.renderOpenAIDeviceError(w, r, err.Error(), http.StatusBadRequest, orgID)
		return
	}
	binding, err := h.openAIDeviceSessionBinding(r)
	if err != nil {
		h.renderOpenAIDeviceError(w, r, "Your session is no longer available. Sign in again.", http.StatusUnauthorized, orgID)
		return
	}
	csrfToken, err := h.openAIDeviceCSRFToken(r)
	if err != nil {
		http.Error(w, "session is not available", http.StatusUnauthorized)
		return
	}
	if !validOpenAIDeviceCSRF(r, csrfToken) {
		h.renderOpenAIDeviceError(w, r, "This form has expired. Refresh the credentials page and try again.", http.StatusForbidden, orgID)
		return
	}
	result, err := h.proxyLifecycleHandlers().StartOpenAIDeviceAuth(r.Context(), orgID, binding)
	if err != nil {
		h.renderOpenAIDeviceError(w, r, openAIDeviceStartErrorMessage(err), http.StatusServiceUnavailable, orgID)
		return
	}
	view := openAIDeviceStartView(result)
	view.CSRFToken = csrfToken
	view.StatusValues = openAIDeviceStatusValues(csrfToken)
	render(r.Context(), w, pages.OpenAIDeviceAuthStarted(view))
}

func (h *UIHandler) handleOpenAIDeviceStatus(w http.ResponseWriter, r *http.Request) {
	setOpenAIDeviceNoStore(w)
	flowID := strings.TrimSpace(r.FormValue("flow_id"))
	if flowID == "" {
		http.Error(w, "flow_id is required", http.StatusBadRequest)
		return
	}
	binding, err := h.openAIDeviceSessionBinding(r)
	if err != nil {
		http.Error(w, "session is not available", http.StatusUnauthorized)
		return
	}
	csrfToken, err := h.openAIDeviceCSRFToken(r)
	if err != nil {
		http.Error(w, "session is not available", http.StatusUnauthorized)
		return
	}
	if !validOpenAIDeviceCSRF(r, csrfToken) {
		http.Error(w, "invalid form session", http.StatusForbidden)
		return
	}
	var result proxyhandler.OpenAIDeviceAuthStatus
	err = h.validateOpenAIDeviceScope(r, flowID, binding)
	if err == nil {
		result, err = h.proxyLifecycleHandlers().PollOpenAIDeviceAuth(r.Context(), flowID, binding)
	}
	if err != nil {
		switch {
		case errors.Is(err, openaioauth.ErrDeviceAuthOwnership):
			http.Error(w, "forbidden", http.StatusForbidden)
		case errors.Is(err, openaioauth.ErrDeviceAuthNotFound):
			http.Error(w, "device login not found", http.StatusNotFound)
		default:
			http.Error(w, "device login status unavailable", http.StatusServiceUnavailable)
		}
		return
	}
	view := h.openAIDeviceStatusView(result)
	view.CSRFToken = csrfToken
	view.StatusValues = openAIDeviceStatusValues(csrfToken)
	render(r.Context(), w, pages.OpenAIDeviceAuthStatus(view))
}

func (h *UIHandler) handleOpenAIDeviceCancel(w http.ResponseWriter, r *http.Request) {
	setOpenAIDeviceNoStore(w)
	flowID := strings.TrimSpace(r.FormValue("flow_id"))
	if flowID == "" {
		http.Error(w, "flow_id is required", http.StatusBadRequest)
		return
	}
	binding, err := h.openAIDeviceSessionBinding(r)
	if err != nil {
		http.Error(w, "session is not available", http.StatusUnauthorized)
		return
	}
	csrfToken, err := h.openAIDeviceCSRFToken(r)
	if err != nil {
		http.Error(w, "session is not available", http.StatusUnauthorized)
		return
	}
	if !validOpenAIDeviceCSRF(r, csrfToken) {
		http.Error(w, "invalid form session", http.StatusForbidden)
		return
	}
	err = h.validateOpenAIDeviceScope(r, flowID, binding)
	if err == nil {
		err = h.proxyLifecycleHandlers().CancelOpenAIDeviceAuth(r.Context(), flowID, binding)
	}
	if err != nil {
		if errors.Is(err, openaioauth.ErrDeviceAuthOwnership) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		if errors.Is(err, proxyhandler.ErrOpenAIDeviceAuthExchangeInProgress) {
			http.Error(w, "device login is completing; refresh its status", http.StatusConflict)
			return
		}
		http.Error(w, "device login could not be cancelled", http.StatusServiceUnavailable)
		return
	}
	if result, pollErr := h.proxyLifecycleHandlers().PollOpenAIDeviceAuth(r.Context(), flowID, binding); pollErr == nil {
		view := h.openAIDeviceStatusView(result)
		view.CSRFToken = csrfToken
		view.StatusValues = openAIDeviceStatusValues(csrfToken)
		render(r.Context(), w, pages.OpenAIDeviceAuthStatus(view))
		return
	}
	http.Error(w, "device login status unavailable", http.StatusServiceUnavailable)
}

// Status and cancel may confirm scope, never replace the stored flow identity.
func (h *UIHandler) validateOpenAIDeviceScope(r *http.Request, flowID, binding string) error {
	record, err := openaioauth.NewDeviceStore(h.Cache).Get(r.Context(), flowID)
	if err != nil && !errors.Is(err, openaioauth.ErrDeviceAuthExpired) {
		return err
	}
	if !record.OwnedBy(binding) {
		return openaioauth.ErrDeviceAuthOwnership
	}
	// Form contains every body and query value, including duplicate aliases.
	for _, scope := range r.Form["scope"] {
		if scope != "" && (scope != "global" || record.OrgID != "") {
			return openaioauth.ErrDeviceAuthOwnership
		}
	}
	for _, field := range []string{"org_id", "organization_id"} {
		for _, orgID := range r.Form[field] {
			if orgID != "" && orgID != record.OrgID {
				return openaioauth.ErrDeviceAuthOwnership
			}
		}
	}
	return nil
}

func (h *UIHandler) openAIDeviceCSRFToken(r *http.Request) (string, error) {
	if token := h.getSessionManager().GetString(r.Context(), openAIDeviceCSRFSessionKey); token != "" {
		return token, nil
	}
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return "", fmt.Errorf("generate device csrf token: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(value)
	h.getSessionManager().Put(r.Context(), openAIDeviceCSRFSessionKey, token)
	return token, nil
}

func (h *UIHandler) openAIDeviceSessionBinding(r *http.Request) (string, error) {
	token := h.getSessionManager().Token(r.Context())
	if strings.TrimSpace(token) == "" {
		return "", errors.New("session token unavailable")
	}
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:]), nil
}

func setOpenAIDeviceNoStore(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
}

func validOpenAIDeviceCSRF(r *http.Request, expected string) bool {
	provided := strings.TrimSpace(r.FormValue("csrf_token"))
	return provided != "" && subtle.ConstantTimeCompare([]byte(provided), []byte(expected)) == 1
}

func openAIDeviceStatusValues(csrfToken string) string {
	payload, _ := json.Marshal(map[string]string{"csrf_token": csrfToken})
	return string(payload)
}

func openAIDeviceStartErrorMessage(err error) string {
	switch {
	case errors.Is(err, proxyhandler.ErrOpenAIConnectDisabled):
		return proxyhandler.ErrOpenAIConnectDisabled.Error()
	case errors.Is(err, openai.ErrDeviceAuthUnavailable):
		return "Device-code login is not enabled for this OpenAI account or workspace."
	case errors.Is(err, proxyhandler.ErrOpenAIDeviceAuthPersistenceUnavailable):
		return "Device-code login is unavailable while credential persistence is offline."
	case errors.Is(err, openaioauth.ErrDeviceAuthCacheUnavailable):
		return "Device-code login is unavailable while shared session storage is offline."
	default:
		return "Device-code login is temporarily unavailable."
	}
}

func (h *UIHandler) renderOpenAIDeviceError(w http.ResponseWriter, r *http.Request, message string, status int, orgID string) {
	csrfToken, _ := h.openAIDeviceCSRFToken(r)
	if r.Header.Get("HX-Request") == "true" {
		status = http.StatusOK
	}
	w.WriteHeader(status)
	render(r.Context(), w, pages.OpenAIDeviceAuthError(pages.OpenAIDeviceAuthView{
		CSRFToken:      csrfToken,
		ErrorMessage:   message,
		CallbackURL:    h.openAIDeviceCallbackURL(orgID),
		OrganizationID: orgID,
	}))
}

func (h *UIHandler) openAIDeviceCallbackURL(orgID string) string {
	if h.Config == nil || !h.Config.GeneralSettings.OpenAIOAuth.Enabled {
		return ""
	}
	return openAIDeviceCallbackURLForOrg(orgID)
}

func openAIDeviceCallbackURLForOrg(orgID string) string {
	if orgID == "" {
		return "/ui/openai/connect?scope=global"
	}
	return "/ui/openai/connect?org_id=" + url.QueryEscape(orgID)
}

// Local observation may precede provider eligibility, but must see expiry
// within its fixed grace. Round up and floor at one second to avoid hot polling.
func openAIDeviceObservationInterval(intervalSeconds int64, expiresAt time.Time) int64 {
	if intervalSeconds <= 0 {
		intervalSeconds = 5
	}
	return max(1, min(intervalSeconds, int64(math.Ceil(time.Until(expiresAt).Seconds()))))
}

func openAIDeviceStartView(result proxyhandler.OpenAIDeviceAuthStart) pages.OpenAIDeviceAuthView {
	return pages.OpenAIDeviceAuthView{
		FlowID:          result.FlowID,
		VerificationURI: result.VerificationURI,
		UserCode:        result.UserCode,
		StatusURL:       "/ui/openai/device/status?flow_id=" + url.QueryEscape(result.FlowID),
		CancelURL:       "/ui/openai/device/cancel",
		CallbackURL:     openAIDeviceCallbackURLForOrg(result.OrganizationID),
		OrganizationID:  result.OrganizationID,
		ExpiresAt:       result.ExpiresAt.UTC().Format(time.RFC3339),
		IntervalSeconds: openAIDeviceObservationInterval(result.IntervalSeconds, result.ExpiresAt),
		Status:          "pending",
		Polling:         true,
	}
}

func (h *UIHandler) openAIDeviceStatusView(result proxyhandler.OpenAIDeviceAuthStatus) pages.OpenAIDeviceAuthView {
	callbackURL := h.openAIDeviceCallbackURL(result.OrganizationID)
	message := openAIDeviceStatusErrorMessage(result.ErrorCode)
	if callbackURL == "" && result.Status == openaioauth.DeviceAuthStatusPending {
		message = proxyhandler.ErrOpenAIConnectDisabled.Error()
	}
	return pages.OpenAIDeviceAuthView{
		FlowID:          result.FlowID,
		VerificationURI: result.VerificationURI,
		UserCode:        result.UserCode,
		StatusURL:       "/ui/openai/device/status?flow_id=" + url.QueryEscape(result.FlowID),
		CancelURL:       "/ui/openai/device/cancel",
		CallbackURL:     callbackURL,
		OrganizationID:  result.OrganizationID,
		ExpiresAt:       result.ExpiresAt.UTC().Format(time.RFC3339),
		IntervalSeconds: openAIDeviceObservationInterval(result.IntervalSeconds, result.ExpiresAt),
		Status:          string(result.Status),
		CredentialID:    result.CredentialID,
		ErrorMessage:    message,
		Polling:         (callbackURL != "" && result.Status == openaioauth.DeviceAuthStatusPending) || result.Status == openaioauth.DeviceAuthStatusExchanging,
	}
}

func openAIDeviceStatusErrorMessage(errorCode string) string {
	if errorCode == "" {
		return ""
	}
	switch errorCode {
	case "access_denied":
		return "OpenAI declined this device authorization."
	case "expired", "expired_token":
		return "The device code expired. Start a new device login."
	case "slow_down":
		return "OpenAI asked Tianji to slow down. The next check will use a longer interval."
	case "provider_unavailable", "temporarily_unavailable":
		return "OpenAI is temporarily unavailable. Start a new login or use the browser callback."
	case "save_failed":
		return "OpenAI authorized the login, but Tianji could not save the credential."
	case "exchange_failed":
		return "OpenAI authorization could not be exchanged for a credential."
	default:
		return "The device login could not be completed."
	}
}
