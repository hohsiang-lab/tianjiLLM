package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// SSOConfig holds OIDC/SSO configuration.
type SSOConfig struct {
	ClientID     string
	ClientSecret string
	AuthURL      string // authorization endpoint
	TokenURL     string // token endpoint
	UserInfoURL  string // user info endpoint
	RedirectURI  string
	Scopes       []string
	// RoleMapping maps IDP groups/roles to TianjiLLM roles.
	RoleMapping map[string]Role
}

// SSOHandler handles SSO login and callback flows.
type SSOHandler struct {
	config SSOConfig
}

// NewSSOHandler creates a new SSO handler.
func NewSSOHandler(cfg SSOConfig) *SSOHandler {
	if len(cfg.Scopes) == 0 {
		cfg.Scopes = []string{"openid", "profile", "email"}
	}
	return &SSOHandler{config: cfg}
}

// LoginURL returns the OIDC authorization URL to redirect the user to.
func (h *SSOHandler) LoginURL(state string) string {
	params := url.Values{
		"client_id":     {h.config.ClientID},
		"redirect_uri":  {h.config.RedirectURI},
		"response_type": {"code"},
		"scope":         {strings.Join(h.config.Scopes, " ")},
		"state":         {state},
	}
	return h.config.AuthURL + "?" + params.Encode()
}

// TokenResponse holds the OIDC token exchange response.
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	IDToken      string `json:"id_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
}

// ExchangeCode exchanges an authorization code for tokens.
func (h *SSOHandler) ExchangeCode(ctx context.Context, code string) (*TokenResponse, error) {
	data := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {h.config.RedirectURI},
		"client_id":     {h.config.ClientID},
		"client_secret": {h.config.ClientSecret},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.config.TokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("token exchange: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read token response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token exchange failed (%d): %s", resp.StatusCode, string(body))
	}

	var tokenResp TokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("parse token response: %w", err)
	}

	return &tokenResp, nil
}

// UserInfo holds the OIDC user info response.
type UserInfo struct {
	Sub    string   `json:"sub"`
	Email  string   `json:"email"`
	Name   string   `json:"name"`
	Groups []string `json:"groups"`
}

// GetUserInfo fetches user info from the OIDC provider.
func (h *SSOHandler) GetUserInfo(ctx context.Context, accessToken string) (*UserInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, h.config.UserInfoURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("user info request: %w", err)
	}
	defer resp.Body.Close()

	var info UserInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("parse user info: %w", err)
	}

	return &info, nil
}

// MapRole maps IDP groups to a TianjiLLM role using the configured mapping.
// Returns the highest-privilege matching role, or RoleInternalUser as default.
func (h *SSOHandler) MapRole(groups []string) Role {
	bestLevel := 0
	bestRole := RoleInternalUser

	for _, group := range groups {
		if role, ok := h.config.RoleMapping[group]; ok {
			if level := roleLevel[role]; level > bestLevel {
				bestLevel = level
				bestRole = role
			}
		}
	}

	return bestRole
}
