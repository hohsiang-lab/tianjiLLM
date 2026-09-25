package ui

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSessionManagerDefaults(t *testing.T) {
	manager, cleanup := NewSessionManager(nil, true)
	t.Cleanup(cleanup)

	assert.Equal(t, sessionLifetime, manager.Lifetime)
	assert.Equal(t, sessionIdleTime, manager.IdleTimeout)
	assert.Equal(t, sessionCookieName, manager.Cookie.Name)
	assert.Equal(t, "/", manager.Cookie.Path)
	assert.True(t, manager.Cookie.HttpOnly)
	assert.True(t, manager.Cookie.Persist)
	assert.True(t, manager.Cookie.Secure)
	assert.Equal(t, http.SameSiteLaxMode, manager.Cookie.SameSite)
}

func TestSessionManagerRoundTripAndDestroy(t *testing.T) {
	manager, cleanup := NewSessionManager(nil, true)
	t.Cleanup(cleanup)

	login := manager.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, manager.RenewToken(r.Context()))
		manager.Put(r.Context(), "user_id", "user-1")
		w.WriteHeader(http.StatusNoContent)
	}))

	loginReq := httptest.NewRequest(http.MethodPost, "/ui/login", nil)
	loginW := httptest.NewRecorder()
	login.ServeHTTP(loginW, loginReq)

	require.Equal(t, http.StatusNoContent, loginW.Code)
	var sessionCookie *http.Cookie
	for _, cookie := range loginW.Result().Cookies() {
		if cookie.Name == sessionCookieName {
			sessionCookie = cookie
			break
		}
	}
	require.NotNil(t, sessionCookie)
	assert.True(t, sessionCookie.HttpOnly)
	assert.Equal(t, http.SameSiteLaxMode, sessionCookie.SameSite)

	read := manager.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "user-1", manager.GetString(r.Context(), "user_id"))
		w.WriteHeader(http.StatusNoContent)
	}))
	readReq := httptest.NewRequest(http.MethodGet, "/ui/", nil)
	readReq.AddCookie(sessionCookie)
	readW := httptest.NewRecorder()
	read.ServeHTTP(readW, readReq)
	require.Equal(t, http.StatusNoContent, readW.Code)

	logout := manager.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, manager.Destroy(r.Context()))
		w.WriteHeader(http.StatusNoContent)
	}))
	logoutReq := httptest.NewRequest(http.MethodPost, "/ui/logout", nil)
	logoutReq.AddCookie(sessionCookie)
	logoutW := httptest.NewRecorder()
	logout.ServeHTTP(logoutW, logoutReq)
	require.Equal(t, http.StatusNoContent, logoutW.Code)

	var cleared bool
	for _, cookie := range logoutW.Result().Cookies() {
		if cookie.Name == sessionCookieName {
			cleared = cookie.MaxAge < 0
			assert.Empty(t, cookie.Value)
			assert.Equal(t, "/", cookie.Path)
			assert.True(t, cookie.HttpOnly)
			assert.True(t, cookie.Secure)
			assert.Equal(t, http.SameSiteLaxMode, cookie.SameSite)
		}
	}
	assert.True(t, cleared)

	replayReq := httptest.NewRequest(http.MethodGet, "/ui/", nil)
	replayReq.AddCookie(sessionCookie)
	replayW := httptest.NewRecorder()
	replay := manager.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Empty(t, manager.GetString(r.Context(), "user_id"))
		w.WriteHeader(http.StatusNoContent)
	}))
	replay.ServeHTTP(replayW, replayReq)
	require.Equal(t, http.StatusNoContent, replayW.Code)
}
