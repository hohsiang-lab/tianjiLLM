package passthrough

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/praxisllmlab/tianjiLLM/internal/callback"
	"github.com/praxisllmlab/tianjiLLM/internal/pricing"
	"github.com/praxisllmlab/tianjiLLM/internal/provider/anthropic"
	"github.com/praxisllmlab/tianjiLLM/internal/proxy/middleware"
)

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
	callbacks *callback.Registry
}

// NewRouter creates a new pass-through router.
func NewRouter(endpoints []Endpoint, guardrail GuardrailHook, callbacks *callback.Registry) *Router {
	return &Router{
		endpoints: endpoints,
		loggers:   make(map[string]LoggingHandler),
		guardrail: guardrail,
		callbacks: callbacks,
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
		if strings.TrimSpace(matched.APIKey) == "" {
			writeError(w, http.StatusUnauthorized, "provider credential not configured")
			return
		}

		target, err := url.Parse(matched.Target)
		if err != nil {
			http.Error(w, `{"error":"invalid upstream URL"}`, http.StatusInternalServerError)
			return
		}

		// Pre-call guardrail
		if rt.guardrail != nil {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				log.Printf("pass-through %s: failed to read request body for guardrail: %v", matched.Provider, err)
				writeError(w, http.StatusBadRequest, "failed to read request body")
				return
			}
			r.Body = io.NopCloser(bytes.NewReader(body))
			if err := rt.guardrail.PreCall(r, body); err != nil {
				writeError(w, http.StatusForbidden, "guardrail blocked request: "+err.Error())
				return
			}
		}

		providerName := matched.Provider
		logger := rt.loggers[providerName]
		startTime := time.Now()
		ctx := r.Context()
		requestModel := extractPassthroughRequestModel(r)

		proxy := &httputil.ReverseProxy{
			Rewrite: func(proxyReq *httputil.ProxyRequest) {
				req := proxyReq.Out
				req.URL.Scheme = target.Scheme
				req.URL.Host = target.Host
				req.URL.Path = target.Path + trimmedPath
				req.Host = target.Host

				// Provider-specific auth; setProviderAuth scrubs caller aliases first.
				setProviderAuth(req, providerName, matched.APIKey)
			},
			ModifyResponse: func(resp *http.Response) error {
				if resp.StatusCode != http.StatusOK {
					return nil
				}

				streaming := strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream")

				if streaming {
					// Wrap body: tee all bytes while ReverseProxy streams to client.
					// On Close, parse usage and call spend callback.
					sr := &streamingReader{
						src:          resp.Body,
						logger:       logger,
						guardrail:    rt.guardrail,
						provider:     providerName,
						resp:         resp,
						ctx:          ctx,
						startTime:    startTime,
						requestModel: requestModel,
						callbacks:    rt.callbacks,
					}
					resp.Body = readCloserOnly{sr}
					return nil
				}

				// Non-streaming: buffer body for usage parsing + guardrail + spend callback.
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					log.Printf("pass-through %s: failed to read response body: %v", providerName, err)
					return fmt.Errorf("read response body: %w", err)
				}
				resp.Body = io.NopCloser(bytes.NewReader(body))

				if logger != nil && rt.callbacks != nil {
					prompt, completion, cacheRead, cacheCreation, modelName := logger.ParseUsage(body)
					if prompt > 0 || completion > 0 {
						if modelName == "" {
							modelName = requestModel
						}
						logData := buildPassthroughLogData(
							ctx, providerName, modelName, startTime,
							prompt, completion, cacheRead, cacheCreation,
						)
						go func() {
							defer func() {
								if p := recover(); p != nil {
									log.Printf("pass-through %s: panic in LogSuccess callback: %v", providerName, p)
								}
							}()
							rt.callbacks.LogSuccess(logData)
						}()
					}
				}
				if rt.guardrail != nil {
					if gErr := rt.guardrail.PostCall(nil, resp, body); gErr != nil {
						log.Printf("pass-through %s: guardrail postcall error: %v", providerName, gErr)
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

// buildPassthroughLogData constructs a LogData from passthrough usage info.
// Mirrors buildNativeLogData in handler/native_format.go.
func buildPassthroughLogData(ctx context.Context, providerName, modelName string, startTime time.Time, prompt, completion, cacheRead, cacheCreation int) callback.LogData {
	endTime := time.Now()
	regularInput := prompt - cacheRead - cacheCreation
	if regularInput < 0 {
		regularInput = 0
	}
	tokenUsage := pricing.TokenUsage{
		PromptTokens:             regularInput,
		CompletionTokens:         completion,
		CacheReadInputTokens:     cacheRead,
		CacheCreationInputTokens: cacheCreation,
	}
	promptCost, completionCost := pricing.Default().Cost(modelName, tokenUsage)

	var data callback.LogData
	data.StartTime = startTime
	data.EndTime = endTime
	data.Latency = endTime.Sub(startTime)
	data.Model = modelName
	data.Provider = providerName
	data.PromptTokens = prompt
	data.CompletionTokens = completion
	data.TotalTokens = prompt + completion
	data.CacheReadInputTokens = cacheRead
	data.CacheCreationInputTokens = cacheCreation
	data.Cost = promptCost + completionCost

	if tokenHash, ok := ctx.Value(middleware.ContextKeyTokenHash).(string); ok {
		data.APIKey = tokenHash
	}
	if userID, ok := ctx.Value(middleware.ContextKeyUserID).(string); ok {
		data.UserID = userID
	}
	if teamID, ok := ctx.Value(middleware.ContextKeyTeamID).(string); ok {
		data.TeamID = teamID
	}
	if orgID, ok := ctx.Value(middleware.ContextKeyOrgID).(string); ok {
		data.OrganizationID = orgID
	}
	if ip, ok := ctx.Value(middleware.ContextKeyRequesterIP).(string); ok {
		data.RequesterIPAddress = ip
	}
	return data
}

// extractPassthroughRequestModel peeks the "model" field from the request body
// without consuming it.
func extractPassthroughRequestModel(r *http.Request) string {
	if r.Body == nil {
		return ""
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("pass-through: failed to peek request model from body: %v", err)
		return ""
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	var m struct {
		Model string `json:"model"`
	}
	if err := json.Unmarshal(body, &m); err != nil {
		log.Printf("pass-through: request body is not valid JSON, model field unavailable: %v", err)
		return ""
	}
	return m.Model
}

var inboundProviderAuthHeaders = []string{
	"Authorization",
	"api-key",
	"x-api-key",
	"x-goog-api-key",
	"Proxy-Authorization",
}

func clearInboundProviderAuthHeaders(req *http.Request) {
	if req == nil {
		return
	}
	for _, header := range inboundProviderAuthHeaders {
		req.Header.Del(header)
	}
}

// setProviderAuth sets the appropriate auth header based on provider type.
func setProviderAuth(req *http.Request, providerName, apiKey string) {
	clearInboundProviderAuthHeaders(req)
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
		req.Header.Set("anthropic-version", "2023-06-01")
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

// streamingReader wraps a streaming response body. It tees all bytes into a
// buffer while ReverseProxy streams them to the client. On Close, it parses
// usage and fires the spend callback.
type streamingReader struct {
	src          io.ReadCloser
	buf          bytes.Buffer
	logger       LoggingHandler
	guardrail    GuardrailHook
	provider     string
	resp         *http.Response
	ctx          context.Context
	startTime    time.Time
	requestModel string
	callbacks    *callback.Registry
}

func (r *streamingReader) Read(p []byte) (int, error) {
	n, err := r.src.Read(p)
	if n > 0 {
		r.buf.Write(p[:n])
	}
	return n, err
}

func (r *streamingReader) Close() error {
	err := r.src.Close()
	body := r.buf.Bytes()

	if r.logger != nil && r.callbacks != nil {
		prompt, completion, cacheRead, cacheCreation, modelName := r.logger.ParseSSEUsage(body)
		if prompt > 0 || completion > 0 {
			if modelName == "" {
				modelName = r.requestModel
			}
			logData := buildPassthroughLogData(
				r.ctx, r.provider, modelName, r.startTime,
				prompt, completion, cacheRead, cacheCreation,
			)
			go func() {
				defer func() {
					if p := recover(); p != nil {
						log.Printf("pass-through %s: panic in LogSuccess callback: %v", r.provider, p)
					}
				}()
				r.callbacks.LogSuccess(logData)
			}()
		}
	}
	if r.guardrail != nil {
		if gErr := r.guardrail.PostCall(nil, r.resp, body); gErr != nil {
			log.Printf("pass-through %s: streaming guardrail postcall error: %v", r.provider, gErr)
		}
	}

	return err
}

// readCloserOnly hides io.WriterTo to prevent io.Copy from bypassing
// our Read() via splice/sendfile optimization.
type readCloserOnly struct{ io.ReadCloser }
