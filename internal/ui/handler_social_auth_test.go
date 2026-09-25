package ui

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"

	"github.com/praxisllmlab/tianjiLLM/internal/auth/social"
	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/praxisllmlab/tianjiLLM/internal/db"
)

type stubSocialProvider struct {
	name          string
	identity      social.Identity
	completeErr   error
	mu            sync.Mutex
	beginVerifier string
	lastVerifier  string
	completeCalls int
}

func (p *stubSocialProvider) Name() string {
	return p.name
}

func (p *stubSocialProvider) AuthorizationURL(state, verifier string) string {
	p.mu.Lock()
	p.beginVerifier = verifier
	p.mu.Unlock()
	values := url.Values{
		"state":                 {state},
		"code_challenge":        {oauth2.S256ChallengeFromVerifier(verifier)},
		"code_challenge_method": {"S256"},
	}
	return "https://provider.example/authorize?" + values.Encode()
}

func (p *stubSocialProvider) Complete(_ context.Context, _, verifier string) (social.Identity, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.completeCalls++
	p.lastVerifier = verifier
	return p.identity, p.completeErr
}

func (p *stubSocialProvider) snapshot() (beginVerifier, lastVerifier string, calls int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.beginVerifier, p.lastVerifier, p.completeCalls
}

func boolPointer(value bool) *bool {
	return &value
}

func newSocialAuthTestHandler(t *testing.T, provider social.Provider) *UIHandler {
	t.Helper()
	manager, cleanup := NewSessionManager(nil, false)
	t.Cleanup(cleanup)
	return &UIHandler{
		Config: &config.ProxyConfig{GeneralSettings: config.GeneralSettings{
			MasterKey: "test-master-key",
			SocialAuth: config.SocialAuthConfig{
				Enabled:               true,
				PublicBaseURL:         "https://tianji.example",
				MasterKeyLoginEnabled: boolPointer(false),
			},
		}},
		MasterKey:      "test-master-key",
		SessionManager: manager,
		SocialAuth:     social.NewService(provider),
	}
}

func socialAuthTestRouter(h *UIHandler) chi.Router {
	router := chi.NewRouter()
	router.Route("/auth", h.RegisterAuthRoutes)
	router.Route("/ui", h.RegisterRoutes)
	return router
}

func beginSocialLogin(t *testing.T, router http.Handler, provider string) (*http.Cookie, string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/auth/"+provider, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusFound, w.Code, w.Body.String())
	require.Equal(t, "no-store", w.Header().Get("Cache-Control"))

	location, err := url.Parse(w.Header().Get("Location"))
	require.NoError(t, err)
	state := location.Query().Get("state")
	require.NotEmpty(t, state)
	require.Equal(t, "S256", location.Query().Get("code_challenge_method"))
	require.NotEmpty(t, location.Query().Get("code_challenge"))
	return responseCookie(t, w, sessionCookieName), state
}

func responseCookie(t *testing.T, w *httptest.ResponseRecorder, name string) *http.Cookie {
	t.Helper()
	for _, cookie := range w.Result().Cookies() {
		if cookie.Name == name {
			return cookie
		}
	}
	t.Fatalf("cookie %q not found", name)
	return nil
}

func createOAuthStateCookie(
	t *testing.T,
	h *UIHandler,
	provider string,
	state string,
	verifier string,
	startedAt time.Time,
) *http.Cookie {
	t.Helper()
	handler := h.getSessionManager().LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.getSessionManager().Put(r.Context(), oauthProviderKey, provider)
		h.getSessionManager().Put(r.Context(), oauthStateKey, state)
		h.getSessionManager().Put(r.Context(), oauthVerifierKey, verifier)
		h.getSessionManager().Put(r.Context(), oauthStartedAtKey, startedAt.Unix())
		w.WriteHeader(http.StatusNoContent)
	}))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/test/oauth-state", nil))
	require.Equal(t, http.StatusNoContent, w.Code)
	return responseCookie(t, w, sessionCookieName)
}

func TestSocialLoginUnknownProviderReturnsNotFound(t *testing.T) {
	provider := &stubSocialProvider{name: "github"}
	h := newSocialAuthTestHandler(t, provider)

	req := httptest.NewRequest(http.MethodGet, "/auth/unknown", nil)
	w := httptest.NewRecorder()
	socialAuthTestRouter(h).ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	_, _, calls := provider.snapshot()
	assert.Zero(t, calls)
}

func TestSocialCallbackConsumesStateOnMismatchAndReplay(t *testing.T) {
	provider := &stubSocialProvider{name: "github"}
	h := newSocialAuthTestHandler(t, provider)
	router := socialAuthTestRouter(h)
	cookie, state := beginSocialLogin(t, router, "github")

	badReq := httptest.NewRequest(http.MethodGet, "/auth/github/callback?code=code&state=wrong", nil)
	badReq.AddCookie(cookie)
	badW := httptest.NewRecorder()
	router.ServeHTTP(badW, badReq)
	require.Equal(t, http.StatusSeeOther, badW.Code)
	assert.Contains(t, badW.Header().Get("Location"), "/ui/login?error=")

	replayReq := httptest.NewRequest(http.MethodGet, "/auth/github/callback?code=code&state="+url.QueryEscape(state), nil)
	replayReq.AddCookie(cookie)
	replayW := httptest.NewRecorder()
	router.ServeHTTP(replayW, replayReq)
	assert.Equal(t, http.StatusSeeOther, replayW.Code)

	_, _, calls := provider.snapshot()
	assert.Zero(t, calls, "provider exchange must not run after invalid or replayed state")
}

func TestSocialCallbackRejectsExpiredState(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	t.Cleanup(func() { mock.Close() })

	provider := &stubSocialProvider{name: "discord"}
	h := newSocialAuthTestHandler(t, provider)
	h.DB = db.New(mock)
	state := "expired-state"
	cookie := createOAuthStateCookie(
		t,
		h,
		"discord",
		state,
		oauth2.GenerateVerifier(),
		time.Now().Add(-oauthStateTTL-time.Minute),
	)

	req := httptest.NewRequest(http.MethodGet, "/auth/discord/callback?code=code&state="+state, nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	socialAuthTestRouter(h).ServeHTTP(w, req)

	assert.Equal(t, http.StatusSeeOther, w.Code)
	assert.Contains(t, w.Header().Get("Location"), "error=invalid_request")
	_, _, calls := provider.snapshot()
	assert.Zero(t, calls)
}

func TestOAuthStateExpiryRejectsMissingOldAndFutureTimestamps(t *testing.T) {
	now := time.Now()
	assert.True(t, oauthStateIsExpired(0, now))
	assert.True(t, oauthStateIsExpired(now.Add(-oauthStateTTL-time.Second).Unix(), now))
	assert.True(t, oauthStateIsExpired(now.Add(2*time.Minute).Unix(), now))
	assert.False(t, oauthStateIsExpired(now.Add(-time.Minute).Unix(), now))
}

func TestSocialCallbackCreatesUserSessionAndCannotReplay(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	t.Cleanup(func() { mock.Close() })

	provider := &stubSocialProvider{
		name: "github",
		identity: social.Identity{
			Provider:       "github",
			ProviderUserID: "provider-user-1",
			Email:          "user@example.com",
			EmailVerified:  true,
			DisplayName:    "Tianji User",
			AvatarURL:      "https://avatars.example/user",
		},
	}
	h := newSocialAuthTestHandler(t, provider)
	h.DB = db.New(mock)
	router := socialAuthTestRouter(h)

	now := time.Now()
	identityColumns := []string{
		"identity_id", "user_id", "provider", "provider_user_id", "provider_email",
		"email_verified", "display_name", "avatar_url", "last_login_at", "created_at", "updated_at",
	}
	userColumns := []string{
		"user_id", "user_alias", "user_email", "user_role", "teams", "max_budget",
		"spend", "models", "metadata", "tpm_limit", "rpm_limit", "budget_duration",
		"budget_reset_at", "budget_id", "created_at", "created_by", "updated_at", "updated_by", "auth_version",
	}
	providerEmail := "user@example.com"
	displayName := "Tianji User"
	avatarURL := "https://avatars.example/user"
	identityRow := func() *pgxmock.Rows {
		return pgxmock.NewRows(identityColumns).AddRow(
			"identity-1", "user-1", "github", "provider-user-1", &providerEmail,
			true, &displayName, &avatarURL, now, now, now,
		)
	}
	mock.ExpectQuery(`(?s)SELECT .*FROM "UserIdentityTable"`).
		WithArgs("github", "provider-user-1").
		WillReturnRows(identityRow())
	mock.ExpectQuery(`SELECT .+ FROM "UserTable" WHERE user_id = \$1`).
		WithArgs("user-1").
		WillReturnRows(pgxmock.NewRows(userColumns).AddRow(
			"user-1", &displayName, &providerEmail, "internal_user_viewer", []string{},
			nil, float64(0), []string{}, []byte(`{"status":"active"}`), nil, nil, nil,
			nil, nil, now, "admin", now, "admin", int64(7),
		))
	mock.ExpectQuery(`(?s)UPDATE "UserIdentityTable"`).
		WithArgs(
			"github",
			"provider-user-1",
			pgxmock.AnyArg(),
			true,
			pgxmock.AnyArg(),
			pgxmock.AnyArg(),
		).
		WillReturnRows(identityRow())

	loginCookie, state := beginSocialLogin(t, router, "github")
	callbackReq := httptest.NewRequest(
		http.MethodGet,
		"/auth/github/callback?code=authorization-code&state="+url.QueryEscape(state),
		nil,
	)
	callbackReq.AddCookie(loginCookie)
	callbackW := httptest.NewRecorder()
	router.ServeHTTP(callbackW, callbackReq)
	require.Equal(t, http.StatusSeeOther, callbackW.Code, callbackW.Body.String())
	assert.Equal(t, "/ui/", callbackW.Header().Get("Location"))
	sessionCookie := responseCookie(t, callbackW, sessionCookieName)

	var authenticated bool
	var userID, role, oauthState, oauthVerifier, accessToken string
	var authVersion int64
	inspect := h.getSessionManager().LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authenticated = h.getSessionManager().GetBool(r.Context(), sessionAuthenticatedKey)
		userID = h.getSessionManager().GetString(r.Context(), sessionUserIDKey)
		role = h.getSessionManager().GetString(r.Context(), sessionRoleKey)
		authVersion = h.getSessionManager().GetInt64(r.Context(), sessionAuthVersionKey)
		oauthState = h.getSessionManager().GetString(r.Context(), oauthStateKey)
		oauthVerifier = h.getSessionManager().GetString(r.Context(), oauthVerifierKey)
		accessToken = h.getSessionManager().GetString(r.Context(), "access_token")
		w.WriteHeader(http.StatusNoContent)
	}))
	inspectReq := httptest.NewRequest(http.MethodGet, "/inspect", nil)
	inspectReq.AddCookie(sessionCookie)
	inspectW := httptest.NewRecorder()
	inspect.ServeHTTP(inspectW, inspectReq)
	require.Equal(t, http.StatusNoContent, inspectW.Code)
	assert.True(t, authenticated)
	assert.Equal(t, "user-1", userID)
	assert.Equal(t, "internal_user_viewer", role)
	assert.Equal(t, int64(7), authVersion)
	assert.Empty(t, oauthState)
	assert.Empty(t, oauthVerifier)
	assert.Empty(t, accessToken, "provider access tokens must never be stored in the UI session")

	beginVerifier, lastVerifier, calls := provider.snapshot()
	assert.Equal(t, beginVerifier, lastVerifier)
	assert.NotEmpty(t, lastVerifier)
	assert.Equal(t, 1, calls)

	replayReq := httptest.NewRequest(
		http.MethodGet,
		"/auth/github/callback?code=authorization-code&state="+url.QueryEscape(state),
		nil,
	)
	replayReq.AddCookie(sessionCookie)
	replayW := httptest.NewRecorder()
	router.ServeHTTP(replayW, replayReq)
	assert.Equal(t, http.StatusSeeOther, replayW.Code)
	_, _, calls = provider.snapshot()
	assert.Equal(t, 1, calls)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestResolveSocialIdentityLinksExactlyOneInvitedUser(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	t.Cleanup(func() { mock.Close() })

	h := newSocialAuthTestHandler(t, &stubSocialProvider{name: "github"})
	h.Config.GeneralSettings.SocialAuth.AllowVerifiedEmailLink = true
	h.DB = db.New(mock)

	identity := social.Identity{
		Provider:       "github",
		ProviderUserID: "provider-user-2",
		Email:          "invited@example.com",
		EmailVerified:  true,
		DisplayName:    "Invited User",
		AvatarURL:      "https://avatars.example/invited",
	}
	now := time.Now()
	email := identity.Email
	displayName := identity.DisplayName
	avatarURL := identity.AvatarURL
	userColumns := []string{
		"user_id", "user_alias", "user_email", "user_role", "teams", "max_budget",
		"spend", "models", "metadata", "tpm_limit", "rpm_limit", "budget_duration",
		"budget_reset_at", "budget_id", "created_at", "created_by", "updated_at", "updated_by", "auth_version",
	}
	identityColumns := []string{
		"identity_id", "user_id", "provider", "provider_user_id", "provider_email",
		"email_verified", "display_name", "avatar_url", "last_login_at", "created_at", "updated_at",
	}

	mock.ExpectQuery(`(?s)SELECT .*FROM "UserIdentityTable"`).
		WithArgs("github", "provider-user-2").
		WillReturnError(pgx.ErrNoRows)
	mock.ExpectQuery(`(?s)SELECT .*FROM "UserTable".*LOWER\(user_email\)`).
		WithArgs("invited@example.com").
		WillReturnRows(pgxmock.NewRows(userColumns).AddRow(
			"user-2", &displayName, &email, "internal_user", []string{},
			nil, float64(0), []string{}, []byte(`{"status":"active"}`), nil, nil, nil,
			nil, nil, now, "admin", now, "admin", int64(3),
		))
	mock.ExpectQuery(`(?s)SELECT COUNT\(\*\).*FROM "UserIdentityTable"`).
		WithArgs("user-2").
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(int64(0)))
	mock.ExpectQuery(`(?s)INSERT INTO "UserIdentityTable"`).
		WithArgs(
			pgxmock.AnyArg(),
			"user-2",
			"github",
			"provider-user-2",
			pgxmock.AnyArg(),
			true,
			pgxmock.AnyArg(),
			pgxmock.AnyArg(),
		).
		WillReturnRows(pgxmock.NewRows(identityColumns).AddRow(
			"identity-2", "user-2", "github", "provider-user-2", &email,
			true, &displayName, &avatarURL, now, now, now,
		))

	req := httptest.NewRequest(http.MethodGet, "/auth/github/callback", nil)
	user, err := h.resolveSocialIdentity(req, identity)
	require.NoError(t, err)
	assert.Equal(t, "user-2", user.UserID)
	assert.Equal(t, "internal_user", user.UserRole)
	assert.Equal(t, int64(3), user.AuthVersion)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestResolveSocialIdentityConsumesExplicitEnrollmentAndAllowsAdditionalProvider(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	t.Cleanup(func() { mock.Close() })

	h := newSocialAuthTestHandler(t, &stubSocialProvider{name: "discord"})
	h.Config.GeneralSettings.SocialAuth.AllowVerifiedEmailLink = false
	h.DB = db.New(mock)

	identity := social.Identity{
		Provider:       "discord",
		ProviderUserID: "discord-user-2",
		Email:          "invited@example.com",
		EmailVerified:  true,
		DisplayName:    "Invited User",
		AvatarURL:      "https://avatars.example/invited",
	}
	now := time.Now()
	email := identity.Email
	displayName := identity.DisplayName
	avatarURL := identity.AvatarURL
	userColumns := []string{
		"user_id", "user_alias", "user_email", "user_role", "teams", "max_budget",
		"spend", "models", "metadata", "tpm_limit", "rpm_limit", "budget_duration",
		"budget_reset_at", "budget_id", "created_at", "created_by", "updated_at", "updated_by", "auth_version",
	}
	identityColumns := []string{
		"identity_id", "user_id", "provider", "provider_user_id", "provider_email",
		"email_verified", "display_name", "avatar_url", "last_login_at", "created_at", "updated_at",
	}

	mock.ExpectQuery(`(?s)SELECT .*FROM "UserIdentityTable"`).
		WithArgs("discord", "discord-user-2").
		WillReturnError(pgx.ErrNoRows)
	mock.ExpectQuery(`(?s)SELECT .*FROM "UserTable".*LOWER\(user_email\)`).
		WithArgs(email).
		WillReturnRows(pgxmock.NewRows(userColumns).AddRow(
			"user-2", &displayName, &email, "internal_user", []string{},
			nil, float64(0), []string{},
			[]byte(`{"status":"active","social_auth_enrollment_enabled":true}`),
			nil, nil, nil, nil, nil, now, "admin", now, "admin", int64(3),
		))
	mock.ExpectQuery(`(?s)SELECT COUNT\(\*\).*FROM "UserIdentityTable"`).
		WithArgs("user-2").
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(int64(1)))
	mock.ExpectQuery(`(?s)INSERT INTO "UserIdentityTable"`).
		WithArgs(
			pgxmock.AnyArg(),
			"user-2",
			"discord",
			"discord-user-2",
			pgxmock.AnyArg(),
			true,
			pgxmock.AnyArg(),
			pgxmock.AnyArg(),
		).
		WillReturnRows(pgxmock.NewRows(identityColumns).AddRow(
			"identity-2", "user-2", "discord", "discord-user-2", &email,
			true, &displayName, &avatarURL, now, now, now,
		))
	mock.ExpectQuery(`(?s)UPDATE "UserTable".*social_auth_enrollment_enabled.*RETURNING`).
		WithArgs(false, "social-auth", "user-2").
		WillReturnRows(pgxmock.NewRows(userColumns).AddRow(
			"user-2", &displayName, &email, "internal_user", []string{},
			nil, float64(0), []string{},
			[]byte(`{"status":"active","social_auth_enrollment_enabled":false}`),
			nil, nil, nil, nil, nil, now, "admin", now, "social-auth", int64(3),
		))

	req := httptest.NewRequest(http.MethodGet, "/auth/discord/callback", nil)
	user, err := h.resolveSocialIdentity(req, identity)
	require.NoError(t, err)
	assert.Equal(t, "user-2", user.UserID)
	assert.Equal(t, int64(3), user.AuthVersion)
	assert.False(t, userSocialAuthEnrollmentEnabled(user.Metadata))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestResolveSocialIdentityRejectsClosedExplicitEnrollment(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	t.Cleanup(func() { mock.Close() })

	h := newSocialAuthTestHandler(t, &stubSocialProvider{name: "discord"})
	h.Config.GeneralSettings.SocialAuth.AllowVerifiedEmailLink = false
	h.DB = db.New(mock)

	email := "invited@example.com"
	displayName := "Invited User"
	now := time.Now()
	userColumns := []string{
		"user_id", "user_alias", "user_email", "user_role", "teams", "max_budget",
		"spend", "models", "metadata", "tpm_limit", "rpm_limit", "budget_duration",
		"budget_reset_at", "budget_id", "created_at", "created_by", "updated_at", "updated_by", "auth_version",
	}

	mock.ExpectQuery(`(?s)SELECT .*FROM "UserIdentityTable"`).
		WithArgs("discord", "discord-user-2").
		WillReturnError(pgx.ErrNoRows)
	mock.ExpectQuery(`(?s)SELECT .*FROM "UserTable".*LOWER\(user_email\)`).
		WithArgs(email).
		WillReturnRows(pgxmock.NewRows(userColumns).AddRow(
			"user-2", &displayName, &email, "internal_user", []string{},
			nil, float64(0), []string{},
			[]byte(`{"status":"active","social_auth_enrollment_enabled":false}`),
			nil, nil, nil, nil, nil, now, "admin", now, "admin", int64(3),
		))

	req := httptest.NewRequest(http.MethodGet, "/auth/discord/callback", nil)
	_, err = h.resolveSocialIdentity(req, social.Identity{
		Provider:       "discord",
		ProviderUserID: "discord-user-2",
		Email:          email,
		EmailVerified:  true,
	})
	assert.ErrorContains(t, err, "has not been linked by an administrator")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestResolveSocialIdentityRejectsAdditionalAutomaticLink(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	t.Cleanup(func() { mock.Close() })

	h := newSocialAuthTestHandler(t, &stubSocialProvider{name: "discord"})
	h.Config.GeneralSettings.SocialAuth.AllowVerifiedEmailLink = true
	h.DB = db.New(mock)

	email := "invited@example.com"
	displayName := "Invited User"
	now := time.Now()
	userColumns := []string{
		"user_id", "user_alias", "user_email", "user_role", "teams", "max_budget",
		"spend", "models", "metadata", "tpm_limit", "rpm_limit", "budget_duration",
		"budget_reset_at", "budget_id", "created_at", "created_by", "updated_at", "updated_by", "auth_version",
	}

	mock.ExpectQuery(`(?s)SELECT .*FROM "UserIdentityTable"`).
		WithArgs("discord", "discord-user-2").
		WillReturnError(pgx.ErrNoRows)
	mock.ExpectQuery(`(?s)SELECT .*FROM "UserTable".*LOWER\(user_email\)`).
		WithArgs(email).
		WillReturnRows(pgxmock.NewRows(userColumns).AddRow(
			"user-2", &displayName, &email, "internal_user", []string{},
			nil, float64(0), []string{}, []byte(`{"status":"active"}`), nil, nil, nil,
			nil, nil, now, "admin", now, "admin", int64(3),
		))
	mock.ExpectQuery(`(?s)SELECT COUNT\(\*\).*FROM "UserIdentityTable"`).
		WithArgs("user-2").
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(int64(1)))

	req := httptest.NewRequest(http.MethodGet, "/auth/discord/callback", nil)
	_, err = h.resolveSocialIdentity(req, social.Identity{
		Provider:       "discord",
		ProviderUserID: "discord-user-2",
		Email:          email,
		EmailVerified:  true,
	})
	assert.ErrorContains(t, err, "requires an administrator")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSocialCallbackProviderFailureIsSafe(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	t.Cleanup(func() { mock.Close() })

	provider := &stubSocialProvider{
		name:        "github",
		completeErr: errors.New("upstream included sensitive token: secret-token"),
	}
	h := newSocialAuthTestHandler(t, provider)
	h.DB = db.New(mock)
	router := socialAuthTestRouter(h)
	cookie, state := beginSocialLogin(t, router, "github")

	req := httptest.NewRequest(
		http.MethodGet,
		"/auth/github/callback?code=code&state="+url.QueryEscape(state),
		nil,
	)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusSeeOther, w.Code)
	assert.Contains(t, w.Header().Get("Location"), "error=provider_failed")
	assert.NotContains(t, w.Header().Get("Location"), "secret-token")
}

func TestMasterKeyLoginDefaultsAndBreakGlassEnforcement(t *testing.T) {
	assert.True(t, (&UIHandler{}).masterKeyLoginEnabled())

	enabledConfig := &config.ProxyConfig{GeneralSettings: config.GeneralSettings{
		SocialAuth: config.SocialAuthConfig{Enabled: true},
	}}
	assert.False(t, (&UIHandler{Config: enabledConfig}).masterKeyLoginEnabled())

	enabledConfig.GeneralSettings.SocialAuth.MasterKeyLoginEnabled = boolPointer(true)
	assert.True(t, (&UIHandler{Config: enabledConfig}).masterKeyLoginEnabled())

	provider := &stubSocialProvider{name: "github"}
	h := newSocialAuthTestHandler(t, provider)
	router := socialAuthTestRouter(h)

	getReq := httptest.NewRequest(http.MethodGet, "/ui/login?error=provider_failed", nil)
	getW := httptest.NewRecorder()
	router.ServeHTTP(getW, getReq)
	require.Equal(t, http.StatusOK, getW.Code)
	assert.Contains(t, getW.Body.String(), `href="/auth/github"`)
	assert.Contains(t, getW.Body.String(), "Provider authentication failed")
	assert.NotContains(t, getW.Body.String(), `name="api_key"`)
	assert.Equal(t, "no-store", getW.Header().Get("Cache-Control"))

	untrustedReq := httptest.NewRequest(http.MethodGet, "/ui/login?error=Click+this+fake+support+link", nil)
	untrustedW := httptest.NewRecorder()
	router.ServeHTTP(untrustedW, untrustedReq)
	assert.NotContains(t, untrustedW.Body.String(), "fake support link")

	postReq := httptest.NewRequest(http.MethodPost, "/ui/login", nil)
	postW := httptest.NewRecorder()
	router.ServeHTTP(postW, postReq)
	assert.Equal(t, http.StatusForbidden, postW.Code)
	assert.Contains(t, postW.Body.String(), "Master-key login is disabled")
}
