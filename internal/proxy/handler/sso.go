package handler

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"

	"github.com/praxisllmlab/tianjiLLM/internal/auth"
	"github.com/praxisllmlab/tianjiLLM/internal/model"
)

// SSOHandler holds SSO-specific dependencies.
type SSOHandler struct {
	SSO *auth.SSOHandler
}

// SSOLogin handles GET /sso/login — redirects to IDP authorization URL.
func (h *Handlers) SSOLogin(w http.ResponseWriter, r *http.Request) {
	if h.SSOHandler == nil {
		writeJSON(w, http.StatusNotImplemented, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "SSO not configured", Type: "not_implemented"},
		})
		return
	}

	state := generateState()
	loginURL := h.SSOHandler.SSO.LoginURL(state)

	// Store state in cookie for CSRF verification on callback
	http.SetCookie(w, &http.Cookie{
		Name:     "tianji_sso_state",
		Value:    state,
		Path:     "/sso",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})

	http.Redirect(w, r, loginURL, http.StatusFound)
}

// SSOCallback handles GET /sso/callback — exchanges code for tokens.
func (h *Handlers) SSOCallback(w http.ResponseWriter, r *http.Request) {
	if h.SSOHandler == nil {
		writeJSON(w, http.StatusNotImplemented, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "SSO not configured", Type: "not_implemented"},
		})
		return
	}

	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")

	if code == "" {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "missing authorization code", Type: "invalid_request_error"},
		})
		return
	}

	// Verify CSRF state
	cookie, err := r.Cookie("tianji_sso_state")
	if err != nil || cookie.Value != state {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "invalid state parameter", Type: "invalid_request_error"},
		})
		return
	}

	// Exchange code for tokens
	tokenResp, err := h.SSOHandler.SSO.ExchangeCode(r.Context(), code)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "token exchange failed: " + err.Error(), Type: "internal_error"},
		})
		return
	}

	// Get user info
	userInfo, err := h.SSOHandler.SSO.GetUserInfo(r.Context(), tokenResp.AccessToken)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "user info failed: " + err.Error(), Type: "internal_error"},
		})
		return
	}

	// Map IDP groups to TianjiLLM role
	role := h.SSOHandler.SSO.MapRole(userInfo.Groups)

	// Clear the state cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "tianji_sso_state",
		Value:    "",
		Path:     "/sso",
		MaxAge:   -1,
		HttpOnly: true,
	})

	writeJSON(w, http.StatusOK, map[string]any{
		"user_id":      userInfo.Sub,
		"user_email":   userInfo.Email,
		"user_name":    userInfo.Name,
		"role":         string(role),
		"groups":       userInfo.Groups,
		"access_token": tokenResp.AccessToken,
		"id_token":     tokenResp.IDToken,
		"expires_in":   tokenResp.ExpiresIn,
	})
}

func generateState() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
