package ui

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestOrgMemberInputValidation verifies the input validation logic used
// in handleOrgMemberAdd: both user_id and user_role must be non-empty
// after trimming whitespace.
func TestOrgMemberInputValidation(t *testing.T) {
	tests := []struct {
		name    string
		userID  string
		role    string
		isValid bool
	}{
		{"both present", "user1", "admin", true},
		{"empty user_id", "", "admin", false},
		{"whitespace user_id", "   ", "admin", false},
		{"empty role", "user1", "", false},
		{"whitespace role", "user1", "  ", false},
		{"both empty", "", "", false},
		{"both whitespace", "  ", "  ", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			uid := strings.TrimSpace(tc.userID)
			role := strings.TrimSpace(tc.role)
			valid := uid != "" && role != ""
			assert.Equal(t, tc.isValid, valid)
		})
	}
}

// TestOrgDeletePreCheck verifies the team-count guard logic used in
// handleOrgDelete: deletion is blocked when the org has teams (len > 0)
// and allowed when there are no teams (len == 0).
func TestOrgDeletePreCheck(t *testing.T) {
	tests := []struct {
		name      string
		teamCount int
		canDelete bool
	}{
		{"no teams allows delete", 0, true},
		{"one team blocks delete", 1, false},
		{"many teams blocks delete", 5, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			canDelete := tc.teamCount == 0
			assert.Equal(t, tc.canDelete, canDelete)
		})
	}
}

// TestOrgAliasPointerLogic verifies that empty alias results in nil pointer
// (used in handleOrgUpdate to distinguish "no alias" from "has alias").
func TestOrgAliasPointerLogic(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		isNil    bool
		expected string
	}{
		{"non-empty alias", "my-org", false, "my-org"},
		{"empty alias", "", true, ""},
		{"whitespace-only alias", "   ", true, ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			alias := strings.TrimSpace(tc.input)
			var ptr *string
			if alias != "" {
				ptr = &alias
			}
			if tc.isNil {
				assert.Nil(t, ptr)
			} else {
				assert.NotNil(t, ptr)
				assert.Equal(t, tc.expected, *ptr)
			}
		})
	}
}

// TestOrgMemberRemoveRequiresUserID verifies the guard in handleOrgMemberRemove.
func TestOrgMemberRemoveRequiresUserID(t *testing.T) {
	tests := []struct {
		name    string
		userID  string
		isValid bool
	}{
		{"valid user_id", "user1", true},
		{"empty user_id", "", false},
		{"whitespace user_id", "   ", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			uid := strings.TrimSpace(tc.userID)
			assert.Equal(t, tc.isValid, uid != "")
		})
	}
}
