package db

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUserMutationsIncrementAuthVersionAtomically(t *testing.T) {
	for name, query := range map[string]string{
		"update user":     updateUser,
		"set user status": setUserStatus,
		"soft delete":     softDeleteUser,
	} {
		t.Run(name, func(t *testing.T) {
			assert.Contains(t, query, "auth_version = auth_version + 1")
		})
	}
}

func TestSecuritySensitiveUserQueriesOnlySelectActiveUsers(t *testing.T) {
	assert.Contains(
		t,
		strings.Join(strings.Fields(listUsersByVerifiedEmailCandidate), " "),
		"COALESCE(metadata->>'status', 'active') = 'active'",
	)
	assert.Contains(
		t,
		strings.Join(strings.Fields(countUsersByRole), " "),
		"COALESCE(metadata->>'status', 'active') = 'active'",
	)
	assert.Contains(
		t,
		strings.Join(strings.Fields(getUserByEmail), " "),
		"LOWER(user_email) = LOWER($1)",
	)
	assert.Contains(
		t,
		strings.Join(strings.Fields(getUserByEmail), " "),
		"COALESCE(metadata->>'status', 'active') != 'deleted'",
	)
}
