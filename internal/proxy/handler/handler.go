package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"golang.org/x/sync/singleflight"

	"github.com/praxisllmlab/tianjiLLM/internal/a2a"
	"github.com/praxisllmlab/tianjiLLM/internal/cache"
	"github.com/praxisllmlab/tianjiLLM/internal/callback"
	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/praxisllmlab/tianjiLLM/internal/db"
	"github.com/praxisllmlab/tianjiLLM/internal/guardrail"
	"github.com/praxisllmlab/tianjiLLM/internal/model"
	"github.com/praxisllmlab/tianjiLLM/internal/provider"
	"github.com/praxisllmlab/tianjiLLM/internal/provider/chatgptcodex"
	"github.com/praxisllmlab/tianjiLLM/internal/proxy/hook"
	"github.com/praxisllmlab/tianjiLLM/internal/proxy/middleware"
	"github.com/praxisllmlab/tianjiLLM/internal/router"
	"github.com/praxisllmlab/tianjiLLM/internal/token"
	"github.com/praxisllmlab/tianjiLLM/internal/wildcard"
)

// DBPinger is the interface for database health checks.
type DBPinger interface {
	Ping(ctx context.Context) error
}

// Handlers holds all HTTP handler dependencies.
type Handlers struct {
	Config                     *config.ProxyConfig
	DB                         db.Store
	Cache                      cache.Cache
	Router                     *router.Router
	RuntimeModels              *RuntimeModelSource
	Callbacks                  *callback.Registry
	Guardrails                 *guardrail.Registry
	PolicyEngine               *router.PolicyEngine
	SSOHandler                 *SSOHandler
	RealtimeRelay              http.Handler
	OpenAIOAuthHTTPClient      *http.Client
	OpenAIUpstreamHTTPClient   *http.Client
	OpenAIUpstreamBaseURL      string
	openAISubscriptionNow      func() time.Time
	openAIRefreshGroup         singleflight.Group
	codexUsageRefreshGroup     singleflight.Group
	codexUsageAsyncGroup       singleflight.Group
	TokenCounter               *token.Counter
	AgentRegistry              *a2a.AgentRegistry
	CompletionBridge           *a2a.CompletionBridge
	EventDispatcher            *hook.ManagementEventDispatcher
	DiscordAlerter             *callback.DiscordRateLimitAlerter
	RateLimitStore             callback.RateLimitStore
	RateLimitDB                callback.RateLimitDB // for DB fallback when memory entry is absent
	DisabledTokens             callback.DisabledTokenStore
	CodexUsageFetcher          chatgptcodex.UsageFetcher
	CodexResetConsumer         chatgptcodex.ResetCreditConsumer
	CodexUsageCache            *OpenAISubscriptionCodexUsageCache
	CodexCatalogFetcher        chatgptcodex.CatalogFetcher
	CodexCatalogCache          *OpenAISubscriptionCodexCatalogCache
	Capabilities               model.CapabilityMatrix
	codexCatalogCacheMu        sync.Mutex
	codexUsageCacheMu          sync.Mutex
	CodexCatalogTTL            time.Duration
	CodexCatalogRefreshTimeout time.Duration
	CodexUsageTTL              time.Duration
	CodexUsageBackoffBase      time.Duration
	CodexUsageBackoffJitter    func(time.Duration) time.Duration

	// roundRobin holds per-provider atomic counters for round-robin upstream selection.
	// Kept on the Handlers instance (not package-level) so that each Handlers has its
	// own independent counter — prevents test interference and supports multiple instances.
	roundRobinMu       sync.RWMutex
	roundRobinCounters map[string]*atomic.Uint64
	runtimeModelsMu    sync.Mutex

	// stickyState holds per-provider "current token" for sticky strategy.
	// Re-evaluates when the token's 5h window resets (detected by comparing
	// the stored reset timestamp against the current one from the rate-limit
	// store). This prevents draining a single token's 7d quota across many
	// 5h cycles without ever considering better candidates.
	stickyMu    sync.Mutex
	stickyState map[string]stickyEntry // trackKey ("provider:all" or "provider:sonnet") → current selection
	stickyCore  stickyStrategyStore

	openAISubscriptionRoutingMu sync.Mutex
	openAISubscriptionRR        map[string]uint64
	openAISubscriptionSticky    map[string]string
}

func (h *Handlers) ListModels(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.buildListModelsResponse(r.Context(), h.visibleModels(r.Context())))
}

func (h *Handlers) RetrieveModel(w http.ResponseWriter, r *http.Request) {
	modelName := chi.URLParam(r, "model")
	for _, configured := range h.visibleModels(r.Context()) {
		if configured.ModelName == modelName {
			writeJSON(w, http.StatusOK, openAIModelListItemFromConfig(configured))
			return
		}
	}
	writeRequestError(w, model.ModelNotFound(modelName))
}

func (h *Handlers) visibleModels(ctx context.Context) []config.ModelConfig {
	if ctx == nil {
		ctx = context.Background()
	}
	// Build set of hidden model aliases to filter out
	hiddenAliases := make(map[string]bool)
	if h.Router != nil {
		for alias, item := range h.Router.ModelGroupAlias() {
			if item.Hidden {
				hiddenAliases[alias] = true
			}
		}
	}

	modelList := h.runtimeModelList(ctx)
	models := make([]config.ModelConfig, 0, len(modelList))
	for _, m := range modelList {
		if hiddenAliases[m.ModelName] {
			continue
		}
		models = append(models, m)
	}
	return models
}

func (h *Handlers) KeyGenerate(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusNotImplemented, map[string]string{
		"error": "not implemented yet",
	})
}

// resolveProviderFromConfig looks up a model config and returns the provider,
// api key, and resolved model name. It handles api_base fallback to
// OpenAI-compatible providers, matching Python LiteLLM's behavior.
func (h *Handlers) resolveProviderFromConfig(modelName string) (provider.Provider, string, string, error) {
	return h.resolveProviderFromConfigWithContext(context.Background(), modelName)
}

type resolvedProviderRoute struct {
	Provider               provider.Provider
	APIKey                 string
	PublicModel            string
	ModelName              string
	Backend                model.Backend
	Capability             model.CapabilityRecord
	TianjiParams           config.TianjiParams
	SubscriptionCandidates []resolvedOpenAISubscriptionCredential
	ChatGPTCodexBackend    bool
}

type providerRouteResolutionOptions struct {
	skipCredentialResolutionForUnsupportedCodexChat bool
}

func (h *Handlers) requestUsesOfficialOpenAIModel(modelName string) bool {
	if strings.TrimSpace(modelName) == "" {
		return false
	}
	modelCfg, resolved := h.findModelConfig(modelName)
	if modelCfg == nil || resolved == "" {
		return false
	}
	providerName, _ := provider.ParseModelName(resolved)
	return providerName == "openai"
}

func writeUnsupportedChatGPTCodexEndpoint(w http.ResponseWriter, endpoint string) {
	writeJSON(w, http.StatusNotImplemented, model.ErrorResponse{
		Error: model.ErrorDetail{
			Message: fmt.Sprintf("OpenAI ChatGPT Codex backend does not support %s", endpoint),
			Type:    "not_supported",
		},
	})
}

func (h *Handlers) resolveProviderFromConfigRouteWithContext(ctx context.Context, modelName string, options ...providerRouteResolutionOptions) (resolvedProviderRoute, error) {
	modelCfg, resolvedFullModel := h.findModelConfig(modelName)
	if modelCfg == nil {
		return resolvedProviderRoute{}, fmt.Errorf("model %q not found in config", modelName)
	}

	providerName, resolvedModel := provider.ParseModelName(resolvedFullModel)
	routeParams := normalizeOfficialOpenAIParams(modelCfg.TianjiParams)

	apiBase := ""
	if routeParams.APIBase != nil {
		apiBase = *routeParams.APIBase
	}

	p, err := provider.GetWithBaseURL(providerName, apiBase)
	if err != nil {
		return resolvedProviderRoute{}, err
	}

	auth, err := h.resolveOpenAISubscriptionRouteForResolvedModel(ctx, resolvedFullModel, routeParams, options...)
	if err != nil {
		return resolvedProviderRoute{}, err
	}

	route := resolvedProviderRoute{
		Provider:               p,
		APIKey:                 auth.APIKey,
		PublicModel:            modelCfg.ModelName,
		ModelName:              resolvedModel,
		Backend:                model.BackendDirectOpenAIHTTP,
		TianjiParams:           routeParams,
		SubscriptionCandidates: auth.Candidates,
		ChatGPTCodexBackend:    isChatGPTCodexBackendTransport(routeParams),
	}
	if route.ChatGPTCodexBackend {
		route.Backend = model.BackendChatGPTCodex
	}
	return h.attachRouteCapabilities(route), nil
}

func (h *Handlers) resolveProviderFromConfigWithContext(ctx context.Context, modelName string) (provider.Provider, string, string, error) {
	route, err := h.resolveProviderFromConfigRouteWithContext(ctx, modelName)
	if err != nil {
		return nil, "", "", err
	}
	return route.Provider, route.APIKey, route.ModelName, nil
}

// findModelConfig looks up a model by exact name, then by wildcard pattern.
// Returns the matched config and the fully-resolved model string (with wildcards
// replaced by captured segments from the request model name).
func (h *Handlers) findModelConfig(modelName string) (*config.ModelConfig, string) {
	modelList := h.runtimeModelList(context.Background())
	// Exact match first
	for i := range modelList {
		if modelList[i].ModelName == modelName {
			return &modelList[i], modelList[i].TianjiParams.Model
		}
	}

	// Collect wildcard candidates and sort by specificity
	type candidate struct {
		index    int
		captured []string
		length   int
		wcCount  int
	}
	var candidates []candidate
	for i := range modelList {
		pattern := modelList[i].ModelName
		if !strings.Contains(pattern, "*") {
			continue
		}
		segs := wildcard.Match(pattern, modelName)
		if segs == nil {
			continue
		}
		l, wc := wildcard.Specificity(pattern)
		candidates = append(candidates, candidate{i, segs, l, wc})
	}
	if len(candidates) == 0 {
		return nil, ""
	}

	// Most specific first: longest pattern wins; ties broken by fewer wildcards
	sort.Slice(candidates, func(a, b int) bool {
		if candidates[a].length != candidates[b].length {
			return candidates[a].length > candidates[b].length
		}
		return candidates[a].wcCount < candidates[b].wcCount
	})

	best := candidates[0]
	cfg := &modelList[best.index]
	resolved := wildcard.ResolveModel(cfg.TianjiParams.Model, best.captured)
	return cfg, resolved
}

// decodeJSON decodes the request body as JSON into v.
func decodeJSON(r *http.Request, v any) error {
	return json.NewDecoder(r.Body).Decode(v)
}

// copyResponseBody reads the entire upstream response body, copies headers, and
// writes the status + body to the client. Returns the body bytes (for callback
// use) or writes a 502 and returns nil on read failure.
func copyResponseBody(w http.ResponseWriter, resp *http.Response) []byte {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("error: failed to read upstream response body: %v", err)
		writeJSON(w, http.StatusBadGateway, model.ErrorResponse{
			Error: model.ErrorDetail{
				Message: "failed to read upstream response",
				Type:    "internal_error",
			},
		})
		return nil
	}
	for k, vv := range resp.Header {
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	_, _ = w.Write(body)
	return body
}

// lookupProviderName returns the provider name for a given model alias.
// It looks up the model config and parses the provider prefix from TianjiParams.Model.
// Returns "openai" if no provider prefix is found.
func (h *Handlers) lookupProviderName(modelName string) string {
	cfg, fullModel := h.findModelConfig(modelName)
	if cfg == nil {
		return "unknown"
	}
	name, _ := provider.ParseModelName(fullModel)
	if name == "" {
		return "openai"
	}
	return name
}

// buildBaseLogData extracts auth context fields for spend logging.
// All handlers should call this instead of manually extracting context values.
func buildBaseLogData(ctx context.Context, startTime time.Time) callback.LogData {
	var data callback.LogData
	data.StartTime = startTime
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
	if upToken, ok := ctx.Value(middleware.ContextKeyUpstreamToken).(string); ok {
		data.UpstreamTokenKey = upToken
	}
	return data
}

// errorLogParamsFromContext extracts auth/identity context values shared by all
// error-log call sites. Callers fill in the remaining fields (Model, Provider,
// StatusCode, ErrorType, ErrorMessage) before passing to InsertErrorLog.
func errorLogParamsFromContext(ctx context.Context) db.InsertErrorLogParams {
	var params db.InsertErrorLogParams
	params.RequestID = chiMiddleware.GetReqID(ctx)
	if v, ok := ctx.Value(middleware.ContextKeyTokenHash).(string); ok {
		params.ApiKeyHash = v
	}
	if tid, ok := ctx.Value(middleware.ContextKeyTeamID).(string); ok && tid != "" {
		params.TeamID = &tid
	}
	params.UpstreamTokenKey, _ = ctx.Value(middleware.ContextKeyUpstreamToken).(string)
	params.EndUser, _ = ctx.Value(middleware.ContextKeyUserID).(string)
	params.OrganizationID, _ = ctx.Value(middleware.ContextKeyOrgID).(string)
	return params
}
