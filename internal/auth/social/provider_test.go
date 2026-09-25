package social

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func jsonResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestGitHubAuthorizationURLUsesStateAndPKCE(t *testing.T) {
	provider, err := NewGitHubProvider("client-id", "client-secret", "https://tianji.example")
	require.NoError(t, err)

	verifier := oauth2.GenerateVerifier()
	authURL, err := url.Parse(provider.AuthorizationURL("state-value", verifier))
	require.NoError(t, err)

	query := authURL.Query()
	assert.Equal(t, "state-value", query.Get("state"))
	assert.Equal(t, "S256", query.Get("code_challenge_method"))
	assert.Equal(t, oauth2.S256ChallengeFromVerifier(verifier), query.Get("code_challenge"))
	assert.Equal(t, "https://tianji.example/auth/github/callback", query.Get("redirect_uri"))
	assert.ElementsMatch(t, []string{"read:user", "user:email"}, strings.Fields(query.Get("scope")))
}

func TestGitHubCompleteUsesVerifierAndPrimaryVerifiedEmail(t *testing.T) {
	provider, err := NewGitHubProvider("client-id", "client-secret", "https://tianji.example")
	require.NoError(t, err)
	provider.oauth.Endpoint.AuthStyle = oauth2.AuthStyleInParams

	var mu sync.Mutex
	var receivedVerifier string
	var emailRequests int
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.String() {
		case "https://github.com/login/oauth/access_token":
			require.NoError(t, req.ParseForm())
			mu.Lock()
			receivedVerifier = req.Form.Get("code_verifier")
			mu.Unlock()
			assert.Equal(t, "authorization-code", req.Form.Get("code"))
			return jsonResponse(http.StatusOK, `{"access_token":"github-access-token","token_type":"bearer"}`), nil
		case "https://api.github.com/user":
			assert.Equal(t, "Bearer github-access-token", req.Header.Get("Authorization"))
			return jsonResponse(http.StatusOK, `{
				"id": 1234,
				"login": "octocat",
				"name": "Octo Cat",
				"email": "",
				"avatar_url": "https://avatars.example/octocat"
			}`), nil
		case "https://api.github.com/user/emails":
			assert.Equal(t, "Bearer github-access-token", req.Header.Get("Authorization"))
			mu.Lock()
			emailRequests++
			mu.Unlock()
			return jsonResponse(http.StatusOK, `[
				{"email":"secondary@example.com","primary":false,"verified":true},
				{"email":"unverified@example.com","primary":true,"verified":false},
				{"email":"primary@example.com","primary":true,"verified":true}
			]`), nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL)
			return nil, nil
		}
	})}
	ctx := context.WithValue(context.Background(), oauth2.HTTPClient, client)

	verifier := oauth2.GenerateVerifier()
	identity, err := provider.Complete(ctx, "authorization-code", verifier)
	require.NoError(t, err)

	mu.Lock()
	assert.Equal(t, verifier, receivedVerifier)
	assert.Equal(t, 1, emailRequests)
	mu.Unlock()
	assert.Equal(t, Identity{
		Provider:       "github",
		ProviderUserID: "1234",
		Email:          "primary@example.com",
		EmailVerified:  true,
		DisplayName:    "Octo Cat",
		AvatarURL:      "https://avatars.example/octocat",
	}, identity)
}

func TestGitHubCompleteRejectsMissingPrimaryVerifiedEmail(t *testing.T) {
	provider, err := NewGitHubProvider("client-id", "client-secret", "https://tianji.example")
	require.NoError(t, err)
	provider.oauth.Endpoint.AuthStyle = oauth2.AuthStyleInParams

	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.String() {
		case "https://github.com/login/oauth/access_token":
			return jsonResponse(http.StatusOK, `{"access_token":"token","token_type":"bearer"}`), nil
		case "https://api.github.com/user":
			return jsonResponse(http.StatusOK, `{"id":1234,"login":"octocat","email":"profile@example.com"}`), nil
		case "https://api.github.com/user/emails":
			return jsonResponse(http.StatusOK, `[{"email":"nope@example.com","primary":true,"verified":false}]`), nil
		default:
			t.Fatalf("unexpected request: %s", req.URL)
			return nil, nil
		}
	})}
	ctx := context.WithValue(context.Background(), oauth2.HTTPClient, client)

	_, err = provider.Complete(ctx, "code", oauth2.GenerateVerifier())
	assert.ErrorIs(t, err, ErrUnverifiedEmail)
}

func TestDiscordCompleteRejectsUnverifiedEmail(t *testing.T) {
	provider, err := NewDiscordProvider("client-id", "client-secret", "https://tianji.example")
	require.NoError(t, err)
	provider.oauth.Endpoint.AuthStyle = oauth2.AuthStyleInParams

	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.String() {
		case discordTokenURL:
			require.NoError(t, req.ParseForm())
			assert.NotEmpty(t, req.Form.Get("code_verifier"))
			return jsonResponse(http.StatusOK, `{"access_token":"discord-token","token_type":"bearer"}`), nil
		case "https://discord.com/api/users/@me":
			assert.Equal(t, "Bearer discord-token", req.Header.Get("Authorization"))
			return jsonResponse(http.StatusOK, `{
				"id":"discord-user",
				"username":"tianji-user",
				"email":"user@example.com",
				"verified":false
			}`), nil
		default:
			t.Fatalf("unexpected request: %s", req.URL)
			return nil, nil
		}
	})}
	ctx := context.WithValue(context.Background(), oauth2.HTTPClient, client)
	service := NewService(provider)

	_, err = service.Complete(ctx, "discord", "code", oauth2.GenerateVerifier())
	assert.ErrorIs(t, err, ErrUnverifiedEmail)
}

func TestServiceRejectsUnknownProvider(t *testing.T) {
	service := NewService()

	_, err := service.Begin("unknown", "state", oauth2.GenerateVerifier())
	assert.ErrorIs(t, err, ErrUnknownProvider)

	_, err = service.Complete(context.Background(), "unknown", "code", oauth2.GenerateVerifier())
	assert.True(t, errors.Is(err, ErrUnknownProvider))
}

func TestCallbackURLRejectsNonHTTPURL(t *testing.T) {
	_, err := NewGitHubProvider("client-id", "client-secret", "javascript:alert(1)")
	assert.Error(t, err)

	_, err = NewGitHubProvider("client-id", "client-secret", "http://tianji.example")
	assert.Error(t, err)

	provider, err := NewGitHubProvider("client-id", "client-secret", "http://localhost:4000")
	require.NoError(t, err)
	assert.Equal(t, "http://localhost:4000/auth/github/callback", provider.oauth.RedirectURL)
}
