package anthropic

import (
	"net/http"
	"strings"
)

const (
	OAuthTokenPrefix = "sk-ant-oat"
	OAuthBetaHeader  = "oauth-2025-04-20"
)

// IsOAuthToken checks if the API key is an Anthropic OAuth token.
func IsOAuthToken(apiKey string) bool {
	return strings.HasPrefix(apiKey, OAuthTokenPrefix)
}

// SetOAuthHeaders replaces x-api-key auth with OAuth Bearer token auth.
func SetOAuthHeaders(req *http.Request, apiKey string) {
	req.Header.Del("x-api-key")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	if existing := req.Header.Get("anthropic-beta"); existing != "" {
		req.Header.Set("anthropic-beta", existing+","+OAuthBetaHeader)
	} else {
		req.Header.Set("anthropic-beta", OAuthBetaHeader)
	}
	req.Header.Set("anthropic-dangerous-direct-browser-access", "true")
}
