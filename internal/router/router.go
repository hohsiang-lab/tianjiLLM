package router

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/praxisllmlab/tianjiLLM/internal/model"
	"github.com/praxisllmlab/tianjiLLM/internal/provider"
	"github.com/praxisllmlab/tianjiLLM/internal/proxy/middleware"
	"github.com/praxisllmlab/tianjiLLM/internal/wildcard"
)

// ErrNoDeployments is returned when no deployments exist for a requested model.
var ErrNoDeployments = errors.New("no deployments")

// ErrAccessDenied is returned when deployments exist but none are accessible to the caller.
var ErrAccessDenied = errors.New("access denied")

// Strategy selects a deployment from a list of healthy deployments.
type Strategy interface {
	Pick(deployments []*Deployment) *Deployment
}

// TagPicker extends Strategy with tag-based filtering.
type TagPicker interface {
	Strategy
	PickWithTags(deployments []*Deployment, tags []string, matchAny bool) *Deployment
}

// AutoRouterFunc is the function signature for semantic auto-routing.
type AutoRouterFunc func(ctx context.Context, lastUserMessage string) (string, error)

// Router routes requests across multiple deployments with fallback.
type Router struct {
	mu          sync.RWMutex
	deployments map[string][]*Deployment // modelName → deployments
	strategy    Strategy
	settings    RouterSettings
	autoRouters map[string]AutoRouterFunc // prefix → router
}

// RouterSettings configures router behavior.
type RouterSettings struct {
	AllowedFails int
	CooldownTime time.Duration
	NumRetries   int

	// ContextWindowFallbacks maps model names to fallback models with larger
	// context windows. When a request exceeds a model's context window,
	// the router retries with the fallback model.
	// Example: {"gpt-3.5-turbo": ["gpt-4-turbo", "gpt-4o"]}
	ContextWindowFallbacks map[string][]string

	// ModelGroupAlias maps alias names to actual model groups.
	// Resolved before deployment lookup in Route().
	ModelGroupAlias map[string]ModelGroupAliasItem

	// Fallbacks maps model names to ordered fallback model lists.
	// Example: {"gpt-4": ["claude-3", "gemini-pro"]}
	Fallbacks map[string][]string

	// DefaultFallbacks is the fallback list when no model-specific fallback exists.
	DefaultFallbacks []string

	// ContentPolicyFallbacks maps model names to fallback models for HTTP 400 content policy errors.
	ContentPolicyFallbacks map[string][]string

	// ModelGroupRetryPolicy provides per-model-group retry/timeout config.
	ModelGroupRetryPolicy map[string]RetryPolicy

	// EnableTagFiltering enables tag-based deployment filtering.
	EnableTagFiltering bool

	// TagFilteringMatchAny uses OR logic for tag matching when true (AND when false).
	TagFilteringMatchAny bool
}

// ModelGroupAliasItem maps an alias to a target model group.
// Matches Python's RouterModelGroupAliasItem.
type ModelGroupAliasItem struct {
	Model  string // target model group name
	Hidden bool   // if true, alias is not returned in /v1/models list
}

// RetryPolicy configures per-model-group retry behavior.
type RetryPolicy struct {
	NumRetries        int
	TimeoutSeconds    int
	RetryAfterSeconds int
}

// New creates a Router from model configs and strategy.
func New(models []config.ModelConfig, strategy Strategy, settings RouterSettings) *Router {
	if settings.AllowedFails == 0 {
		settings.AllowedFails = 3
	}
	if settings.CooldownTime == 0 {
		settings.CooldownTime = 60 * time.Second
	}
	if settings.NumRetries == 0 {
		settings.NumRetries = 2
	}

	deployments := make(map[string][]*Deployment)
	for i := range models {
		m := &models[i]
		providerName, modelName := provider.ParseModelName(m.TianjiParams.Model)

		d := &Deployment{
			ID:           fmt.Sprintf("%s-%d", m.ModelName, i),
			ProviderName: providerName,
			ModelName:    modelName,
			Region:       m.TianjiParams.Region,
			Config:       m,
			allowedFails: settings.AllowedFails,
			cooldownTime: settings.CooldownTime,
		}

		deployments[m.ModelName] = append(deployments[m.ModelName], d)
	}

	return &Router{
		deployments: deployments,
		strategy:    strategy,
		settings:    settings,
	}
}

// RegisterAutoRouter registers a semantic auto-router for a model prefix.
func (r *Router) RegisterAutoRouter(prefix string, fn AutoRouterFunc) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.autoRouters == nil {
		r.autoRouters = make(map[string]AutoRouterFunc)
	}
	r.autoRouters[prefix] = fn
}

// Route picks a healthy deployment for the given model and calls the provider.
// On failure, it tries fallback deployments up to NumRetries times.
func (r *Router) Route(ctx context.Context, modelName string, req *model.ChatCompletionRequest) (*Deployment, provider.Provider, error) {
	// Check for auto_router/ prefix — resolve via semantic routing
	if strings.HasPrefix(modelName, "auto_router/") {
		r.mu.RLock()
		arFunc := r.autoRouters[modelName]
		r.mu.RUnlock()
		if arFunc != nil && req != nil {
			lastMsg := extractLastUserMessage(req)
			if resolved, err := arFunc(ctx, lastMsg); err == nil && resolved != "" {
				modelName = resolved
			}
		}
	}

	// Resolve model group alias before deployment lookup
	if alias, ok := r.settings.ModelGroupAlias[modelName]; ok {
		modelName = alias.Model
	}

	r.mu.RLock()
	allDeployments := r.deployments[modelName]
	r.mu.RUnlock()

	if len(allDeployments) == 0 {
		allDeployments = r.wildcardMatch(modelName)
	}
	if len(allDeployments) == 0 {
		return nil, nil, fmt.Errorf("%w for model %q", ErrNoDeployments, modelName)
	}

	// Filter by access control before health check.
	allDeployments = r.filterByAccessControl(ctx, allDeployments)
	if len(allDeployments) == 0 {
		return nil, nil, fmt.Errorf("%w for model %q", ErrAccessDenied, modelName)
	}

	healthy := r.healthyDeployments(allDeployments)
	if len(healthy) == 0 {
		// All in cooldown — try them anyway as last resort
		healthy = allDeployments
	}

	// Use per-group retry policy if configured, else global
	numRetries := r.settings.NumRetries
	if groupPolicy, ok := r.settings.ModelGroupRetryPolicy[modelName]; ok && groupPolicy.NumRetries > 0 {
		numRetries = groupPolicy.NumRetries
	}

	tried := make(map[string]bool)
	for attempt := 0; attempt <= numRetries; attempt++ {
		available := filterUntried(healthy, tried)
		if len(available) == 0 {
			break
		}

		var d *Deployment
		if r.settings.EnableTagFiltering {
			if tp, ok := r.strategy.(TagPicker); ok {
				tags := extractTags(req)
				d = tp.PickWithTags(available, tags, r.settings.TagFilteringMatchAny)
			} else {
				d = r.strategy.Pick(available)
			}
		} else {
			d = r.strategy.Pick(available)
		}
		if d == nil {
			break
		}
		tried[d.ID] = true

		apiBase := ""
		if d.Config.TianjiParams.APIBase != nil {
			apiBase = *d.Config.TianjiParams.APIBase
		}
		p, err := provider.GetWithBaseURL(d.ProviderName, apiBase)
		if err != nil {
			d.RecordFailure()
			continue
		}

		return d, p, nil
	}

	return nil, nil, fmt.Errorf("all deployments failed for model %q", modelName)
}

// ContextWindowFallback returns the fallback model deployments when a request
// exceeds the current model's context window. Returns nil if no fallbacks configured.
func (r *Router) ContextWindowFallback(modelName string) (*Deployment, provider.Provider, error) {
	fallbacks := r.settings.ContextWindowFallbacks
	if len(fallbacks) == 0 {
		return nil, nil, fmt.Errorf("no context window fallbacks configured for %q", modelName)
	}

	models, ok := fallbacks[modelName]
	if !ok || len(models) == 0 {
		return nil, nil, fmt.Errorf("no context window fallbacks configured for %q", modelName)
	}

	for _, fallbackModel := range models {
		d, p, err := r.Route(context.Background(), fallbackModel, nil)
		if err == nil {
			return d, p, nil
		}
	}

	return nil, nil, fmt.Errorf("all context window fallbacks exhausted for %q", modelName)
}

// RecordSuccess records a successful call on a deployment.
func (r *Router) RecordSuccess(d *Deployment, latency time.Duration) {
	d.RecordSuccess(latency)
}

// RecordFailure records a failed call on a deployment.
func (r *Router) RecordFailure(d *Deployment) {
	d.RecordFailure()
}

// ModelGroupAlias returns the configured model group alias map.
func (r *Router) ModelGroupAlias() map[string]ModelGroupAliasItem {
	return r.settings.ModelGroupAlias
}

// GetDeployments returns all deployments for a model (for testing).
func (r *Router) GetDeployments(modelName string) []*Deployment {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.deployments[modelName]
}

// ListModelGroups returns model group names and their deployments, filtered by access control.
// The caller's identity is extracted from ctx (same as Route). Master key callers see all.
func (r *Router) ListModelGroups(ctx context.Context) map[string][]*Deployment {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make(map[string][]*Deployment, len(r.deployments))
	for k, v := range r.deployments {
		filtered := r.filterByAccessControl(ctx, v)
		if len(filtered) > 0 {
			result[k] = filtered
		}
	}
	return result
}

// filterByAccessControl removes deployments the caller is not authorized to use.
// Master key callers bypass all access control checks.
func (r *Router) filterByAccessControl(ctx context.Context, deployments []*Deployment) []*Deployment {
	isMaster, _ := ctx.Value(middleware.ContextKeyIsMasterKey).(bool)
	if isMaster {
		return deployments
	}

	orgID, _ := ctx.Value(middleware.ContextKeyOrgID).(string)
	teamID, _ := ctx.Value(middleware.ContextKeyTeamID).(string)
	tokenHash, _ := ctx.Value(middleware.ContextKeyTokenHash).(string)

	filtered := make([]*Deployment, 0, len(deployments))
	for _, d := range deployments {
		if d.Config.AccessControl.IsAllowed(orgID, teamID, tokenHash) {
			filtered = append(filtered, d)
		}
	}
	return filtered
}

func (r *Router) healthyDeployments(all []*Deployment) []*Deployment {
	healthy := make([]*Deployment, 0, len(all))
	for _, d := range all {
		if d.IsHealthy() {
			healthy = append(healthy, d)
		}
	}
	return healthy
}

func filterUntried(deployments []*Deployment, tried map[string]bool) []*Deployment {
	result := make([]*Deployment, 0, len(deployments))
	for _, d := range deployments {
		if !tried[d.ID] {
			result = append(result, d)
		}
	}
	return result
}

// extractLastUserMessage returns the last user message content for auto-routing.
func extractLastUserMessage(req *model.ChatCompletionRequest) string {
	for i := len(req.Messages) - 1; i >= 0; i-- {
		if req.Messages[i].Role == "user" {
			if s, ok := req.Messages[i].Content.(string); ok {
				return s
			}
		}
	}
	return ""
}

// wildcardMatch finds the best wildcard-pattern deployment group for modelName.
// It clones each matched deployment with the resolved model name so callers
// get correct provider/model routing.
func (r *Router) wildcardMatch(modelName string) []*Deployment {
	r.mu.RLock()
	defer r.mu.RUnlock()

	type candidate struct {
		pattern  string
		deps     []*Deployment
		captured []string
		length   int
		wcCount  int
	}
	var candidates []candidate
	for pat, deps := range r.deployments {
		if !strings.Contains(pat, "*") {
			continue
		}
		segs := wildcard.Match(pat, modelName)
		if segs == nil {
			continue
		}
		l, wc := wildcard.Specificity(pat)
		candidates = append(candidates, candidate{pat, deps, segs, l, wc})
	}
	if len(candidates) == 0 {
		return nil
	}

	sort.Slice(candidates, func(a, b int) bool {
		if candidates[a].length != candidates[b].length {
			return candidates[a].length > candidates[b].length
		}
		return candidates[a].wcCount < candidates[b].wcCount
	})

	best := candidates[0]
	cloned := make([]*Deployment, len(best.deps))
	for i, d := range best.deps {
		resolved := wildcard.ResolveModel(d.Config.TianjiParams.Model, best.captured)
		_, resolvedModelName := provider.ParseModelName(resolved)
		cloned[i] = &Deployment{
			ID:           d.ID,
			ProviderName: d.ProviderName,
			ModelName:    resolvedModelName,
			Region:       d.Region,
			Config:       d.Config,
			allowedFails: d.allowedFails,
			cooldownTime: d.cooldownTime,
		}
	}
	return cloned
}

// extractTags pulls tags from request metadata.
func extractTags(req *model.ChatCompletionRequest) []string {
	if req == nil || req.Metadata == nil {
		return nil
	}
	tagsRaw, ok := req.Metadata["tags"]
	if !ok {
		return nil
	}
	arr, ok := tagsRaw.([]any)
	if !ok {
		return nil
	}
	tags := make([]string, 0, len(arr))
	for _, v := range arr {
		if s, ok := v.(string); ok {
			tags = append(tags, s)
		}
	}
	return tags
}

// RouterProvider is the interface that handlers depend on.
// *Router satisfies this interface.
type RouterProvider interface {
	Route(ctx context.Context, modelName string, req *model.ChatCompletionRequest) (*Deployment, provider.Provider, error)
	GeneralFallback(modelName string) (*Deployment, provider.Provider, error)
	ListModelGroups(ctx context.Context) map[string][]*Deployment
	ModelGroupAlias() map[string]ModelGroupAliasItem
}
