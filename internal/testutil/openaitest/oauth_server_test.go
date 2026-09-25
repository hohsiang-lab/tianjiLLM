package openaitest

import (
	"context"
	"net/http"
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/config"
	openaiProvider "github.com/praxisllmlab/tianjiLLM/internal/provider/openai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOAuthMockServer_ExchangeCodeRecordsPublicClientPKCE(t *testing.T) {
	server := NewOAuthServer(t, OAuthServerOptions{
		TokenFixtures: []TokenFixture{{
			Code: "callback-code",
			Response: map[string]any{
				"access_token":  "access-token",
				"refresh_token": "refresh-token",
				"id_token":      "id-token",
				"token_type":    "Bearer",
				"expires_in":    3600,
			},
		}},
	})

	cfg := config.OpenAIOAuthConfig{TokenURL: server.TokenURL(), ClientID: "app_test"}
	bundle, err := openaiProvider.ExchangeCode(context.Background(), server.Client(), cfg, "callback-code", "https://tianji.example.com/oauth/openai/callback", "verifier123")
	require.NoError(t, err)
	assert.Equal(t, "access-token", bundle.AccessToken)
	assert.Equal(t, "refresh-token", bundle.RefreshToken)

	requests := server.Requests()
	require.Len(t, requests, 1)
	req := requests[0]
	assert.Equal(t, http.MethodPost, req.Method)
	assert.Equal(t, "/oauth/token", req.Path)
	assert.Equal(t, "authorization_code", req.Form.Get("grant_type"))
	assert.Equal(t, "callback-code", req.Form.Get("code"))
	assert.Equal(t, "app_test", req.Form.Get("client_id"))
	assert.Equal(t, "verifier123", req.Form.Get("code_verifier"))
	server.AssertPublicClientPKCE(t, req)
}

func TestOAuthMockServer_RefreshTokenRotationFixture(t *testing.T) {
	server := NewOAuthServer(t, OAuthServerOptions{
		TokenFixtures: []TokenFixture{{
			RefreshToken: "old-refresh-token",
			Response: map[string]any{
				"access_token":  "new-access-token",
				"refresh_token": "new-refresh-token",
				"token_type":    "Bearer",
				"expires_in":    7200,
			},
		}},
	})

	form := map[string]string{
		"grant_type":    "refresh_token",
		"refresh_token": "old-refresh-token",
		"client_id":     "app_test",
	}
	resp := server.PostTokenForm(t, form)

	assert.Equal(t, "new-access-token", resp["access_token"])
	assert.Equal(t, "new-refresh-token", resp["refresh_token"])
	requests := server.Requests()
	require.Len(t, requests, 1)
	assert.Equal(t, "refresh_token", requests[0].Form.Get("grant_type"))
	assert.Equal(t, "old-refresh-token", requests[0].Form.Get("refresh_token"))
	server.AssertPublicClientPKCE(t, requests[0])
}
