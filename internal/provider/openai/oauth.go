package openai

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/praxisllmlab/tianjiLLM/internal/security/redact"
)

const (
	codeVerifierBytes = 64
	stateBytes        = 32
	oauthMaxBodyBytes = 1 << 20
)

var errOAuthBodyTooLarge = errors.New("response too large")

type PKCEPair struct {
	CodeVerifier        string
	CodeChallenge       string
	CodeChallengeMethod string
}

type TokenBundle struct {
	AccessToken  string         `json:"access_token"`
	RefreshToken string         `json:"refresh_token"`
	IDToken      string         `json:"id_token"`
	TokenType    string         `json:"token_type"`
	ExpiresIn    int            `json:"expires_in"`
	Scope        string         `json:"scope"`
	Raw          map[string]any `json:"-"`
}

func GeneratePKCE() (PKCEPair, error) {
	verifier, err := randomURLSafe(codeVerifierBytes)
	if err != nil {
		return PKCEPair{}, fmt.Errorf("generate pkce verifier: %w", err)
	}
	return PKCEPair{
		CodeVerifier:        verifier,
		CodeChallenge:       ChallengeFromVerifier(verifier),
		CodeChallengeMethod: "S256",
	}, nil
}

func ChallengeFromVerifier(verifier string) string {
	digest := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(digest[:])
}

func readOAuthResponseBody(reader io.Reader) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(reader, oauthMaxBodyBytes+1))
	if err != nil {
		return nil, err
	}
	if len(body) > oauthMaxBodyBytes {
		return nil, errOAuthBodyTooLarge
	}
	return body, nil
}

func GenerateState() (string, error) {
	state, err := randomURLSafe(stateBytes)
	if err != nil {
		return "", fmt.Errorf("generate oauth state: %w", err)
	}
	return state, nil
}

func BuildAuthorizeURL(cfg config.OpenAIOAuthConfig, redirectURI, codeChallenge, state string) (string, error) {
	resolved := config.ResolveOpenAIOAuthConfig(cfg)
	parsed, err := url.Parse(resolved.AuthorizeURL)
	if err != nil {
		return "", fmt.Errorf("parse openai authorize URL: %w", err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("openai authorize URL must be absolute")
	}
	query := parsed.Query()
	query.Set("response_type", "code")
	query.Set("client_id", resolved.ClientID)
	query.Set("redirect_uri", redirectURI)
	query.Set("scope", strings.Join(resolved.Scopes, " "))
	query.Set("code_challenge", codeChallenge)
	query.Set("code_challenge_method", "S256")
	query.Set("state", state)
	if resolved.IDTokenAddOrganizations {
		query.Set("id_token_add_organizations", "true")
	}
	if resolved.CodexCLISimplifiedFlow {
		query.Set("codex_cli_simplified_flow", "true")
	}
	if resolved.Originator != "" {
		query.Set("originator", resolved.Originator)
	}
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func ExchangeCode(ctx context.Context, client *http.Client, cfg config.OpenAIOAuthConfig, code, redirectURI, codeVerifier string) (*TokenBundle, error) {
	if client == nil {
		client = http.DefaultClient
	}
	resolved := config.ResolveOpenAIOAuthConfig(cfg)
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", redirectURI)
	form.Set("client_id", resolved.ClientID)
	form.Set("code_verifier", codeVerifier)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, resolved.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("create openai token exchange request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("openai token exchange: %w", err)
	}
	defer resp.Body.Close()

	body, err := readOAuthResponseBody(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read openai token response: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("openai token exchange failed (%d): %s", resp.StatusCode, string(redact.RawJSON(body)))
	}

	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("parse openai token response: %w", err)
	}
	var bundle TokenBundle
	if err := json.Unmarshal(body, &bundle); err != nil {
		return nil, fmt.Errorf("parse openai token bundle: %w", err)
	}
	bundle.Raw = raw
	return &bundle, nil
}

func RefreshToken(ctx context.Context, client *http.Client, cfg config.OpenAIOAuthConfig, refreshToken string) (*TokenBundle, error) {
	if client == nil {
		client = http.DefaultClient
	}
	resolved := config.ResolveOpenAIOAuthConfig(cfg)
	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", refreshToken)
	form.Set("client_id", resolved.ClientID)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, resolved.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("create openai token refresh request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("openai token refresh: %w", err)
	}
	defer resp.Body.Close()

	body, err := readOAuthResponseBody(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read openai token refresh response: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("openai token refresh failed (%d): %s", resp.StatusCode, string(redact.RawJSON(body)))
	}

	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("parse openai token refresh response: %w", err)
	}
	var bundle TokenBundle
	if err := json.Unmarshal(body, &bundle); err != nil {
		return nil, fmt.Errorf("parse openai token refresh bundle: %w", err)
	}
	bundle.Raw = raw
	return &bundle, nil
}

func randomURLSafe(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
