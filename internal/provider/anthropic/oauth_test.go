package anthropic

import (
	"net/http"
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

	SetOAuthHeaders(req, "sk-ant-oat01-abc123")

	assert.Equal(t, "Bearer sk-ant-oat01-abc123", req.Header.Get("Authorization"))
	assert.Equal(t, OAuthBetaHeader, req.Header.Get("anthropic-beta"))
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

func TestSetupHeaders_EmptyKey(t *testing.T) {
	p := New()
	req, err := http.NewRequest(http.MethodPost, "https://api.anthropic.com/v1/messages", nil)
	require.NoError(t, err)

	p.SetupHeaders(req, "")

	assert.Equal(t, "", req.Header.Get("x-api-key"))
	assert.Empty(t, req.Header.Get("Authorization"))
	assert.Empty(t, req.Header.Get("anthropic-beta"))
}

func TestSetOAuthHeaders_MergeExistingBeta(t *testing.T) {
	req, err := http.NewRequest(http.MethodPost, "https://api.anthropic.com/v1/messages", nil)
	require.NoError(t, err)

	// Pre-set a beta header that should be preserved
	req.Header.Set("anthropic-beta", "prompt-caching-2024-07-31")

	SetOAuthHeaders(req, "sk-ant-oat01-abc123")

	got := req.Header.Get("anthropic-beta")
	assert.Equal(t, "prompt-caching-2024-07-31,"+OAuthBetaHeader, got)
}

func TestSetOAuthHeaders_NoBetaPreset(t *testing.T) {
	req, err := http.NewRequest(http.MethodPost, "https://api.anthropic.com/v1/messages", nil)
	require.NoError(t, err)

	SetOAuthHeaders(req, "sk-ant-oat01-abc123")

	assert.Equal(t, OAuthBetaHeader, req.Header.Get("anthropic-beta"))
}
