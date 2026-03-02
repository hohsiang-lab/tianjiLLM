package passthrough

import (
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/praxisllmlab/tianjiLLM/internal/provider/anthropic"
)

// Config holds pass-through proxy configuration.
type Config struct {
	// ProviderEndpoints maps route prefixes to upstream URLs.
	// e.g., "/anthropic" → "https://api.anthropic.com"
	ProviderEndpoints map[string]string
	APIKeys           map[string]string // provider → API key
}

// Handler creates an HTTP handler for pass-through proxy.
func Handler(cfg Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Find matching provider
		var upstreamBase string
		var providerName string
		for prefix, base := range cfg.ProviderEndpoints {
			if strings.HasPrefix(r.URL.Path, prefix) {
				upstreamBase = base
				providerName = strings.TrimPrefix(prefix, "/")
				r.URL.Path = strings.TrimPrefix(r.URL.Path, prefix)
				break
			}
		}

		if upstreamBase == "" {
			http.Error(w, `{"error":"unknown pass-through endpoint"}`, http.StatusNotFound)
			return
		}

		target, err := url.Parse(upstreamBase)
		if err != nil {
			http.Error(w, `{"error":"invalid upstream URL"}`, http.StatusInternalServerError)
			return
		}

		proxy := &httputil.ReverseProxy{
			Director: func(req *http.Request) {
				req.URL.Scheme = target.Scheme
				req.URL.Host = target.Host
				req.URL.Path = target.Path + req.URL.Path
				req.Host = target.Host

				// Set provider-specific auth
				if apiKey, ok := cfg.APIKeys[providerName]; ok {
					switch providerName {
					case "anthropic":
						if anthropic.IsOAuthToken(apiKey) {
							anthropic.SetOAuthHeaders(req, apiKey)
						} else {
							req.Header.Set("x-api-key", apiKey)
						}
						if req.Header.Get("anthropic-version") == "" {
							req.Header.Set("anthropic-version", "2023-06-01")
						}
					default:
						req.Header.Set("Authorization", "Bearer "+apiKey)
					}
				}
			},
			ModifyResponse: func(resp *http.Response) error {
				// Extract usage for spend tracking (future use)
				return nil
			},
			ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
				log.Printf("pass-through proxy error: %v", err)
				http.Error(w, `{"error":"upstream request failed"}`, http.StatusBadGateway)
			},
		}

		proxy.ServeHTTP(w, r)
	}
}

// ExtractAnthropicUsage extracts token usage from an Anthropic response body.
func ExtractAnthropicUsage(body io.Reader) (promptTokens, completionTokens int) {
	// Simple extraction — in production, this would parse the response JSON
	// without consuming the body (using TeeReader)
	return 0, 0
}
