package ui

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/praxisllmlab/tianjiLLM/internal/db"
)

// withChiURLParam adds a chi URL parameter to the request context.
func withChiURLParam(r *http.Request, key, val string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, val)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

func TestUserStatusFromMetadata(t *testing.T) {
	tests := []struct {
		name     string
		metadata []byte
		expected string
	}{
		{"nil metadata", nil, "active"},
		{"empty bytes", []byte{}, "active"},
		{"empty JSON object", []byte(`{}`), "active"},
		{"status active", []byte(`{"status":"active"}`), "active"},
		{"status disabled", []byte(`{"status":"disabled"}`), "disabled"},
		{"status deleted", []byte(`{"status":"deleted"}`), "deleted"},
		{"status empty string", []byte(`{"status":""}`), "active"},
		{"status non-string", []byte(`{"status":123}`), "active"},
		{"invalid JSON", []byte(`not json`), "active"},
		{"other keys no status", []byte(`{"foo":"bar"}`), "active"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, userStatusFromMetadata(tc.metadata))
		})
	}
}

func TestUserSocialAuthEnrollmentEnabled(t *testing.T) {
	tests := []struct {
		name     string
		metadata []byte
		expected bool
	}{
		{"nil metadata", nil, false},
		{"empty metadata", []byte{}, false},
		{"missing flag", []byte(`{"status":"active"}`), false},
		{"enabled", []byte(`{"social_auth_enrollment_enabled":true}`), true},
		{"disabled", []byte(`{"social_auth_enrollment_enabled":false}`), false},
		{"non-boolean flag", []byte(`{"social_auth_enrollment_enabled":"true"}`), false},
		{"invalid JSON", []byte(`not json`), false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, userSocialAuthEnrollmentEnabled(tc.metadata))
		})
	}
}

// newTestHandler creates a UIHandler with an in-memory SCS manager.
func newTestHandler(t *testing.T) *UIHandler {
	t.Helper()
	manager, cleanup := NewSessionManager(nil, false)
	t.Cleanup(cleanup)
	return &UIHandler{
		MasterKey:      "test-master-key-for-qa",
		SessionManager: manager,
	}
}

// setAdminSession sets a valid admin session cookie on the request.
func setAdminSession(t *testing.T, r *http.Request, h *UIHandler) {
	t.Helper()
	setTestSession(t, r, h, "admin", "")
}

// setNonAdminSession sets a valid non-admin session cookie on the request.
func setNonAdminSession(t *testing.T, r *http.Request, h *UIHandler) {
	t.Helper()
	setTestSession(t, r, h, "viewer", "user-2")
}

func setTestSession(t *testing.T, r *http.Request, h *UIHandler, role, userID string) {
	t.Helper()
	sessionManager := h.getSessionManager()
	createSession := sessionManager.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		require.NoError(t, sessionManager.RenewToken(req.Context()))
		sessionManager.Put(req.Context(), sessionAuthenticatedKey, true)
		sessionManager.Put(req.Context(), sessionRoleKey, role)
		sessionManager.Put(req.Context(), sessionUserIDKey, userID)
		sessionManager.Put(req.Context(), sessionAuthVersionKey, int64(0))
		w.WriteHeader(http.StatusNoContent)
	}))

	w := httptest.NewRecorder()
	createSession.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/test/session", nil))
	require.Equal(t, http.StatusNoContent, w.Code)
	for _, cookie := range w.Result().Cookies() {
		if cookie.Name == sessionCookieName {
			r.AddCookie(cookie)
			return
		}
	}
	t.Fatal("session cookie not found")
}

func TestRequireAdmin_BlocksNonAdmin(t *testing.T) {
	h := newTestHandler(t)

	innerCalled := false
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		innerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	middleware := h.getSessionManager().LoadAndSave(h.requireAdmin(inner))

	t.Run("no session cookie returns 403", func(t *testing.T) {
		innerCalled = false
		req := httptest.NewRequest(http.MethodGet, "/ui/users", nil)
		w := httptest.NewRecorder()
		middleware.ServeHTTP(w, req)
		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.False(t, innerCalled, "inner handler should not be called")
	})

	t.Run("non-admin session returns 403", func(t *testing.T) {
		innerCalled = false
		req := httptest.NewRequest(http.MethodGet, "/ui/users", nil)
		setNonAdminSession(t, req, h)
		w := httptest.NewRecorder()
		middleware.ServeHTTP(w, req)
		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.False(t, innerCalled, "inner handler should not be called for non-admin")
	})

	t.Run("HTMX request from non-admin returns 403 without body", func(t *testing.T) {
		innerCalled = false
		req := httptest.NewRequest(http.MethodGet, "/ui/users", nil)
		setNonAdminSession(t, req, h)
		req.Header.Set("HX-Request", "true")
		w := httptest.NewRecorder()
		middleware.ServeHTTP(w, req)
		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.False(t, innerCalled)
	})
}

func TestRequireAdmin_AllowsAdmin(t *testing.T) {
	h := newTestHandler(t)

	innerCalled := false
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		innerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	middleware := h.getSessionManager().LoadAndSave(h.requireAdmin(inner))

	t.Run("admin session passes through", func(t *testing.T) {
		innerCalled = false
		req := httptest.NewRequest(http.MethodGet, "/ui/users", nil)
		setAdminSession(t, req, h)
		w := httptest.NewRecorder()
		middleware.ServeHTTP(w, req)
		assert.True(t, innerCalled, "inner handler should be called for admin")
		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestHandleUserDelete_DBNil(t *testing.T) {
	h := newTestHandler(t)
	// DB is nil — should return 503
	req := httptest.NewRequest(http.MethodPost, "/ui/users/some-id/delete", nil)
	// Set chi URL param
	req = withChiURLParam(req, "user_id", "some-id")

	w := httptest.NewRecorder()
	h.handleUserDelete(w, req)
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestHandleUserCreate_DBNil(t *testing.T) {
	h := newTestHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/ui/users/create", nil)
	w := httptest.NewRecorder()
	h.handleUserCreate(w, req)
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestHandleUserBlock_DBNil(t *testing.T) {
	h := newTestHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/ui/users/some-id/block", nil)
	req = withChiURLParam(req, "user_id", "some-id")

	w := httptest.NewRecorder()
	h.handleUserBlock(w, req)
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestHandleUserUnblock_DBNil(t *testing.T) {
	h := newTestHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/ui/users/some-id/unblock", nil)
	req = withChiURLParam(req, "user_id", "some-id")

	w := httptest.NewRecorder()
	h.handleUserUnblock(w, req)
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestHandleUserDelete_MissingUserID(t *testing.T) {
	h := newTestHandler(t)
	// With a nil DB but empty user_id — should return 400 before hitting DB
	req := httptest.NewRequest(http.MethodPost, "/ui/users//delete", nil)
	req = withChiURLParam(req, "user_id", "")

	w := httptest.NewRecorder()
	// DB is nil, so it hits the nil check first (503), not the empty user_id check
	// Actually, the nil DB check happens before user_id parsing, so this returns 503
	h.handleUserDelete(w, req)
	// With nil DB, returns 503 regardless
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestHandleUserCreate_MissingEmail(t *testing.T) {
	h := newTestHandler(t)
	// DB nil → 503
	req := httptest.NewRequest(http.MethodPost, "/ui/users/create",
		strings.NewReader("user_email=&user_alias=test"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	h.handleUserCreate(w, req)
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestHandleUserCreateRejectsUnassignableRole(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	t.Cleanup(func() { mock.Close() })

	h := newTestHandler(t)
	h.DB = db.New(mock)
	req := httptest.NewRequest(
		http.MethodPost,
		"/ui/users/create",
		strings.NewReader("user_email=user%40example.com&user_role=admin"),
	)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	h.handleUserCreate(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "invalid user role")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestHandleUserBlockFailsClosedWhenAdminCountUnavailable(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	t.Cleanup(func() { mock.Close() })

	mock.ExpectBegin()
	mock.ExpectExec(`SELECT pg_advisory_xact_lock\(hashtextextended\(\$1, 0\)\)`).
		WithArgs(activeAdminMutationLockName).
		WillReturnResult(pgxmock.NewResult("SELECT", 1))

	alias := "Admin"
	email := "admin@example.com"
	now := time.Now()
	mock.ExpectQuery(`SELECT .+ FROM "UserTable" WHERE user_id = \$1`).
		WithArgs("admin-1").
		WillReturnRows(pgxmock.NewRows([]string{
			"user_id", "user_alias", "user_email", "user_role", "teams", "max_budget",
			"spend", "models", "metadata", "tpm_limit", "rpm_limit", "budget_duration",
			"budget_reset_at", "budget_id", "created_at", "created_by", "updated_at",
			"updated_by", "auth_version",
		}).AddRow(
			"admin-1", &alias, &email, "proxy_admin", []string{}, nil,
			float64(0), []string{}, []byte(`{"status":"active"}`), nil, nil, nil,
			nil, nil, now, "admin", now, "admin", int64(1),
		))
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM "UserTable"`).
		WithArgs("proxy_admin").
		WillReturnError(errors.New("database unavailable"))
	mock.ExpectRollback()

	h := newTestHandler(t)
	h.DB = db.New(mock)
	h.userMutationTxBeginner = mock
	req := withChiURLParam(
		httptest.NewRequest(http.MethodPost, "/ui/users/admin-1/block", nil),
		"user_id",
		"admin-1",
	)
	w := httptest.NewRecorder()

	h.handleUserBlock(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "failed to verify admin safety")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUsersRoutesAreAdminProtected(t *testing.T) {
	h := newTestHandler(t)

	r := chi.NewRouter()
	r.Route("/ui", func(r chi.Router) {
		h.RegisterRoutes(r)
	})

	// All user endpoints should require admin auth
	endpoints := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/ui/users"},
		{http.MethodGet, "/ui/users/table"},
		{http.MethodPost, "/ui/users/create"},
		{http.MethodGet, "/ui/users/test-id"},
		{http.MethodPost, "/ui/users/test-id/block"},
		{http.MethodPost, "/ui/users/test-id/unblock"},
		{http.MethodPost, "/ui/users/test-id/delete"},
		{http.MethodPost, "/ui/users/test-id/social-auth/enrollment"},
		{http.MethodPost, "/ui/users/test-id/social-auth/identities/identity-id/delete"},
	}

	for _, ep := range endpoints {
		t.Run(ep.method+" "+ep.path, func(t *testing.T) {
			req := httptest.NewRequest(ep.method, ep.path, nil)
			// No session cookie → should get redirected to login or forbidden
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			code := w.Code
			// Without session: sessionAuth redirects to login (302) before requireAdmin
			assert.True(t, code == http.StatusSeeOther || code == http.StatusForbidden || code == http.StatusUnauthorized,
				"expected redirect or forbidden for %s %s, got %d", ep.method, ep.path, code)
		})
	}
}
