package ui

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/praxisllmlab/tianjiLLM/internal/auth/social"
	"github.com/praxisllmlab/tianjiLLM/internal/db"
)

var userIdentityTestUserColumns = []string{
	"user_id", "user_alias", "user_email", "user_role", "teams", "max_budget",
	"spend", "models", "metadata", "tpm_limit", "rpm_limit", "budget_duration",
	"budget_reset_at", "budget_id", "created_at", "created_by", "updated_at",
	"updated_by", "auth_version",
}

var userIdentityTestIdentityColumns = []string{
	"identity_id", "user_id", "provider", "provider_user_id", "provider_email",
	"email_verified", "display_name", "avatar_url", "last_login_at", "created_at",
	"updated_at",
}

func userIdentityTestUserRows(
	userID string,
	role string,
	metadata string,
	authVersion int64,
) *pgxmock.Rows {
	now := time.Now()
	alias := "Test User"
	email := "user@example.com"
	return pgxmock.NewRows(userIdentityTestUserColumns).AddRow(
		userID, &alias, &email, role, []string{}, nil, float64(0), []string{},
		[]byte(metadata), nil, nil, nil, nil, nil, now, "admin", now, "admin",
		authVersion,
	)
}

func userIdentityTestIdentityRows(identityID, userID, provider string) *pgxmock.Rows {
	now := time.Now()
	email := "user@example.com"
	displayName := "Test User"
	return pgxmock.NewRows(userIdentityTestIdentityColumns).AddRow(
		identityID, userID, provider, provider+"-subject", &email, true, &displayName,
		nil, now, now, now,
	)
}

func expectUserDetailReload(
	mock pgxmock.PgxPoolIface,
	userID string,
	role string,
	metadata string,
	authVersion int64,
) {
	mock.ExpectQuery(`SELECT .+ FROM "UserTable" WHERE user_id = \$1`).
		WithArgs(userID).
		WillReturnRows(userIdentityTestUserRows(userID, role, metadata, authVersion))
	mock.ExpectQuery(`(?s)SELECT .*FROM "UserIdentityTable".*WHERE user_id = \$1.*ORDER BY created_at`).
		WithArgs(userID).
		WillReturnRows(pgxmock.NewRows(userIdentityTestIdentityColumns))
}

func requestWithPrincipal(
	req *http.Request,
	userID string,
	identityID string,
	principal *Principal,
) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("user_id", userID)
	if identityID != "" {
		rctx.URLParams.Add("identity_id", identityID)
	}
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	if principal != nil {
		req = req.WithContext(context.WithValue(req.Context(), principalContextKey{}, principal))
	}
	return req
}

func TestHandleUserSocialAuthEnrollmentEnablesNextProvider(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	t.Cleanup(func() { mock.Close() })

	h := newSocialAuthTestHandler(t, &stubSocialProvider{name: "github"})
	h.DB = db.New(mock)

	mock.ExpectQuery(`SELECT .+ FROM "UserTable" WHERE user_id = \$1`).
		WithArgs("user-1").
		WillReturnRows(userIdentityTestUserRows(
			"user-1",
			"internal_user",
			`{"status":"active"}`,
			3,
		))
	mock.ExpectQuery(`(?s)UPDATE "UserTable".*social_auth_enrollment_enabled.*RETURNING`).
		WithArgs(true, "admin", "user-1").
		WillReturnRows(userIdentityTestUserRows(
			"user-1",
			"internal_user",
			`{"status":"active","social_auth_enrollment_enabled":true}`,
			3,
		))
	expectUserDetailReload(
		mock,
		"user-1",
		"internal_user",
		`{"status":"active","social_auth_enrollment_enabled":true}`,
		3,
	)

	req := httptest.NewRequest(
		http.MethodPost,
		"/ui/users/user-1/social-auth/enrollment",
		strings.NewReader("enabled=true"),
	)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = requestWithPrincipal(req, "user-1", "", &Principal{
		UserID: "admin-1",
		Role:   "proxy_admin",
	})
	w := httptest.NewRecorder()

	h.handleUserSocialAuthEnrollment(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Next verified provider enrollment enabled")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestHandleUserIdentityDeleteRejectsSelfLastIdentity(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	t.Cleanup(func() { mock.Close() })

	mock.ExpectBegin()
	mock.ExpectExec(`SELECT pg_advisory_xact_lock\(hashtextextended\(\$1, 0\)\)`).
		WithArgs(activeAdminMutationLockName).
		WillReturnResult(pgxmock.NewResult("SELECT", 1))
	mock.ExpectQuery(`SELECT .+ FROM "UserTable" WHERE user_id = \$1`).
		WithArgs("admin-1").
		WillReturnRows(userIdentityTestUserRows(
			"admin-1",
			"proxy_admin",
			`{"status":"active"}`,
			5,
		))
	mock.ExpectQuery(`(?s)SELECT .*FROM "UserIdentityTable".*WHERE identity_id = \$1`).
		WithArgs("identity-1").
		WillReturnRows(userIdentityTestIdentityRows("identity-1", "admin-1", "github"))
	mock.ExpectQuery(`SELECT COUNT\(\*\).*FROM "UserIdentityTable"`).
		WithArgs("admin-1").
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(int64(1)))
	mock.ExpectRollback()
	expectUserDetailReload(
		mock,
		"admin-1",
		"proxy_admin",
		`{"status":"active"}`,
		5,
	)

	h := newTestHandler(t)
	h.DB = db.New(mock)
	h.userMutationTxBeginner = mock
	req := requestWithPrincipal(
		httptest.NewRequest(
			http.MethodPost,
			"/ui/users/admin-1/social-auth/identities/identity-1/delete",
			nil,
		),
		"admin-1",
		"identity-1",
		&Principal{UserID: "admin-1", Role: "proxy_admin"},
	)
	w := httptest.NewRecorder()

	h.handleUserIdentityDelete(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "You cannot unlink your last sign-in identity")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestHandleUserIdentityDeleteRejectsLastActiveAdminIdentity(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	t.Cleanup(func() { mock.Close() })

	mock.ExpectBegin()
	mock.ExpectExec(`SELECT pg_advisory_xact_lock\(hashtextextended\(\$1, 0\)\)`).
		WithArgs(activeAdminMutationLockName).
		WillReturnResult(pgxmock.NewResult("SELECT", 1))
	mock.ExpectQuery(`SELECT .+ FROM "UserTable" WHERE user_id = \$1`).
		WithArgs("admin-1").
		WillReturnRows(userIdentityTestUserRows(
			"admin-1",
			"proxy_admin",
			`{"status":"active"}`,
			5,
		))
	mock.ExpectQuery(`(?s)SELECT .*FROM "UserIdentityTable".*WHERE identity_id = \$1`).
		WithArgs("identity-1").
		WillReturnRows(userIdentityTestIdentityRows("identity-1", "admin-1", "github"))
	mock.ExpectQuery(`SELECT COUNT\(\*\).*FROM "UserIdentityTable"`).
		WithArgs("admin-1").
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(int64(1)))
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM "UserTable"`).
		WithArgs("proxy_admin").
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(int64(1)))
	mock.ExpectRollback()
	expectUserDetailReload(
		mock,
		"admin-1",
		"proxy_admin",
		`{"status":"active"}`,
		5,
	)

	h := newTestHandler(t)
	h.DB = db.New(mock)
	h.userMutationTxBeginner = mock
	req := requestWithPrincipal(
		httptest.NewRequest(
			http.MethodPost,
			"/ui/users/admin-1/social-auth/identities/identity-1/delete",
			nil,
		),
		"admin-1",
		"identity-1",
		&Principal{UserID: "admin-2", Role: "proxy_admin"},
	)
	w := httptest.NewRecorder()

	h.handleUserIdentityDelete(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Cannot unlink the last active admin identity")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestHandleUserIdentityDeleteBumpsAuthVersion(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	t.Cleanup(func() { mock.Close() })

	mock.ExpectBegin()
	mock.ExpectExec(`SELECT pg_advisory_xact_lock\(hashtextextended\(\$1, 0\)\)`).
		WithArgs(activeAdminMutationLockName).
		WillReturnResult(pgxmock.NewResult("SELECT", 1))
	mock.ExpectQuery(`SELECT .+ FROM "UserTable" WHERE user_id = \$1`).
		WithArgs("user-1").
		WillReturnRows(userIdentityTestUserRows(
			"user-1",
			"internal_user",
			`{"status":"active"}`,
			5,
		))
	mock.ExpectQuery(`(?s)SELECT .*FROM "UserIdentityTable".*WHERE identity_id = \$1`).
		WithArgs("identity-1").
		WillReturnRows(userIdentityTestIdentityRows("identity-1", "user-1", "github"))
	mock.ExpectQuery(`SELECT COUNT\(\*\).*FROM "UserIdentityTable"`).
		WithArgs("user-1").
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(int64(1)))
	mock.ExpectExec(`(?s)DELETE FROM "UserIdentityTable".*identity_id = \$1 AND user_id = \$2`).
		WithArgs("identity-1", "user-1").
		WillReturnResult(pgxmock.NewResult("DELETE", 1))
	mock.ExpectExec(`(?s)UPDATE "UserTable".*auth_version = auth_version \+ 1`).
		WithArgs("admin", "user-1").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	mock.ExpectCommit()
	expectUserDetailReload(
		mock,
		"user-1",
		"internal_user",
		`{"status":"active"}`,
		6,
	)

	h := newTestHandler(t)
	h.DB = db.New(mock)
	h.userMutationTxBeginner = mock
	h.SocialAuth = social.NewService(&stubSocialProvider{name: "github"})
	req := requestWithPrincipal(
		httptest.NewRequest(
			http.MethodPost,
			"/ui/users/user-1/social-auth/identities/identity-1/delete",
			nil,
		),
		"user-1",
		"identity-1",
		&Principal{UserID: "admin-1", Role: "proxy_admin"},
	)
	w := httptest.NewRecorder()

	h.handleUserIdentityDelete(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Provider identity unlinked")
	require.NoError(t, mock.ExpectationsWereMet())
}
