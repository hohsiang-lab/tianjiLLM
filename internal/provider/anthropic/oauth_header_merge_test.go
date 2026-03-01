package anthropic

import (
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test 1: SetOAuthHeaders should MERGE with existing anthropic-beta, not overwrite.
func TestSetOAuthHeaders_PreservesExistingBetaHeader(t *testing.T) {
	req, err := http.NewRequest(http.MethodPost, "https://api.anthropic.com/v1/messages", nil)
	require.NoError(t, err)

	req.Header.Set("anthropic-beta", "prompt-caching-scope-2026-01-05")

	SetOAuthHeaders(req, "sk-ant-oat01-abc123")

	betaValues := req.Header.Values("anthropic-beta")
	combined := strings.Join(betaValues, ",")

	assert.Contains(t, combined, "prompt-caching-scope-2026-01-05",
		"original beta header value should be preserved")
	assert.Contains(t, combined, OAuthBetaHeader,
		"oauth beta header should be added")
}

// Test 3: Regression guard - no prior beta header, should just set oauth one. Should PASS.
func TestSetOAuthHeaders_NoPriorBetaHeader(t *testing.T) {
	req, err := http.NewRequest(http.MethodPost, "https://api.anthropic.com/v1/messages", nil)
	require.NoError(t, err)

	SetOAuthHeaders(req, "sk-ant-oat01-abc123")

	betaValues := req.Header.Values("anthropic-beta")
	assert.Len(t, betaValues, 1, "should have exactly one beta header value")
	assert.Equal(t, OAuthBetaHeader, betaValues[0])
}
