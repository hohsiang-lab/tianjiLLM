package integration

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/praxisllmlab/tianjiLLM/internal/ui"
)

func TestPostgresUISessionDestroyPreventsTokenReplay(t *testing.T) {
	queries := setupTestDB(t)
	pool := getPool(t, queries)
	manager, cleanup := ui.NewSessionManager(pool, true)
	t.Cleanup(cleanup)

	tests := []struct {
		name        string
		role        string
		userID      string
		authVersion int64
	}{
		{
			name: "break glass",
			role: "proxy_admin",
		},
		{
			name:        "social identity",
			role:        "internal_user",
			userID:      "social-user-1",
			authVersion: 7,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			_, err := pool.Exec(ctx, `DELETE FROM ui_sessions`)
			require.NoError(t, err)

			login := manager.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				require.NoError(t, manager.RenewToken(r.Context()))
				manager.Put(r.Context(), "authenticated", true)
				manager.Put(r.Context(), "role", tc.role)
				manager.Put(r.Context(), "user_id", tc.userID)
				manager.Put(r.Context(), "auth_version", tc.authVersion)
				w.WriteHeader(http.StatusNoContent)
			}))
			loginW := httptest.NewRecorder()
			login.ServeHTTP(loginW, httptest.NewRequest(http.MethodPost, "/ui/login", nil))
			require.Equal(t, http.StatusNoContent, loginW.Code)

			sessionCookie := responseSessionCookie(t, loginW)
			var storedSessions int
			err = pool.QueryRow(
				ctx,
				`SELECT COUNT(*) FROM ui_sessions WHERE token = $1`,
				sessionCookie.Value,
			).Scan(&storedSessions)
			require.NoError(t, err)
			require.Equal(t, 1, storedSessions)

			logout := manager.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				require.NoError(t, manager.Destroy(r.Context()))
				http.Redirect(w, r, "/ui/login", http.StatusSeeOther)
			}))
			logoutReq := httptest.NewRequest(http.MethodPost, "/ui/logout", nil)
			logoutReq.AddCookie(sessionCookie)
			logoutW := httptest.NewRecorder()
			logout.ServeHTTP(logoutW, logoutReq)
			require.Equal(t, http.StatusSeeOther, logoutW.Code)
			assert.Equal(t, "/ui/login", logoutW.Header().Get("Location"))

			clearedCookie := responseSessionCookie(t, logoutW)
			assert.Empty(t, clearedCookie.Value)
			assert.Less(t, clearedCookie.MaxAge, 0)
			assert.Equal(t, "/", clearedCookie.Path)
			assert.True(t, clearedCookie.HttpOnly)
			assert.True(t, clearedCookie.Secure)
			assert.Equal(t, http.SameSiteLaxMode, clearedCookie.SameSite)

			err = pool.QueryRow(
				ctx,
				`SELECT COUNT(*) FROM ui_sessions WHERE token = $1`,
				sessionCookie.Value,
			).Scan(&storedSessions)
			require.NoError(t, err)
			assert.Zero(t, storedSessions)

			var authenticated bool
			replay := manager.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				authenticated = manager.GetBool(r.Context(), "authenticated")
				w.WriteHeader(http.StatusNoContent)
			}))
			replayReq := httptest.NewRequest(http.MethodGet, "/ui/", nil)
			replayReq.AddCookie(sessionCookie)
			replayW := httptest.NewRecorder()
			replay.ServeHTTP(replayW, replayReq)
			require.Equal(t, http.StatusNoContent, replayW.Code)
			assert.False(t, authenticated)
		})
	}
}

func responseSessionCookie(t *testing.T, w *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	for _, cookie := range w.Result().Cookies() {
		if cookie.Name == "tianji_session" {
			return cookie
		}
	}
	t.Fatal("tianji_session cookie not found")
	return nil
}
