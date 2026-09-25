package config

import (
	"fmt"
	"net/url"
	"strings"
)

const (
	DefaultOpenAIOAuthIssuerURL         = "https://auth.openai.com"
	DefaultOpenAIOAuthAuthorizeURL      = "https://auth.openai.com/oauth/authorize"
	DefaultOpenAIOAuthTokenURL          = "https://auth.openai.com/oauth/token"
	DefaultOpenAIOAuthRedirectURI       = "http://localhost:1455/auth/callback"
	DefaultOpenAIOAuthClientID          = "app_EMoamEEZ73f0CkXaXp7hrann"
	DefaultOpenAIOAuthOriginator        = "tianjillm"
	DefaultOpenAICodexBackendBaseURL    = "https://chatgpt.com/backend-api"
	DefaultOpenAICodexBackendOriginator = "codex_cli_rs"

	openAIOAuthLocalhostCallbackPath = "/auth/callback"
)

// DefaultOpenAICodexBackendClientVersion is a build-time compatibility value.
// Release builds override it from .codex-client-version using -ldflags.
var DefaultOpenAICodexBackendClientVersion = "0.144.1"

var DefaultOpenAIOAuthScopes = []string{
	"openid",
	"profile",
	"email",
	"offline_access",
	"api.connectors.read",
	"api.connectors.invoke",
}

// OpenAIOAuthConfig holds deployment-level OpenAI OAuth public-client metadata.
type OpenAIOAuthConfig struct {
	Enabled                   bool     `yaml:"enabled,omitempty"`
	IssuerURL                 string   `yaml:"issuer_url,omitempty"`
	AuthorizeURL              string   `yaml:"authorize_url,omitempty"`
	TokenURL                  string   `yaml:"token_url,omitempty"`
	RedirectURI               string   `yaml:"redirect_uri,omitempty"`
	ClientID                  string   `yaml:"client_id,omitempty"`
	Scopes                    []string `yaml:"scopes,omitempty"`
	Originator                string   `yaml:"originator,omitempty"`
	CodexBackendBaseURL       string   `yaml:"codex_backend_base_url,omitempty"`
	CodexBackendOriginator    string   `yaml:"codex_backend_originator,omitempty"`
	CodexBackendClientVersion string   `yaml:"codex_backend_client_version,omitempty"`
	IDTokenAddOrganizations   bool     `yaml:"id_token_add_organizations,omitempty"`
	CodexCLISimplifiedFlow    bool     `yaml:"codex_cli_simplified_flow,omitempty"`
}

// ResolveOpenAIOAuthConfig applies OpenAI OAuth defaults and endpoint/client/scope overrides.
func ResolveOpenAIOAuthConfig(input OpenAIOAuthConfig) OpenAIOAuthConfig {
	cfg := input
	if cfg.IssuerURL == "" {
		cfg.IssuerURL = DefaultOpenAIOAuthIssuerURL
	}
	cfg.IssuerURL = strings.TrimRight(cfg.IssuerURL, "/")
	if cfg.AuthorizeURL == "" {
		cfg.AuthorizeURL = cfg.IssuerURL + "/oauth/authorize"
	}
	if cfg.TokenURL == "" {
		cfg.TokenURL = cfg.IssuerURL + "/oauth/token"
	}
	if cfg.RedirectURI == "" {
		cfg.RedirectURI = DefaultOpenAIOAuthRedirectURI
	}
	if cfg.ClientID == "" {
		cfg.ClientID = DefaultOpenAIOAuthClientID
	}
	if len(cfg.Scopes) == 0 {
		cfg.Scopes = append([]string(nil), DefaultOpenAIOAuthScopes...)
	}
	if cfg.Originator == "" {
		cfg.Originator = DefaultOpenAIOAuthOriginator
	}
	if cfg.CodexBackendBaseURL == "" {
		cfg.CodexBackendBaseURL = DefaultOpenAICodexBackendBaseURL
	}
	cfg.CodexBackendBaseURL = strings.TrimRight(cfg.CodexBackendBaseURL, "/")
	if cfg.CodexBackendOriginator == "" {
		cfg.CodexBackendOriginator = DefaultOpenAICodexBackendOriginator
	}
	if cfg.CodexBackendClientVersion == "" {
		cfg.CodexBackendClientVersion = DefaultOpenAICodexBackendClientVersion
	}
	// These OpenAI/Codex flow flags are default-on. The yaml fields are intentionally
	// not pointer booleans so this slice avoids adding tri-state config until a real
	// deployment needs to disable them.
	cfg.IDTokenAddOrganizations = true
	cfg.CodexCLISimplifiedFlow = true
	return cfg
}

func ResolveOpenAIOAuthRedirectURI(input OpenAIOAuthConfig) (string, error) {
	cfg := ResolveOpenAIOAuthConfig(input)
	parsed, err := url.Parse(strings.TrimSpace(cfg.RedirectURI))
	if err != nil {
		return "", fmt.Errorf("parse openai oauth redirect URI: %w", err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("openai oauth redirect URI must be absolute")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", fmt.Errorf("openai oauth redirect URI must not include query or fragment")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("openai oauth redirect URI must use http or https")
	}
	if parsed.Scheme == "http" && parsed.Hostname() != "localhost" && parsed.Hostname() != "127.0.0.1" && parsed.Hostname() != "::1" {
		return "", fmt.Errorf("openai oauth http redirect URI must use localhost")
	}
	if parsed.Path != openAIOAuthLocalhostCallbackPath && parsed.Scheme == "http" {
		return "", fmt.Errorf("openai oauth localhost redirect URI must use %s", openAIOAuthLocalhostCallbackPath)
	}
	return parsed.String(), nil
}
