package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
)

// DBPinger is the interface for database health checks.
type DBPinger interface {
	Ping(ctx context.Context) error
}

// Handlers holds all HTTP handler dependencies.
type Handlers struct {
	Config           *config.ProxyConfig
	DB               *db.Queries
	Cache            cache.Cache
	Router           *router.Router
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
	modelCfg := h.findModelConfig(modelName)
	if modelCfg == nil {
		return nil, "", "", fmt.Errorf("model %q not found in config", modelName)
	}

	providerName, resolvedModel := provider.ParseModelName(modelCfg.TianjiParams.Model)

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

// findModelConfig looks up a model by exact name or wildcard match.
func (h *Handlers) findModelConfig(modelName string) *config.ModelConfig {
	// Exact match first
	for i := range h.Config.ModelList {
		if h.Config.ModelList[i].ModelName == modelName {
			return &h.Config.ModelList[i]
		}
	}

	// Wildcard match: "openai/*" matches "openai/gpt-4o-mini"
	for i := range h.Config.ModelList {
		pattern := h.Config.ModelList[i].ModelName
		if strings.HasSuffix(pattern, "/*") {
			prefix := strings.TrimSuffix(pattern, "*")
			if strings.HasPrefix(modelName, prefix) {
				return &h.Config.ModelList[i]
			}
		}
	}

	return nil
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
