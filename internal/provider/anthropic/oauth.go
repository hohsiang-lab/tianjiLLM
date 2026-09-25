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

// MergeBetaHeaders combines two comma-separated beta header strings,
// deduplicating values. Returns empty string if both inputs are empty.
func MergeBetaHeaders(existing, additional string) string {
	seen := make(map[string]struct{})
	var parts []string
	for _, s := range []string{existing, additional} {
		for _, p := range strings.Split(s, ",") {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			if _, ok := seen[p]; !ok {
				seen[p] = struct{}{}
				parts = append(parts, p)
			}
		}
	}
	return strings.Join(parts, ",")
}

// SetOAuthHeaders replaces x-api-key auth with OAuth Bearer token auth.
// Preserves any existing anthropic-beta values by merging (dedup) with the
// required OAuth beta header.
func SetOAuthHeaders(req *http.Request, apiKey string) {
	req.Header.Del("x-api-key")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("anthropic-beta", MergeBetaHeaders(
		req.Header.Get("anthropic-beta"), OAuthBetaHeader,
	))
	req.Header.Set("anthropic-dangerous-direct-browser-access", "true")
}
