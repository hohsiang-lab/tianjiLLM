package auth

import (
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var keyPattern = regexp.MustCompile(`^sk-ant-oat01-tianji-[0-9a-f]{60}$`)

func TestGenerateVirtualKey_Format(t *testing.T) {
	key := GenerateVirtualKey()

	assert.True(t, strings.HasPrefix(key, VirtualKeyPrefix),
		"key must start with %s, got: %s", VirtualKeyPrefix, key)
	assert.Equal(t, 80, len(key),
		"key must be exactly 80 chars (prefix 20 + hex 60), got %d", len(key))
	assert.Regexp(t, keyPattern, key,
		"key must match expected pattern, got: %s", key)
}

func TestGenerateVirtualKey_PassesOAuthCheck(t *testing.T) {
	key := GenerateVirtualKey()
	// Virtual keys must start with "sk-ant-oat" to pass OAuth detection.
	assert.True(t, strings.HasPrefix(key, "sk-ant-oat"),
		"key must pass OAuth prefix check, got: %s", key)
}

func TestGenerateVirtualKey_Uniqueness(t *testing.T) {
	seen := make(map[string]struct{}, 1000)
	for i := range 1000 {
		key := GenerateVirtualKey()
		require.NotContains(t, seen, key, "duplicate key at iteration %d: %s", i, key)
		seen[key] = struct{}{}
	}
}

func TestHashKey(t *testing.T) {
	hash := HashKey("test-key")
	assert.Len(t, hash, 64, "SHA256 hex digest must be 64 chars")

	// Deterministic: same input produces same output.
	assert.Equal(t, hash, HashKey("test-key"))

	// Different input produces different output.
	assert.NotEqual(t, hash, HashKey("other-key"))
}
