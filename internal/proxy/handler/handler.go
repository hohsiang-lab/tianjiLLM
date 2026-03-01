package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"

	"github.com/praxisllmlab/tianjiLLM/internal/a2a"
	"github.com/praxisllmlab/tianjiLLM/internal/cache"
	"github.com/praxisllmlab/tianjiLLM/internal/callback"
	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/praxisllmlab/tianjiLLM/internal/db"
	"github.com/praxisllmlab/tianjiLLM/internal/guardrail"
	"github.com/praxisllmlab/tianjiLLM/internal/policy"
	"github.com/praxisllmlab/tianjiLLM/internal/provider"
	"github.com/praxisllmlab/tianjiLLM/internal/proxy/hook"
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
	Config           *config.ProxyConfig
	DB               db.Store
	Cache            cache.Cache
	Router           router.RouterProvider
	Callbacks        *callback.Registry
	Guardrails       *guardrail.Registry
	PolicyEngine     *router.PolicyEngine
	PolicyEng        *policy.Engine
	SSOHandler       *SSOHandler
	RealtimeRelay    http.Handler
	TokenCounter     *token.Counter
	AgentRegistry    *a2a.AgentRegistry
	CompletionBridge *a2a.CompletionBridge
	EventDispatcher  *hook.ManagementEventDispatcher
	DiscordAlerter   *callback.DiscordRateLimitAlerter
}

func (h *Handlers) ListModels(w http.ResponseWriter, r *http.Request) {
	// Build set of hidden model aliases to filter out
	hiddenAliases := make(map[string]bool)
	if h.Router != nil {
		for alias, item := range h.Router.ModelGroupAlias() {
			if item.Hidden {
				hiddenAliases[alias] = true
			}
		}
	}

	models := make([]map[string]any, 0, len(h.Config.ModelList))
	for _, m := range h.Config.ModelList {
		if hiddenAliases[m.ModelName] {
			continue
		}
		models = append(models, map[string]any{
			"id":       m.ModelName,
			"object":   "model",
			"owned_by": "tianji",
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"object": "list",
		"data":   models,
	})
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
	modelCfg, resolvedFullModel := h.findModelConfig(modelName)
	if modelCfg == nil {
		return nil, "", "", fmt.Errorf("model %q not found in config", modelName)
	}

	providerName, resolvedModel := provider.ParseModelName(resolvedFullModel)

	apiBase := ""
	if modelCfg.TianjiParams.APIBase != nil {
		apiBase = *modelCfg.TianjiParams.APIBase
	}

	p, err := provider.GetWithBaseURL(providerName, apiBase)
	if err != nil {
		return nil, "", "", err
	}

	apiKey := ""
	if modelCfg.TianjiParams.APIKey != nil {
		apiKey = *modelCfg.TianjiParams.APIKey
	}

	return p, apiKey, resolvedModel, nil
}

// findModelConfig looks up a model by exact name, then by wildcard pattern.
// Returns the matched config and the fully-resolved model string (with wildcards
// replaced by captured segments from the request model name).
func (h *Handlers) findModelConfig(modelName string) (*config.ModelConfig, string) {
	// Exact match first
	for i := range h.Config.ModelList {
		if h.Config.ModelList[i].ModelName == modelName {
			return &h.Config.ModelList[i], h.Config.ModelList[i].TianjiParams.Model
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
	for i := range h.Config.ModelList {
		pattern := h.Config.ModelList[i].ModelName
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
	cfg := &h.Config.ModelList[best.index]
	resolved := wildcard.ResolveModel(cfg.TianjiParams.Model, best.captured)
	return cfg, resolved
}

// decodeJSON decodes the request body as JSON into v.
func decodeJSON(r *http.Request, v any) error {
	return json.NewDecoder(r.Body).Decode(v)
}

// mustReadAll reads all bytes from a reader, ignoring errors.
func mustReadAll(r io.Reader) []byte {
	data, _ := io.ReadAll(r)
	return data
}
