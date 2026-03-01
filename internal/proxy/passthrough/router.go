package passthrough

import (
	"bytes"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/praxisllmlab/tianjiLLM/internal/provider/anthropic"
)

// LoggingHandler extracts usage/metadata from provider-specific responses.
type LoggingHandler interface {
	// ParseUsage extracts token usage from a response body.
	// Returns promptTokens, completionTokens. Body must remain readable.
	ParseUsage(body []byte) (promptTokens, completionTokens int)
	// ProviderName returns the name of this provider.
	ProviderName() string
}

// GuardrailHook is called before and after pass-through requests.
type GuardrailHook interface {
	PreCall(r *http.Request, body []byte) error
	PostCall(r *http.Request, resp *http.Response, body []byte) error
}

// Endpoint represents a configured pass-through endpoint.
type Endpoint struct {
	Path     string // route path prefix, e.g. "/anthropic"
	Target   string // upstream URL, e.g. "https://api.anthropic.com"
	APIKey   string
	Provider string // provider name for auth header routing
}

// Router creates a pass-through router from configured endpoints.
type Router struct {
	endpoints []Endpoint
	loggers   map[string]LoggingHandler
	guardrail GuardrailHook
}

// NewRouter creates a new pass-through router.
func NewRouter(endpoints []Endpoint, guardrail GuardrailHook) *Router {
	return &Router{
		endpoints: endpoints,
		loggers:   make(map[string]LoggingHandler),
		guardrail: guardrail,
	}
}

// RegisterLogger adds a provider-specific logging handler.
func (rt *Router) RegisterLogger(providerName string, handler LoggingHandler) {
	rt.loggers[providerName] = handler
}

// Handler returns the HTTP handler for all pass-through routes.
func (rt *Router) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var matched *Endpoint
		var trimmedPath string
		for i := range rt.endpoints {
			ep := &rt.endpoints[i]
			if strings.HasPrefix(r.URL.Path, ep.Path) {
				matched = ep
				trimmedPath = strings.TrimPrefix(r.URL.Path, ep.Path)
				break
			}
		}

		if matched == nil {
			http.Error(w, `{"error":"unknown pass-through endpoint"}`, http.StatusNotFound)
			return
		}

		target, err := url.Parse(matched.Target)
		if err != nil {
			http.Error(w, `{"error":"invalid upstream URL"}`, http.StatusInternalServerError)
			return
		}

		// Pre-call guardrail
		if rt.guardrail != nil {
			body, _ := io.ReadAll(r.Body)
			r.Body = io.NopCloser(bytes.NewReader(body))
			if err := rt.guardrail.PreCall(r, body); err != nil {
				writeError(w, http.StatusForbidden, "guardrail blocked request: "+err.Error())
				return
			}
		}

		providerName := matched.Provider
		logger := rt.loggers[providerName]

		proxy := &httputil.ReverseProxy{
			Director: func(req *http.Request) {
				req.URL.Scheme = target.Scheme
				req.URL.Host = target.Host
				req.URL.Path = target.Path + trimmedPath
				req.Host = target.Host

				// Provider-specific auth
				setProviderAuth(req, providerName, matched.APIKey)
			},
			ModifyResponse: func(resp *http.Response) error {
				// Extract usage via logging handler
				if logger != nil && resp.StatusCode == http.StatusOK {
					body, err := io.ReadAll(resp.Body)
					if err == nil {
						prompt, completion := logger.ParseUsage(body)
						if prompt > 0 || completion > 0 {
							log.Printf("pass-through %s: prompt=%d completion=%d", providerName, prompt, completion)
						}
						// Restore body
						resp.Body = io.NopCloser(bytes.NewReader(body))
					}
				}

				// Post-call guardrail
				if rt.guardrail != nil && resp.StatusCode == http.StatusOK {
					body, err := io.ReadAll(resp.Body)
					if err == nil {
						_ = rt.guardrail.PostCall(nil, resp, body)
						resp.Body = io.NopCloser(bytes.NewReader(body))
					}
				}

				return nil
			},
			ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
				log.Printf("pass-through proxy error (%s): %v", providerName, err)
				writeError(w, http.StatusBadGateway, "upstream request failed")
			},
		}

		proxy.ServeHTTP(w, r)
	}
}

// setProviderAuth sets the appropriate auth header based on provider type.
func setProviderAuth(req *http.Request, providerName, apiKey string) {
	if apiKey == "" {
		return
	}
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
	case "vertex_ai", "gemini":
		req.Header.Set("Authorization", "Bearer "+apiKey)
	default:
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(`{"error":"` + msg + `"}`))
}
