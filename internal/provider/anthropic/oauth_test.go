package anthropic

import (
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsOAuthToken(t *testing.T) {
	tests := []struct {
		name   string
		apiKey string
		want   bool
	}{
		{"oauth token", "sk-ant-oat01-abc123", true},
		{"oauth prefix exact", "sk-ant-oat", true},
		{"tianji virtual key", "sk-ant-oat01-tianji-" + strings.Repeat("ab", 30), true},
		{"regular key", "sk-ant-api03-abc123", false},
		{"empty string", "", false},
		{"almost matching", "sk-ant-oa", false},
		{"different prefix", "sk-ant-xyz", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, IsOAuthToken(tt.apiKey))
		})
	}
}

func TestSetOAuthHeaders(t *testing.T) {
	req, err := http.NewRequest(http.MethodPost, "https://api.anthropic.com/v1/messages", nil)
	require.NoError(t, err)

	// Pre-set x-api-key to verify deletion
	req.Header.Set("x-api-key", "sk-ant-oat01-abc123")
	// Pre-set anthropic-beta to verify merge (not overwrite)
	req.Header.Set("anthropic-beta", "claude-code-20250219")

	SetOAuthHeaders(req, "sk-ant-oat01-abc123")

	assert.Equal(t, "Bearer sk-ant-oat01-abc123", req.Header.Get("Authorization"))
	assert.Contains(t, req.Header.Get("anthropic-beta"), OAuthBetaHeader)
	assert.Contains(t, req.Header.Get("anthropic-beta"), "claude-code-20250219",
		"SetOAuthHeaders must preserve existing beta headers, not overwrite them")
	assert.Equal(t, "true", req.Header.Get("anthropic-dangerous-direct-browser-access"))
	assert.Empty(t, req.Header.Get("x-api-key"))
}

func TestSetupHeaders_OAuth(t *testing.T) {
	p := New()
	req, err := http.NewRequest(http.MethodPost, "https://api.anthropic.com/v1/messages", nil)
	require.NoError(t, err)

	p.SetupHeaders(req, "sk-ant-oat01-mytoken")

	assert.Equal(t, "application/json", req.Header.Get("Content-Type"))
	assert.Equal(t, "application/json", req.Header.Get("Accept"))
	assert.Equal(t, apiVersion, req.Header.Get("anthropic-version"))
	assert.Equal(t, "Bearer sk-ant-oat01-mytoken", req.Header.Get("Authorization"))
	assert.Equal(t, OAuthBetaHeader, req.Header.Get("anthropic-beta"))
	assert.Equal(t, "true", req.Header.Get("anthropic-dangerous-direct-browser-access"))
	assert.Empty(t, req.Header.Get("x-api-key"))
}

func TestSetupHeaders_RegularKey(t *testing.T) {
	p := New()
	req, err := http.NewRequest(http.MethodPost, "https://api.anthropic.com/v1/messages", nil)
	require.NoError(t, err)

	p.SetupHeaders(req, "sk-ant-api03-abc123")

	assert.Equal(t, "application/json", req.Header.Get("Content-Type"))
	assert.Equal(t, "application/json", req.Header.Get("Accept"))
	assert.Equal(t, apiVersion, req.Header.Get("anthropic-version"))
	assert.Equal(t, "sk-ant-api03-abc123", req.Header.Get("x-api-key"))
	assert.Empty(t, req.Header.Get("Authorization"))
	assert.Empty(t, req.Header.Get("anthropic-beta"))
	assert.Empty(t, req.Header.Get("anthropic-dangerous-direct-browser-access"))
}

func TestMergeBetaHeaders_NoDuplicates(t *testing.T) {
	result := MergeBetaHeaders("oauth-2025-04-20,claude-code-20250219", "oauth-2025-04-20")
	assert.Contains(t, result, "oauth-2025-04-20")
	assert.Contains(t, result, "claude-code-20250219")
	assert.Equal(t, 1, strings.Count(result, "oauth-2025-04-20"),
		"oauth-2025-04-20 should appear exactly once")
}

func TestMergeBetaHeaders_Empty(t *testing.T) {
	tests := []struct {
		name       string
		existing   string
		additional string
		wantParts  []string
	}{
		{"empty existing", "", "oauth-2025-04-20", []string{"oauth-2025-04-20"}},
		{"empty additional", "claude-code-20250219", "", []string{"claude-code-20250219"}},
		{"both empty", "", "", nil},
		{"both with values", "foo,bar", "baz", []string{"foo", "bar", "baz"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MergeBetaHeaders(tt.existing, tt.additional)
			if tt.wantParts == nil {
				assert.Empty(t, result)
				return
			}
			for _, p := range tt.wantParts {
				assert.Contains(t, result, p)
			}
		})
	}
}

func TestSetupHeaders_EmptyKey(t *testing.T) {
	p := New()
	req, err := http.NewRequest(http.MethodPost, "https://api.anthropic.com/v1/messages", nil)
	require.NoError(t, err)

	p.SetupHeaders(req, "")

	assert.Equal(t, "", req.Header.Get("x-api-key"))
	assert.Empty(t, req.Header.Get("Authorization"))
	assert.Empty(t, req.Header.Get("anthropic-beta"))
}
