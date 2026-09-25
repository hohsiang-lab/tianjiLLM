package ui

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"golang.org/x/oauth2"

	"github.com/praxisllmlab/tianjiLLM/internal/auth/social"
	"github.com/praxisllmlab/tianjiLLM/internal/db"
)

const (
	oauthProviderKey  = "oauth_provider"
	oauthStateKey     = "oauth_state"
	oauthVerifierKey  = "oauth_verifier"
	oauthStartedAtKey = "oauth_started_at"
	oauthStateTTL     = 10 * time.Minute

	loginErrorNotConfigured  = "not_configured"
	loginErrorInvalidRequest = "invalid_request"
	loginErrorCancelled      = "cancelled"
	loginErrorMissingCode    = "missing_code"
	loginErrorProviderFailed = "provider_failed"
	loginErrorUnauthorized   = "unauthorized"
)

// RegisterAuthRoutes mounts the social login entry points at /auth. These
// routes use the same SCS session as /ui, but live outside the /ui prefix so
// their callback URLs remain conventional and provider-friendly.
func (h *UIHandler) RegisterAuthRoutes(r chi.Router) {
	r.Use(h.getSessionManager().LoadAndSave)
	r.Get("/{provider}", h.handleSocialLogin)
	r.Get("/{provider}/callback", h.handleSocialCallback)
}

func (h *UIHandler) socialProviders() []string {
	if h.SocialAuth == nil {
		return nil
	}
	return h.SocialAuth.Providers()
}

func (h *UIHandler) socialAuthEnabled() bool {
	return h.Config != nil &&
		h.Config.GeneralSettings.SocialAuth.Enabled &&
		h.SocialAuth != nil &&
		len(h.SocialAuth.Providers()) > 0
}

func (h *UIHandler) masterKeyLoginEnabled() bool {
	if h.Config == nil {
		return true
	}
	cfg := h.Config.GeneralSettings.SocialAuth
	if cfg.MasterKeyLoginEnabled != nil {
		return *cfg.MasterKeyLoginEnabled
	}
	return !cfg.Enabled
}

func (h *UIHandler) handleSocialLogin(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if !h.socialAuthEnabled() {
		http.NotFound(w, r)
		return
	}
	provider := chi.URLParam(r, "provider")
	state, err := randomOAuthState()
	if err != nil {
		http.Error(w, "failed to start login", http.StatusInternalServerError)
		return
	}
	verifier := oauth2.GenerateVerifier()
	redirectURL, err := h.SocialAuth.Begin(provider, state, verifier)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	sessionManager := h.getSessionManager()
	if err := sessionManager.RenewToken(r.Context()); err != nil {
		http.Error(w, "failed to start login", http.StatusInternalServerError)
		return
	}
	sessionManager.Put(r.Context(), oauthProviderKey, provider)
	sessionManager.Put(r.Context(), oauthStateKey, state)
	sessionManager.Put(r.Context(), oauthVerifierKey, verifier)
	sessionManager.Put(r.Context(), oauthStartedAtKey, time.Now().Unix())
	http.Redirect(w, r, redirectURL, http.StatusFound)
}

func (h *UIHandler) handleSocialCallback(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if !h.socialAuthEnabled() || h.DB == nil {
		h.redirectLoginError(w, r, loginErrorNotConfigured)
		return
	}
	provider := chi.URLParam(r, "provider")
	sessionManager := h.getSessionManager()
	expectedProvider := sessionManager.GetString(r.Context(), oauthProviderKey)
	expectedState := sessionManager.GetString(r.Context(), oauthStateKey)
	verifier := sessionManager.GetString(r.Context(), oauthVerifierKey)
	startedAt := sessionManager.GetInt64(r.Context(), oauthStartedAtKey)
	h.consumeOAuthState(r)

	if provider == "" || provider != expectedProvider ||
		!secureStringEqual(expectedState, r.URL.Query().Get("state")) ||
		verifier == "" || oauthStateIsExpired(startedAt, time.Now()) {
		h.redirectLoginError(w, r, loginErrorInvalidRequest)
		return
	}
	if providerError := r.URL.Query().Get("error"); providerError != "" {
		h.redirectLoginError(w, r, loginErrorCancelled)
		return
	}
	code := r.URL.Query().Get("code")
	if code == "" {
		h.redirectLoginError(w, r, loginErrorMissingCode)
		return
	}

	identity, err := h.SocialAuth.Complete(r.Context(), provider, code, verifier)
	if err != nil {
		h.redirectLoginError(w, r, loginErrorProviderFailed)
		return
	}
	user, err := h.resolveSocialIdentity(r, identity)
	if err != nil {
		h.redirectLoginError(w, r, loginErrorUnauthorized)
		return
	}

	if err := sessionManager.RenewToken(r.Context()); err != nil {
		http.Error(w, "failed to create session", http.StatusInternalServerError)
		return
	}
	sessionManager.Put(r.Context(), sessionAuthenticatedKey, true)
	sessionManager.Put(r.Context(), sessionRoleKey, user.UserRole)
	sessionManager.Put(r.Context(), sessionUserIDKey, user.UserID)
	sessionManager.Put(r.Context(), sessionAuthVersionKey, user.AuthVersion)
	http.Redirect(w, r, "/ui/", http.StatusSeeOther)
}

func (h *UIHandler) resolveSocialIdentity(r *http.Request, identity social.Identity) (db.UserTable, error) {
	ctx := r.Context()
	row, err := h.DB.GetUserIdentityByProviderSubject(ctx, db.GetUserIdentityByProviderSubjectParams{
		Provider:       identity.Provider,
		ProviderUserID: identity.ProviderUserID,
	})
	if err == nil {
		user, userErr := h.DB.GetUser(ctx, row.UserID)
		if userErr != nil || userStatusFromMetadata(user.Metadata) != "active" {
			return db.UserTable{}, errors.New("user is disabled or unavailable")
		}
		if !isAssignableUIRole(user.UserRole) {
			return db.UserTable{}, errors.New("user role is not authorized for the dashboard")
		}
		_, _ = h.DB.UpdateUserIdentityLogin(ctx, updateIdentityParams(identity))
		return user, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return db.UserTable{}, errors.New("unable to resolve user identity")
	}

	cfg := h.Config.GeneralSettings.SocialAuth
	if !identity.EmailVerified {
		return db.UserTable{}, errors.New("this account has not been linked by an administrator")
	}
	users, err := h.DB.ListUsersByVerifiedEmailCandidate(ctx, identity.Email)
	if err != nil || len(users) != 1 {
		return db.UserTable{}, errors.New("no unique invited user matches this verified email")
	}
	user := users[0]
	if userStatusFromMetadata(user.Metadata) != "active" {
		return db.UserTable{}, errors.New("user is disabled")
	}
	if !isAssignableUIRole(user.UserRole) {
		return db.UserTable{}, errors.New("user role is not authorized for the dashboard")
	}
	explicitEnrollment := userSocialAuthEnrollmentEnabled(user.Metadata)
	if !cfg.AllowVerifiedEmailLink && !explicitEnrollment {
		return db.UserTable{}, errors.New("this account has not been linked by an administrator")
	}
	return h.createSocialIdentity(ctx, user, identity, explicitEnrollment)
}

func (h *UIHandler) createSocialIdentity(
	ctx context.Context,
	user db.UserTable,
	identity social.Identity,
	explicitEnrollment bool,
) (db.UserTable, error) {
	userID := user.UserID
	queries := h.DB
	var tx pgx.Tx
	if h.Pool != nil {
		var err error
		tx, err = h.Pool.Begin(ctx)
		if err != nil {
			return db.UserTable{}, errors.New("unable to link provider identity")
		}
		defer func() { _ = tx.Rollback(ctx) }()

		// Serialize provider-link attempts for this local user.
		if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, userID); err != nil {
			return db.UserTable{}, errors.New("unable to link provider identity")
		}
		queries = h.DB.WithTx(tx)
		user, err = queries.GetUser(ctx, userID)
		if err != nil ||
			userStatusFromMetadata(user.Metadata) != "active" ||
			!isAssignableUIRole(user.UserRole) {
			return db.UserTable{}, errors.New("user is disabled or unavailable")
		}
	}
	if explicitEnrollment && !userSocialAuthEnrollmentEnabled(user.Metadata) {
		return db.UserTable{}, errors.New("social sign-in enrollment is closed")
	}

	count, err := queries.CountUserIdentitiesByUser(ctx, userID)
	if err != nil {
		return db.UserTable{}, errors.New("unable to link provider identity")
	}
	if count != 0 && !explicitEnrollment {
		return db.UserTable{}, errors.New("additional identity linking requires an administrator")
	}

	_, err = queries.CreateUserIdentity(ctx, db.CreateUserIdentityParams{
		IdentityID:     uuid.NewString(),
		UserID:         userID,
		Provider:       identity.Provider,
		ProviderUserID: identity.ProviderUserID,
		ProviderEmail:  stringPointer(identity.Email),
		EmailVerified:  identity.EmailVerified,
		DisplayName:    stringPointer(identity.DisplayName),
		AvatarUrl:      stringPointer(identity.AvatarURL),
	})
	if err != nil {
		return db.UserTable{}, errors.New("unable to link provider identity")
	}
	if explicitEnrollment {
		user, err = queries.SetUserSocialAuthEnrollment(ctx, db.SetUserSocialAuthEnrollmentParams{
			Enabled:   false,
			UpdatedBy: "social-auth",
			UserID:    userID,
		})
		if err != nil {
			return db.UserTable{}, errors.New("unable to close social sign-in enrollment")
		}
	}
	if tx != nil {
		if err := tx.Commit(ctx); err != nil {
			return db.UserTable{}, errors.New("unable to link provider identity")
		}
	}
	return user, nil
}

func updateIdentityParams(identity social.Identity) db.UpdateUserIdentityLoginParams {
	return db.UpdateUserIdentityLoginParams{
		Provider:       identity.Provider,
		ProviderUserID: identity.ProviderUserID,
		ProviderEmail:  stringPointer(identity.Email),
		EmailVerified:  identity.EmailVerified,
		DisplayName:    stringPointer(identity.DisplayName),
		AvatarUrl:      stringPointer(identity.AvatarURL),
	}
}

func stringPointer(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func (h *UIHandler) consumeOAuthState(r *http.Request) {
	sessionManager := h.getSessionManager()
	sessionManager.Remove(r.Context(), oauthProviderKey)
	sessionManager.Remove(r.Context(), oauthStateKey)
	sessionManager.Remove(r.Context(), oauthVerifierKey)
	sessionManager.Remove(r.Context(), oauthStartedAtKey)
}

func (h *UIHandler) redirectLoginError(w http.ResponseWriter, r *http.Request, code string) {
	http.Redirect(w, r, "/ui/login?error="+url.QueryEscape(code), http.StatusSeeOther)
}

func loginErrorMessage(code string) string {
	switch code {
	case loginErrorNotConfigured:
		return "Social login is not configured"
	case loginErrorInvalidRequest:
		return "Login request expired or is invalid"
	case loginErrorCancelled:
		return "Login was cancelled"
	case loginErrorMissingCode:
		return "Provider returned no authorization code"
	case loginErrorProviderFailed:
		return "Provider authentication failed"
	case loginErrorUnauthorized:
		return "This account is not authorized"
	default:
		return ""
	}
}

func randomOAuthState() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate state: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func secureStringEqual(expected, actual string) bool {
	if expected == "" || len(expected) != len(actual) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(expected), []byte(actual)) == 1
}

func oauthStateIsExpired(startedAt int64, now time.Time) bool {
	if startedAt == 0 {
		return true
	}
	started := time.Unix(startedAt, 0)
	return started.After(now.Add(time.Minute)) || now.Sub(started) > oauthStateTTL
}
