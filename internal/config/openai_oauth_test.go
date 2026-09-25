package config

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultOpenAICodexBackendClientVersionBuildOverride(t *testing.T) {
	expected := os.Getenv("EXPECTED_CODEX_CLIENT_VERSION")
	if expected == "" {
		t.Skip("EXPECTED_CODEX_CLIENT_VERSION is only set by build metadata verification")
	}
	assert.Equal(t, expected, DefaultOpenAICodexBackendClientVersion)
}

func TestOpenAIOAuthConfig_Defaults(t *testing.T) {
	cfg := ResolveOpenAIOAuthConfig(OpenAIOAuthConfig{})

	assert.Equal(t, DefaultOpenAIOAuthIssuerURL, cfg.IssuerURL)
	assert.Equal(t, "https://auth.openai.com/oauth/authorize", cfg.AuthorizeURL)
	assert.Equal(t, "https://auth.openai.com/oauth/token", cfg.TokenURL)
	assert.Equal(t, "http://localhost:1455/auth/callback", cfg.RedirectURI)
	assert.Equal(t, DefaultOpenAIOAuthClientID, cfg.ClientID)
	assert.Equal(t, []string{"openid", "profile", "email", "offline_access", "api.connectors.read", "api.connectors.invoke"}, cfg.Scopes)
	assert.NotEmpty(t, cfg.Originator)
	assert.Equal(t, DefaultOpenAIOAuthOriginator, cfg.Originator)
	assert.Equal(t, DefaultOpenAICodexBackendBaseURL, cfg.CodexBackendBaseURL)
	assert.Equal(t, DefaultOpenAICodexBackendOriginator, cfg.CodexBackendOriginator)
	assert.Equal(t, DefaultOpenAICodexBackendClientVersion, cfg.CodexBackendClientVersion)
	assert.True(t, cfg.IDTokenAddOrganizations)
	assert.True(t, cfg.CodexCLISimplifiedFlow)
}

func TestResolveOpenAIOAuthRedirectURI_DefaultsToLocalhostCallback(t *testing.T) {
	redirectURI, err := ResolveOpenAIOAuthRedirectURI(OpenAIOAuthConfig{})

	require.NoError(t, err)
	assert.Equal(t, "http://localhost:1455/auth/callback", redirectURI)
}

func TestResolveOpenAIOAuthRedirectURI_UsesExplicitOverride(t *testing.T) {
	redirectURI, err := ResolveOpenAIOAuthRedirectURI(OpenAIOAuthConfig{
		RedirectURI: "http://127.0.0.1:1455/auth/callback",
	})

	require.NoError(t, err)
	assert.Equal(t, "http://127.0.0.1:1455/auth/callback", redirectURI)
}

func TestOpenAIOAuthConfig_EndpointOverrides(t *testing.T) {
	cfg := ResolveOpenAIOAuthConfig(OpenAIOAuthConfig{
		IssuerURL:    "http://127.0.0.1:18080",
		AuthorizeURL: "http://127.0.0.1:18080/custom/authorize",
		TokenURL:     "http://127.0.0.1:18080/custom/token",
	})

	assert.Equal(t, "http://127.0.0.1:18080", cfg.IssuerURL)
	assert.Equal(t, "http://127.0.0.1:18080/custom/authorize", cfg.AuthorizeURL)
	assert.Equal(t, "http://127.0.0.1:18080/custom/token", cfg.TokenURL)
	assert.False(t, strings.HasSuffix(cfg.AuthorizeURL, "/oauth/authorize/oauth/authorize"))
	assert.False(t, strings.HasSuffix(cfg.TokenURL, "/oauth/token/oauth/token"))
}

func TestOpenAIOAuthConfig_ClientIDOverride(t *testing.T) {
	cfg := ResolveOpenAIOAuthConfig(OpenAIOAuthConfig{ClientID: "app_test_override"})

	assert.Equal(t, "app_test_override", cfg.ClientID)
}

func TestOpenAIOAuthConfig_CodexBackendOverrides(t *testing.T) {
	cfg := ResolveOpenAIOAuthConfig(OpenAIOAuthConfig{
		CodexBackendBaseURL:       "https://example.test/backend-api/",
		CodexBackendOriginator:    "custom_origin",
		CodexBackendClientVersion: "0.145.0",
	})

	assert.Equal(t, "https://example.test/backend-api", cfg.CodexBackendBaseURL)
	assert.Equal(t, "custom_origin", cfg.CodexBackendOriginator)
	assert.Equal(t, "0.145.0", cfg.CodexBackendClientVersion)
	assert.Equal(t, DefaultOpenAIOAuthOriginator, cfg.Originator, "backend originator must not change OAuth authorize originator")
}
