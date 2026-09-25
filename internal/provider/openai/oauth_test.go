package openai

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGeneratePKCE_ProducesRFC7636S256Pair(t *testing.T) {
	pkce, err := GeneratePKCE()
	require.NoError(t, err)

	assert.GreaterOrEqual(t, len(pkce.CodeVerifier), 43)
	assert.LessOrEqual(t, len(pkce.CodeVerifier), 128)
	assert.Regexp(t, regexp.MustCompile(`^[A-Za-z0-9_-]+$`), pkce.CodeVerifier)
	assert.NotContains(t, pkce.CodeVerifier, "=")

	digest := sha256.Sum256([]byte(pkce.CodeVerifier))
	expected := base64.RawURLEncoding.EncodeToString(digest[:])
	assert.Equal(t, expected, pkce.CodeChallenge)
	assert.Equal(t, "S256", pkce.CodeChallengeMethod)
}

func TestGeneratePKCE_KnownVerifierChallenge(t *testing.T) {
	challenge := ChallengeFromVerifier("dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk")

	assert.Equal(t, "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM", challenge)
}

func TestGenerateState_UniqueURLSafe(t *testing.T) {
	first, err := GenerateState()
	require.NoError(t, err)
	second, err := GenerateState()
	require.NoError(t, err)

	assert.NotEmpty(t, first)
	assert.NotEmpty(t, second)
	assert.NotEqual(t, first, second)
	assert.Regexp(t, regexp.MustCompile(`^[A-Za-z0-9_-]+$`), first)
	assert.Regexp(t, regexp.MustCompile(`^[A-Za-z0-9_-]+$`), second)
}

func TestBuildAuthorizeURL_OpenAIPKCEFields(t *testing.T) {
	cfg := config.ResolveOpenAIOAuthConfig(config.OpenAIOAuthConfig{
		AuthorizeURL: "https://auth.example.test/oauth/authorize",
		ClientID:     "app_test",
		Originator:   "tianjillm-test",
	})

	authURL, err := BuildAuthorizeURL(cfg, "https://tianji.example.com/oauth/openai/callback", "challenge123", "state123")
	require.NoError(t, err)

	parsed, err := url.Parse(authURL)
	require.NoError(t, err)
	query := parsed.Query()
	assert.Equal(t, "https", parsed.Scheme)
	assert.Equal(t, "auth.example.test", parsed.Host)
	assert.Equal(t, "/oauth/authorize", parsed.Path)
	assert.Equal(t, "code", query.Get("response_type"))
	assert.Equal(t, "app_test", query.Get("client_id"))
	assert.Equal(t, "https://tianji.example.com/oauth/openai/callback", query.Get("redirect_uri"))
	assert.Equal(t, "openid profile email offline_access api.connectors.read api.connectors.invoke", query.Get("scope"))
	assert.Equal(t, "challenge123", query.Get("code_challenge"))
	assert.Equal(t, "S256", query.Get("code_challenge_method"))
	assert.Equal(t, "state123", query.Get("state"))
	assert.Equal(t, "true", query.Get("id_token_add_organizations"))
	assert.Equal(t, "true", query.Get("codex_cli_simplified_flow"))
	assert.Equal(t, "tianjillm-test", query.Get("originator"))
}

func TestExchangeCode_PublicClientPKCERequestShape(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.NoError(t, r.ParseForm())
		assert.Equal(t, "authorization_code", r.Form.Get("grant_type"))
		assert.Equal(t, "callback-code", r.Form.Get("code"))
		assert.Equal(t, "https://tianji.example.com/oauth/openai/callback", r.Form.Get("redirect_uri"))
		assert.Equal(t, "app_test", r.Form.Get("client_id"))
		assert.Equal(t, "verifier123", r.Form.Get("code_verifier"))
		_, hasClientSecret := r.Form["client_secret"]
		assert.False(t, hasClientSecret)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"access","refresh_token":"refresh","id_token":"id","token_type":"Bearer","expires_in":3600,"scope":"openid email"}`))
	}))
	defer server.Close()

	cfg := config.ResolveOpenAIOAuthConfig(config.OpenAIOAuthConfig{TokenURL: server.URL, ClientID: "app_test"})
	_, err := ExchangeCode(context.Background(), server.Client(), cfg, "callback-code", "https://tianji.example.com/oauth/openai/callback", "verifier123")
	require.NoError(t, err)
}

func TestExchangeCode_DoesNotSendClientSecret(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseForm())
		_, hasClientSecret := r.Form["client_secret"]
		assert.False(t, hasClientSecret)
		assert.NotContains(t, r.Header.Get("Authorization"), "Basic")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"access","token_type":"Bearer","expires_in":3600}`))
	}))
	defer server.Close()

	cfg := config.ResolveOpenAIOAuthConfig(config.OpenAIOAuthConfig{TokenURL: server.URL, ClientID: "app_test"})
	_, err := ExchangeCode(context.Background(), server.Client(), cfg, "callback-code", "https://tianji.example.com/oauth/openai/callback", "verifier123")
	require.NoError(t, err)
}

func TestExchangeCode_ParsesTokenBundle(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"access","refresh_token":"refresh","id_token":"id","token_type":"Bearer","expires_in":3600,"scope":"openid profile","organization_id":"org_123"}`))
	}))
	defer server.Close()

	cfg := config.ResolveOpenAIOAuthConfig(config.OpenAIOAuthConfig{TokenURL: server.URL, ClientID: "app_test"})
	bundle, err := ExchangeCode(context.Background(), server.Client(), cfg, "callback-code", "https://tianji.example.com/oauth/openai/callback", "verifier123")
	require.NoError(t, err)

	assert.Equal(t, "access", bundle.AccessToken)
	assert.Equal(t, "refresh", bundle.RefreshToken)
	assert.Equal(t, "id", bundle.IDToken)
	assert.Equal(t, "Bearer", bundle.TokenType)
	assert.Equal(t, 3600, bundle.ExpiresIn)
	assert.Equal(t, "openid profile", bundle.Scope)
	assert.Equal(t, "org_123", bundle.Raw["organization_id"])
}

func TestExchangeCode_ErrorPreservesStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": "invalid_grant", "error_description": "expired code"})
	}))
	defer server.Close()

	cfg := config.ResolveOpenAIOAuthConfig(config.OpenAIOAuthConfig{TokenURL: server.URL, ClientID: "app_test"})
	_, err := ExchangeCode(context.Background(), server.Client(), cfg, "callback-code", "https://tianji.example.com/oauth/openai/callback", "secret-verifier")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "400")
	assert.Contains(t, err.Error(), "invalid_grant")
	assert.NotContains(t, err.Error(), "secret-verifier")
}

func TestExchangeCode_RejectsOversizedResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(bytes.Repeat([]byte("x"), oauthMaxBodyBytes+1))
	}))
	defer server.Close()

	cfg := config.ResolveOpenAIOAuthConfig(config.OpenAIOAuthConfig{TokenURL: server.URL, ClientID: "app_test"})
	_, err := ExchangeCode(context.Background(), server.Client(), cfg, "[REDACTED]", "https://tianji.example.com/oauth/openai/callback", "[REDACTED]")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "response too large")
	_, sizeErr := readOAuthResponseBody(bytes.NewReader(bytes.Repeat([]byte("x"), oauthMaxBodyBytes+1)))
	assert.ErrorIs(t, err, sizeErr)
}

func TestOpenAIRefreshToken_PublicClientRequestShape(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.NoError(t, r.ParseForm())
		assert.Equal(t, "refresh_token", r.Form.Get("grant_type"))
		assert.Equal(t, "refresh-secret", r.Form.Get("refresh_token"))
		assert.Equal(t, "app_test", r.Form.Get("client_id"))
		_, hasClientSecret := r.Form["client_secret"]
		assert.False(t, hasClientSecret)
		assert.NotContains(t, r.Header.Get("Authorization"), "Basic")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"access","token_type":"Bearer","expires_in":3600}`))
	}))
	defer server.Close()

	cfg := config.ResolveOpenAIOAuthConfig(config.OpenAIOAuthConfig{TokenURL: server.URL, ClientID: "app_test"})
	_, err := RefreshToken(context.Background(), server.Client(), cfg, "refresh-secret")
	require.NoError(t, err)
}

func TestOpenAIRefreshToken_ParsesRotatedRefreshToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"new-access","refresh_token":"new-refresh","token_type":"Bearer","expires_in":1800,"scope":"openid email","account_id":"acct_123"}`))
	}))
	defer server.Close()

	cfg := config.ResolveOpenAIOAuthConfig(config.OpenAIOAuthConfig{TokenURL: server.URL, ClientID: "app_test"})
	bundle, err := RefreshToken(context.Background(), server.Client(), cfg, "old-refresh")
	require.NoError(t, err)

	assert.Equal(t, "new-access", bundle.AccessToken)
	assert.Equal(t, "new-refresh", bundle.RefreshToken)
	assert.Equal(t, "Bearer", bundle.TokenType)
	assert.Equal(t, 1800, bundle.ExpiresIn)
	assert.Equal(t, "openid email", bundle.Scope)
	assert.Equal(t, "acct_123", bundle.Raw["account_id"])
}

func TestOpenAIRefreshToken_ErrorRedactsRequestSecrets(t *testing.T) {
	const refreshSecret = "refresh_token=refresh-secret"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error":             "invalid_grant",
			"error_description": "token failed: " + refreshSecret,
		})
	}))
	defer server.Close()

	cfg := config.ResolveOpenAIOAuthConfig(config.OpenAIOAuthConfig{TokenURL: server.URL, ClientID: "app_test"})
	_, err := RefreshToken(context.Background(), server.Client(), cfg, "refresh-secret")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "400")
	assert.Contains(t, err.Error(), "invalid_grant")
	assert.NotContains(t, err.Error(), "refresh-secret")
}
