package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/praxisllmlab/tianjiLLM/internal/model"
	"github.com/praxisllmlab/tianjiLLM/internal/provider"
)

// assistantsProxy creates a reverse proxy for Assistants API endpoints.
// It resolves the upstream from the same model route policy as other OpenAI endpoints.
func (h *Handlers) assistantsProxy(w http.ResponseWriter, r *http.Request) {
	modelName, err := requestModelForOpenAIProxy(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "read request body: " + err.Error(), Type: "invalid_request_error"},
		})
		return
	}

	var route openAIEndpointRoute
	if modelName != "" {
		route, err = h.resolveOpenAIEndpointRouteForModel(r.Context(), modelName, false)
	} else {
		route, err = h.resolveOpenAIEndpointFallbackRoute(r.Context(), false)
	}
	if err != nil {
		writeOpenAIEndpointRouteError(w, err)
		return
	}
	if route.Transport == openAISubscriptionTransportChatGPTCodexBackend {
		writeUnsupportedChatGPTCodexEndpoint(w, "assistants")
		return
	}
	if route.Upstream == "" {
		writeUnsupportedChatGPTCodexEndpoint(w, "assistants")
		return
	}
	if route.ModelName != "" {
		if err := rewriteResponsesRequestModel(r, route.ModelName); err != nil {
			writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
				Error: model.ErrorDetail{Message: err.Error(), Type: "invalid_request_error"},
			})
			return
		}
	}
	h.proxyOpenAIUpstream(w, r, route.Upstream, route.APIKey, true)
}

func (h *Handlers) openAIEndpointProxy(w http.ResponseWriter, r *http.Request) {
	route, err := h.resolveOpenAIEndpointRouteForRequest(r)
	if err != nil {
		writeOpenAIEndpointRouteError(w, err)
		return
	}
	if route.Transport == openAISubscriptionTransportChatGPTCodexBackend {
		if strings.TrimPrefix(r.URL.Path, "/v1") == "/images/variations" {
			writeUnsupportedChatGPTCodexEndpoint(w, "image variations")
		} else {
			writeUnsupportedChatGPTCodexEndpoint(w, "this endpoint")
		}
		return
	}
	if len(route.Candidates) > 0 {
		h.proxyOpenAIUpstreamWithSubscriptionCandidates(w, r, route.Upstream, route.Candidates)
		return
	}
	h.proxyOpenAIUpstream(w, r, route.Upstream, route.APIKey, false)
}

type openAIEndpointRoute struct {
	Upstream     string
	APIKey       string
	ModelName    string
	TianjiParams config.TianjiParams
	Candidates   []resolvedOpenAISubscriptionCredential
	Transport    openAISubscriptionTransport
}

func (h *Handlers) proxyOpenAIUpstream(w http.ResponseWriter, r *http.Request, upstream string, apiKey string, assistantBeta bool) {
	if upstream == "" {
		message := "OpenAI endpoint not configured"
		if assistantBeta {
			message = "assistants API not configured"
		}
		writeJSON(w, http.StatusNotImplemented, model.ErrorResponse{
			Error: model.ErrorDetail{Message: message, Type: "not_supported"},
		})
		return
	}

	target, err := url.Parse(upstream)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "invalid upstream URL", Type: "internal_error"},
		})
		return
	}

	proxy := &httputil.ReverseProxy{
		Rewrite: func(proxyReq *httputil.ProxyRequest) {
			req := proxyReq.Out
			req.URL.Scheme = target.Scheme
			req.URL.Host = target.Host
			req.Host = target.Host
			rewriteOpenAIProxyPath(req, target)
			clearInboundProviderAuthHeaders(req)
			if apiKey != "" {
				req.Header.Set("Authorization", "Bearer "+apiKey)
			}
			if assistantBeta {
				req.Header.Set("OpenAI-Beta", "assistants=v2")
			} else {
				req.Header.Del("OpenAI-Beta")
			}
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			log.Printf("OpenAI upstream proxy error: %v", err)
			http.Error(w, `{"error":"upstream request failed"}`, http.StatusBadGateway)
		},
	}

	proxy.ServeHTTP(w, r)
}

func (h *Handlers) proxyOpenAIUpstreamWithSubscriptionCandidates(
	w http.ResponseWriter,
	r *http.Request,
	upstream string,
	candidates []resolvedOpenAISubscriptionCredential,
) {
	if upstream == "" {
		writeJSON(w, http.StatusNotImplemented, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "OpenAI endpoint not configured", Type: "not_supported"},
		})
		return
	}

	target, err := url.Parse(upstream)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "invalid upstream URL", Type: "internal_error"},
		})
		return
	}

	proxy := &httputil.ReverseProxy{
		Rewrite: func(proxyReq *httputil.ProxyRequest) {
			req := proxyReq.Out
			req.URL.Scheme = target.Scheme
			req.URL.Host = target.Host
			req.Host = target.Host
			rewriteOpenAIProxyPath(req, target)
			clearInboundProviderAuthHeaders(req)
			req.Header.Del("OpenAI-Beta")
		},
		Transport: openAISubscriptionFailoverTransport{
			base:       http.DefaultTransport,
			candidates: candidates,
			refresh:    h.forceRefreshOpenAISubscriptionCredential,
			authFailed: h.recordOpenAISubscriptionAuthFailureAfterRefresh,
			record: func(credentialID string, headers http.Header) {
				if state, ok := parseOpenAIRateLimitHeaders(headers, credentialID, h.openAISubscriptionNowUTC()); ok {
					h.recordOpenAISubscriptionRateLimit(credentialID, state)
				}
			},
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			log.Printf("OpenAI upstream proxy error: %v", err)
			http.Error(w, `{"error":"upstream request failed"}`, http.StatusBadGateway)
		},
	}

	proxy.ServeHTTP(w, r)
}

func rewriteOpenAIProxyPath(req *http.Request, target *url.URL) {
	if req == nil || req.URL == nil || target == nil || target.Path == "" {
		return
	}

	requestPath := req.URL.Path
	switch {
	case requestPath == "/v1":
		requestPath = ""
	case strings.HasPrefix(requestPath, "/v1/"):
		requestPath = strings.TrimPrefix(requestPath, "/v1")
	}

	targetPath := strings.TrimRight(target.Path, "/")
	if targetPath == "" {
		return
	}
	if requestPath == "" {
		req.URL.Path = targetPath
	} else {
		req.URL.Path = targetPath + "/" + strings.TrimLeft(requestPath, "/")
	}
	req.URL.RawPath = ""
}

type openAISubscriptionFailoverTransport struct {
	base       http.RoundTripper
	candidates []resolvedOpenAISubscriptionCredential
	refresh    func(context.Context, string) (resolvedOpenAISubscriptionCredential, error)
	authFailed func(context.Context, string, string) error
	record     func(string, http.Header)
}

func (t openAISubscriptionFailoverTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if len(t.candidates) == 0 {
		return t.base.RoundTrip(req)
	}
	var body []byte
	if req.Body != nil {
		var err error
		body, err = io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		_ = req.Body.Close()
	}

	var lastErr error
	for i, candidate := range t.candidates {
		attempt := req.Clone(req.Context())
		attempt.Header = req.Header.Clone()
		clearInboundProviderAuthHeaders(attempt)
		attempt.Header.Set("Authorization", "Bearer "+candidate.BearerToken)
		attempt.Body = io.NopCloser(bytes.NewReader(body))
		attempt.GetBody = func() (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader(body)), nil
		}
		attempt.ContentLength = int64(len(body))

		resp, err := t.base.RoundTrip(attempt)
		if err != nil {
			lastErr = err
			if i+1 < len(t.candidates) {
				continue
			}
			return nil, err
		}
		if t.record != nil {
			t.record(candidate.CredentialID, resp.Header)
		}
		if resp.StatusCode == http.StatusUnauthorized && t.refresh != nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()

			refreshed, refreshErr := t.refresh(req.Context(), candidate.CredentialID)
			if refreshErr != nil {
				lastErr = refreshErr
				if i+1 < len(t.candidates) {
					continue
				}
				return openAISubscriptionReauthorizationRequiredResponse(attempt), nil
			}

			retry := req.Clone(req.Context())
			retry.Header = req.Header.Clone()
			clearInboundProviderAuthHeaders(retry)
			retry.Header.Set("Authorization", "Bearer "+refreshed.BearerToken)
			retry.Body = io.NopCloser(bytes.NewReader(body))
			retry.GetBody = func() (io.ReadCloser, error) {
				return io.NopCloser(bytes.NewReader(body)), nil
			}
			retry.ContentLength = int64(len(body))

			retryResp, retryErr := t.base.RoundTrip(retry)
			if retryErr != nil {
				lastErr = retryErr
				if i+1 < len(t.candidates) {
					continue
				}
				return nil, retryErr
			}
			if t.record != nil {
				t.record(candidate.CredentialID, retryResp.Header)
			}
			if retryResp.StatusCode == http.StatusUnauthorized && i+1 < len(t.candidates) {
				if t.authFailed != nil {
					lastErr = t.authFailed(req.Context(), candidate.CredentialID, "upstream returned 401 after forced refresh")
				}
				_, _ = io.Copy(io.Discard, retryResp.Body)
				_ = retryResp.Body.Close()
				continue
			}
			if retryResp.StatusCode == http.StatusUnauthorized && t.authFailed != nil {
				_ = t.authFailed(req.Context(), candidate.CredentialID, "upstream returned 401 after forced refresh")
				_, _ = io.Copy(io.Discard, retryResp.Body)
				_ = retryResp.Body.Close()
				return openAISubscriptionReauthorizationRequiredResponse(retry), nil
			}
			if openAISubscriptionRetryableStatus(retryResp.StatusCode) && i+1 < len(t.candidates) {
				_, _ = io.Copy(io.Discard, retryResp.Body)
				_ = retryResp.Body.Close()
				continue
			}
			return retryResp, nil
		}
		if openAISubscriptionRetryableStatus(resp.StatusCode) && i+1 < len(t.candidates) {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			continue
		}
		return resp, nil
	}
	return nil, lastErr
}

func (h *Handlers) resolveOpenAIEndpointRouteForRequest(r *http.Request) (openAIEndpointRoute, error) {
	return h.resolveOpenAIEndpointRouteForRequestWithOpenRouter(r, false)
}

func (h *Handlers) resolveOpenAIEndpointRouteForResponses(r *http.Request) (openAIEndpointRoute, error) {
	return h.resolveOpenAIEndpointRouteForRequestWithPolicy(r, true, true)
}

func (h *Handlers) resolveOpenAIEndpointRouteForCompactResponses(r *http.Request) (openAIEndpointRoute, error) {
	return h.resolveOpenAIEndpointRouteForRequestWithPolicy(r, false, true)
}

func (h *Handlers) resolveOpenAIEndpointRouteForRequestWithOpenRouter(r *http.Request, allowOpenRouter bool) (openAIEndpointRoute, error) {
	return h.resolveOpenAIEndpointRouteForRequestWithPolicy(r, allowOpenRouter, false)
}

func (h *Handlers) resolveOpenAIEndpointRouteForRequestWithPolicy(r *http.Request, allowOpenRouter, rejectUnknownModel bool) (openAIEndpointRoute, error) {
	ctx := context.Background()
	if r != nil {
		ctx = r.Context()
		modelName, err := requestModelForOpenAIProxy(r)
		if err != nil {
			return openAIEndpointRoute{}, fmt.Errorf("read request body: %w", err)
		}
		if modelName != "" {
			if rejectUnknownModel {
				return h.resolveOpenAIEndpointRouteForModelStrict(ctx, modelName, allowOpenRouter)
			}
			return h.resolveOpenAIEndpointRouteForModel(ctx, modelName, allowOpenRouter)
		}
	}

	return h.resolveOpenAIEndpointFallbackRoute(ctx, allowOpenRouter)
}

func (h *Handlers) resolveOpenAIEndpointRouteForModel(ctx context.Context, modelName string, allowOpenRouter bool) (openAIEndpointRoute, error) {
	return h.resolveOpenAIEndpointRouteForModelWithUnknownPolicy(ctx, modelName, allowOpenRouter, false)
}

func (h *Handlers) resolveOpenAIEndpointRouteForModelStrict(ctx context.Context, modelName string, allowOpenRouter bool) (openAIEndpointRoute, error) {
	return h.resolveOpenAIEndpointRouteForModelWithUnknownPolicy(ctx, modelName, allowOpenRouter, true)
}

func (h *Handlers) resolveOpenAIEndpointRouteForModelWithUnknownPolicy(ctx context.Context, modelName string, allowOpenRouter, rejectUnknownModel bool) (openAIEndpointRoute, error) {
	cfg, resolvedFullModel := h.findModelConfig(modelName)
	if cfg == nil {
		if rejectUnknownModel {
			return openAIEndpointRoute{}, model.ModelNotFound(modelName)
		}
		return h.resolveOpenAIEndpointFallbackRoute(ctx, allowOpenRouter)
	}

	providerName, resolvedModel := provider.ParseModelName(resolvedFullModel)
	routeParams := normalizeOfficialOpenAIParams(cfg.TianjiParams)
	if providerName != "openai" && !supportsOpenAICompatibleProvider(providerName, routeParams) {
		return openAIEndpointRoute{}, nil
	}
	if providerName == "openrouter" && !allowOpenRouter {
		return openAIEndpointRoute{}, nil
	}
	auth, err := h.resolveOpenAISubscriptionRouteForResolvedModel(ctx, resolvedFullModel, routeParams)
	if err != nil {
		return openAIEndpointRoute{}, err
	}
	if providerName == "openai" {
		return openAIEndpointRoute{
			ModelName:    resolvedModel,
			TianjiParams: routeParams,
			Candidates:   auth.Candidates,
			Transport:    openAISubscriptionTransportChatGPTCodexBackend,
		}, nil
	}
	apiBase := ""
	modelNameForUpstream := resolvedModel
	if routeParams.APIBase != nil {
		apiBase = *routeParams.APIBase
	}

	baseURL := "https://api.openai.com/v1"
	if apiBase != "" {
		baseURL = apiBase
	} else if providerName == "openrouter" {
		var ok bool
		baseURL, ok = defaultOpenAICompatibleProviderBaseURL(providerName, resolvedModel)
		if !ok {
			return openAIEndpointRoute{}, nil
		}
	} else if providerName != "" && providerName != "openai" {
		var ok bool
		baseURL, ok = defaultOpenAICompatibleProviderBaseURL(providerName, resolvedModel)
		if !ok {
			return openAIEndpointRoute{}, nil
		}
	}
	if providerName == "openrouter" && allowOpenRouter {
		modelNameForUpstream = resolvedModel
	}

	return openAIEndpointRoute{Upstream: baseURL, ModelName: modelNameForUpstream, APIKey: auth.APIKey, Candidates: auth.Candidates, Transport: openAISubscriptionTransportForParams(routeParams), TianjiParams: routeParams}, nil
}

func (h *Handlers) resolveOpenAIEndpointFallbackRoute(ctx context.Context, allowOpenRouter bool) (openAIEndpointRoute, error) {
	models := h.runtimeModelList(ctx)
	hasOfficialModel := false
	for _, m := range models {
		modelName := strings.TrimSpace(m.TianjiParams.Model)
		if modelName == "" {
			continue
		}
		providerName, providerModel := provider.ParseModelName(modelName)
		if providerName == "openai" {
			hasOfficialModel = true
		}
		if providerName == "openrouter" && !allowOpenRouter {
			continue
		}

		params := normalizeOfficialOpenAIParams(m.TianjiParams)
		if providerName != "openai" && !supportsOpenAICompatibleProvider(providerName, params) {
			continue
		}
		auth, err := h.resolveOpenAISubscriptionRouteForResolvedModel(ctx, modelName, params)
		if err != nil {
			if providerName == "openai" {
				return openAIEndpointRoute{}, err
			}
			continue
		}
		if providerName == "openai" {
			return openAIEndpointRoute{
				ModelName:    providerModel,
				TianjiParams: params,
				Candidates:   auth.Candidates,
				Transport:    openAISubscriptionTransportChatGPTCodexBackend,
			}, nil
		}

		baseURL := ""
		if params.APIBase != nil {
			baseURL = strings.TrimSpace(*params.APIBase)
		}
		if baseURL == "" {
			var ok bool
			baseURL, ok = defaultOpenAICompatibleProviderBaseURL(providerName, providerModel)
			if !ok {
				continue
			}
		}
		return openAIEndpointRoute{
			Upstream:     baseURL,
			APIKey:       auth.APIKey,
			ModelName:    providerModel,
			TianjiParams: params,
			Candidates:   auth.Candidates,
			Transport:    openAISubscriptionTransportForParams(params),
		}, nil
	}

	if !hasOfficialModel && h.Config.AssistantSettings != nil {
		return openAIEndpointRoute{
			Upstream: h.Config.AssistantSettings.APIBase,
			APIKey:   h.Config.AssistantSettings.APIKey,
		}, nil
	}
	return openAIEndpointRoute{}, nil
}

func defaultOpenAICompatibleProviderBaseURL(providerName, modelName string) (string, bool) {
	p, err := provider.Get(providerName)
	if err != nil {
		return "", false
	}
	requestURL := p.GetRequestURL(modelName)
	baseURL := strings.TrimSuffix(requestURL, "/chat/completions")
	if baseURL == "" || baseURL == requestURL {
		return "", false
	}
	return baseURL, true
}

const maxOpenAIRequestBodyBytes = 32 << 20

func readOpenAIRequestBody(r *http.Request) ([]byte, error) {
	if r == nil || r.Body == nil {
		return nil, nil
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, maxOpenAIRequestBodyBytes+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxOpenAIRequestBodyBytes {
		return nil, fmt.Errorf("request body exceeds %d-byte limit", maxOpenAIRequestBodyBytes)
	}
	return body, nil
}

func requestModelForOpenAIProxy(r *http.Request) (string, error) {
	if r == nil {
		return "", nil
	}
	queryModel := r.URL.Query().Get("model")
	if r.Body == nil {
		return queryModel, nil
	}

	body, err := readOpenAIRequestBody(r)
	if err != nil {
		return "", err
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	if queryModel != "" {
		return queryModel, nil
	}

	contentType := r.Header.Get("Content-Type")
	mediaType, params, _ := mime.ParseMediaType(contentType)
	switch mediaType {
	case "application/x-www-form-urlencoded":
		values, err := url.ParseQuery(string(body))
		if err == nil {
			return values.Get("model"), nil
		}
	case "multipart/form-data":
		if boundary := params["boundary"]; boundary != "" {
			return multipartModelValue(body, boundary), nil
		}
	}

	var partial struct {
		Model string `json:"model"`
	}
	if json.Unmarshal(body, &partial) != nil {
		return "", nil
	}
	return partial.Model, nil
}

func multipartModelValue(body []byte, boundary string) string {
	form, err := multipart.NewReader(bytes.NewReader(body), boundary).ReadForm(32 << 20)
	if err != nil {
		return ""
	}
	defer func() { _ = form.RemoveAll() }()
	if values := form.Value["model"]; len(values) > 0 {
		return values[0]
	}
	return ""
}

// AssistantCreate handles POST /v1/assistants.
func (h *Handlers) AssistantCreate(w http.ResponseWriter, r *http.Request) {
	h.assistantsProxy(w, r)
}

// AssistantGet handles GET /v1/assistants/{assistant_id}.
func (h *Handlers) AssistantGet(w http.ResponseWriter, r *http.Request) {
	h.assistantsProxy(w, r)
}

// AssistantList handles GET /v1/assistants.
func (h *Handlers) AssistantList(w http.ResponseWriter, r *http.Request) {
	h.assistantsProxy(w, r)
}

// AssistantModify handles POST /v1/assistants/{assistant_id}.
func (h *Handlers) AssistantModify(w http.ResponseWriter, r *http.Request) {
	h.assistantsProxy(w, r)
}

// AssistantDelete handles DELETE /v1/assistants/{assistant_id}.
func (h *Handlers) AssistantDelete(w http.ResponseWriter, r *http.Request) {
	h.assistantsProxy(w, r)
}

// ThreadCreate handles POST /v1/threads.
func (h *Handlers) ThreadCreate(w http.ResponseWriter, r *http.Request) {
	h.assistantsProxy(w, r)
}

// ThreadGet handles GET /v1/threads/{thread_id}.
func (h *Handlers) ThreadGet(w http.ResponseWriter, r *http.Request) {
	h.assistantsProxy(w, r)
}

// ThreadModify handles POST /v1/threads/{thread_id}.
func (h *Handlers) ThreadModify(w http.ResponseWriter, r *http.Request) {
	h.assistantsProxy(w, r)
}

// ThreadDelete handles DELETE /v1/threads/{thread_id}.
func (h *Handlers) ThreadDelete(w http.ResponseWriter, r *http.Request) {
	h.assistantsProxy(w, r)
}

// MessageCreate handles POST /v1/threads/{thread_id}/messages.
func (h *Handlers) MessageCreate(w http.ResponseWriter, r *http.Request) {
	h.assistantsProxy(w, r)
}

// MessageList handles GET /v1/threads/{thread_id}/messages.
func (h *Handlers) MessageList(w http.ResponseWriter, r *http.Request) {
	h.assistantsProxy(w, r)
}

// MessageGet handles GET /v1/threads/{thread_id}/messages/{message_id}.
func (h *Handlers) MessageGet(w http.ResponseWriter, r *http.Request) {
	h.assistantsProxy(w, r)
}

// RunCreate handles POST /v1/threads/{thread_id}/runs.
// Supports stream=true via SSE passthrough.
func (h *Handlers) RunCreate(w http.ResponseWriter, r *http.Request) {
	h.assistantsProxy(w, r)
}

// RunGet handles GET /v1/threads/{thread_id}/runs/{run_id}.
func (h *Handlers) RunGet(w http.ResponseWriter, r *http.Request) {
	h.assistantsProxy(w, r)
}

// RunList handles GET /v1/threads/{thread_id}/runs.
func (h *Handlers) RunList(w http.ResponseWriter, r *http.Request) {
	h.assistantsProxy(w, r)
}

// RunCancel handles POST /v1/threads/{thread_id}/runs/{run_id}/cancel.
func (h *Handlers) RunCancel(w http.ResponseWriter, r *http.Request) {
	h.assistantsProxy(w, r)
}

// RunStepsList handles GET /v1/threads/{thread_id}/runs/{run_id}/steps.
func (h *Handlers) RunStepsList(w http.ResponseWriter, r *http.Request) {
	h.assistantsProxy(w, r)
}

// RunStepGet handles GET /v1/threads/{thread_id}/runs/{run_id}/steps/{step_id}.
func (h *Handlers) RunStepGet(w http.ResponseWriter, r *http.Request) {
	h.assistantsProxy(w, r)
}
