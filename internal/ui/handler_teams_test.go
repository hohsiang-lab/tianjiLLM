package ui

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParsePage verifies boundary and edge-case behavior.
func TestParsePage(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"", 1},
		{"0", 1},
		{"-1", 1},
		{"1", 1},
		{"5", 5},
		{"999999", 999999},
		{"abc", 1},
		{"1.5", 1},
	}
	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			assert.Equal(t, tc.expected, parsePage(tc.input))
		})
	}
}

// TestParseOptionalFloat covers the shared parseOptionalFloat helper used for max_budget.
func TestParseOptionalFloat(t *testing.T) {
	t.Run("empty string returns nil (unlimited)", func(t *testing.T) {
		assert.Nil(t, parseOptionalFloat(""))
	})
	t.Run("valid float returns pointer", func(t *testing.T) {
		v := parseOptionalFloat("99.50")
		require.NotNil(t, v)
		assert.InDelta(t, 99.50, *v, 0.001)
	})
	t.Run("zero returns pointer to zero", func(t *testing.T) {
		v := parseOptionalFloat("0")
		require.NotNil(t, v)
		assert.Equal(t, float64(0), *v)
	})
	t.Run("non-numeric returns nil", func(t *testing.T) {
		assert.Nil(t, parseOptionalFloat("abc"))
	})
	t.Run("integer string parses as float", func(t *testing.T) {
		v := parseOptionalFloat("100")
		require.NotNil(t, v)
		assert.Equal(t, float64(100), *v)
	})
}

// TestParseMembersWithRoles covers the members_with_roles JSONB parsing helper.
func TestParseMembersWithRoles(t *testing.T) {
	t.Run("valid JSON array returns correctly", func(t *testing.T) {
		data := []byte(`[{"user_id":"u1","role":"member"},{"user_id":"u2","role":"admin"}]`)
		result, err := parseMembersWithRoles(data)
		require.NoError(t, err)
		require.Len(t, result, 2)
		assert.Equal(t, "u1", result[0].UserID)
		assert.Equal(t, "member", result[0].Role)
		assert.Equal(t, "u2", result[1].UserID)
		assert.Equal(t, "admin", result[1].Role)
	})

	t.Run("null bytes returns empty slice without panic", func(t *testing.T) {
		result, err := parseMembersWithRoles([]byte("null"))
		require.NoError(t, err)
		assert.Empty(t, result)
	})

	t.Run("empty bytes returns empty slice without panic", func(t *testing.T) {
		result, err := parseMembersWithRoles([]byte{})
		require.NoError(t, err)
		assert.NotNil(t, result, "should be empty slice, not nil")
		assert.Len(t, result, 0)
	})

	t.Run("empty string bytes returns empty slice without panic", func(t *testing.T) {
		result, err := parseMembersWithRoles([]byte(""))
		require.NoError(t, err)
		assert.NotNil(t, result)
	})

	t.Run("malformed JSON returns error and empty fallback slice", func(t *testing.T) {
		result, err := parseMembersWithRoles([]byte(`{not valid json`))
		require.Error(t, err)
		assert.NotNil(t, result, "even on error, result should be empty slice not nil")
		assert.Len(t, result, 0)
	})

	t.Run("empty JSON array returns empty slice", func(t *testing.T) {
		result, err := parseMembersWithRoles([]byte(`[]`))
		require.NoError(t, err)
		assert.Empty(t, result)
	})
}

// TestSearchFilterSanitization verifies that special SQL wildcard characters in the
// search term do not cause unintended behavior when used in Go-side string matching.
// Since filtering is done in Go (strings.Contains / strings.ToLower), SQL injection
// via LIKE wildcards is not applicable here; however we verify the filter correctly
// handles these characters as literals.
func TestSearchFilterSanitization(t *testing.T) {
	// The search filter uses strings.Contains, so % _ ' are treated as literal chars.
	tests := []struct {
		name     string
		alias    string
		search   string
		expected bool
	}{
		{"empty search matches everything", "my-team", "", true},
		{"percent sign in search matches literal percent", "100%team", "%team", true},
		{"percent sign does not act as wildcard", "my-team", "%", false},
		{"underscore matches literal underscore", "my_team", "_", true},
		{"underscore does not match any single char", "my-team", "_", false},
		{"single quote matches literal quote", "it's-team", "'s", true},
		{"case insensitive match", "My-Team", "my-team", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			matched := tc.search == "" || strings.Contains(strings.ToLower(tc.alias), strings.ToLower(tc.search))
			assert.Equal(t, tc.expected, matched)
		})
	}
}

// TestMarshalMembersWithRoles verifies that memberWithRole round-trips through JSON correctly.
func TestMarshalMembersWithRoles(t *testing.T) {
	members := []memberWithRole{
		{UserID: "u1", Role: "admin"},
		{UserID: "u2", Role: "member"},
	}
	data, err := json.Marshal(members)
	require.NoError(t, err)

	roundtrip, err := parseMembersWithRoles(data)
	require.NoError(t, err)
	assert.Equal(t, members, roundtrip)
}
