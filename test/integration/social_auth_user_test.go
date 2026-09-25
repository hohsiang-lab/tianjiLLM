package integration

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/praxisllmlab/tianjiLLM/internal/db"
)

func TestSocialAuthUserMutationsInvalidateSessionsAtomically(t *testing.T) {
	q := setupTestDB(t)
	pool := getPool(t, q)
	ctx := context.Background()
	prefix := "test-social-auth-user-" + uuid.NewString()
	_, _ = pool.Exec(ctx, `DELETE FROM "UserTable" WHERE user_id LIKE $1`, prefix+"%")
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM "UserTable" WHERE user_id LIKE $1`, prefix+"%")
	})

	role := prefix + "-role"
	email := prefix + "@example.com"
	aliasOne := "Social User One"
	aliasTwo := "Social User Two"
	userOneID := prefix + "-1"
	userTwoID := prefix + "-2"

	userOne, err := q.CreateUser(ctx, db.CreateUserParams{
		UserID:    userOneID,
		UserAlias: &aliasOne,
		UserEmail: &email,
		UserRole:  role,
		Teams:     []string{},
		Models:    []string{},
		CreatedBy: "test",
	})
	require.NoError(t, err)
	assert.Equal(t, int64(1), userOne.AuthVersion)

	_, err = q.CreateUser(ctx, db.CreateUserParams{
		UserID:    userTwoID,
		UserAlias: &aliasTwo,
		UserEmail: &email,
		UserRole:  role,
		Teams:     []string{},
		Models:    []string{},
		CreatedBy: "test",
	})
	require.NoError(t, err)

	updated, err := q.UpdateUser(ctx, db.UpdateUserParams{
		UserID:    userOneID,
		UserAlias: &aliasOne,
		UserEmail: &email,
		UserRole:  role,
		Models:    []string{},
		UpdatedBy: "test",
	})
	require.NoError(t, err)
	assert.Equal(t, int64(2), updated.AuthVersion)

	enrollmentUser, err := q.SetUserSocialAuthEnrollment(ctx, db.SetUserSocialAuthEnrollmentParams{
		Enabled:   true,
		UpdatedBy: "test",
		UserID:    userOneID,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(2), enrollmentUser.AuthVersion,
		"opening enrollment must not invalidate existing sessions")
	assert.JSONEq(t, `{"social_auth_enrollment_enabled":true}`, string(enrollmentUser.Metadata))

	require.NoError(t, q.SetUserStatus(ctx, db.SetUserStatusParams{
		UserID:    userOneID,
		Status:    "disabled",
		UpdatedBy: "test",
	}))
	userOne, err = q.GetUser(ctx, userOneID)
	require.NoError(t, err)
	assert.Equal(t, int64(3), userOne.AuthVersion)

	activeAdmins, err := q.CountUsersByRole(ctx, role)
	require.NoError(t, err)
	assert.Equal(t, int64(1), activeAdmins, "disabled users must not count as active administrators")

	candidates, err := q.ListUsersByVerifiedEmailCandidate(ctx, email)
	require.NoError(t, err)
	require.Len(t, candidates, 1)
	assert.Equal(t, userTwoID, candidates[0].UserID)

	require.NoError(t, q.SoftDeleteUser(ctx, db.SoftDeleteUserParams{
		UserID:    userTwoID,
		UpdatedBy: "test",
	}))
	userTwo, err := q.GetUser(ctx, userTwoID)
	require.NoError(t, err)
	assert.Equal(t, int64(2), userTwo.AuthVersion)

	activeAdmins, err = q.CountUsersByRole(ctx, role)
	require.NoError(t, err)
	assert.Zero(t, activeAdmins)

	candidates, err = q.ListUsersByVerifiedEmailCandidate(ctx, email)
	require.NoError(t, err)
	assert.Empty(t, candidates)
}
