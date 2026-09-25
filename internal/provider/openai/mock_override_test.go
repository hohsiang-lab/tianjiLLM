package openai

import (
	"context"
	"net/http"
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/praxisllmlab/tianjiLLM/internal/model"
	"github.com/praxisllmlab/tianjiLLM/internal/testutil/openaitest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenAIOAuthEndpointOverrides_UseMockURLsOnly(t *testing.T) {
	mock := openaitest.NewOAuthServer(t, openaitest.OAuthServerOptions{
		TokenFixtures: []openaitest.TokenFixture{{
			Code:     "callback-code",
			Response: map[string]any{"access_token": "access", "token_type": "Bearer", "expires_in": 3600},
		}},
	})
	cfg := config.OpenAIOAuthConfig{
		AuthorizeURL: mock.AuthorizeURL(),
		TokenURL:     mock.TokenURL(),
		ClientID:     "app_test",
	}

	authURL, err := BuildAuthorizeURL(cfg, "https://tianji.example.com/oauth/openai/callback", "challenge", "state")
	require.NoError(t, err)
	assert.Contains(t, authURL, mock.AuthorizeURL())
	assert.NotContains(t, authURL, "auth.openai.com")

	client := openaitest.NewGuardedClient(mock.Host())
	bundle, err := ExchangeCode(context.Background(), client, cfg, "callback-code", "https://tianji.example.com/oauth/openai/callback", "verifier")
	require.NoError(t, err)
	assert.Equal(t, "access", bundle.AccessToken)
	requests := mock.Requests()
	require.Len(t, requests, 1)
	assert.Equal(t, mock.Host(), requests[0].Host)
}

func TestOpenAIProviderBaseURLOverride_UsesMockUpstreamOnly(t *testing.T) {
	mock := openaitest.NewUpstreamServer(t, openaitest.UpstreamServerOptions{})
	provider := NewWithBaseURL(mock.BaseURL())
	req, err := provider.TransformRequest(context.Background(), &model.ChatCompletionRequest{
		Model:    "gpt-4o",
		Messages: []model.Message{{Role: "user", Content: "hi"}},
	}, "sk-test")
	require.NoError(t, err)
	assert.Equal(t, mock.BaseURL()+"/chat/completions", req.URL.String())
	assert.NotContains(t, req.URL.String(), "api.openai.com")

	resp, err := openaitest.NewGuardedClient(mock.Host()).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	requests := mock.Requests()
	require.Len(t, requests, 1)
	assert.Equal(t, "/v1/chat/completions", requests[0].Path)
	assert.Equal(t, mock.Host(), requests[0].Host)
}

func TestOpenAIEmbeddingBaseURLOverride_UsesMockUpstreamOnly(t *testing.T) {
	mock := openaitest.NewUpstreamServer(t, openaitest.UpstreamServerOptions{})
	provider := NewWithBaseURL(mock.BaseURL())
	req, err := provider.TransformEmbeddingRequest(context.Background(), &model.EmbeddingRequest{
		Model: "text-embedding-3-small",
		Input: "hello",
	}, "sk-test")
	require.NoError(t, err)
	assert.Equal(t, mock.BaseURL()+"/embeddings", req.URL.String())
	assert.NotContains(t, req.URL.String(), "api.openai.com")

	resp, err := openaitest.NewGuardedClient(mock.Host()).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	requests := mock.Requests()
	require.Len(t, requests, 1)
	assert.Equal(t, "/v1/embeddings", requests[0].Path)
}
