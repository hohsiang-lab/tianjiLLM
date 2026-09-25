package provider

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

var (
	mu             sync.RWMutex
	registry       = make(map[string]Provider)
	baseURLFactory func(baseURL string) Provider
)

// Register adds a provider to the global registry.
// Typically called from provider package init() functions.
func Register(name string, p Provider) {
	mu.Lock()
	defer mu.Unlock()
	registry[name] = p
}

// RegisterBaseURLFactory sets the factory used to create OpenAI-compatible
// providers for unknown provider names that have an api_base configured.
// Called once during startup by the openai package.
func RegisterBaseURLFactory(f func(baseURL string) Provider) {
	mu.Lock()
	defer mu.Unlock()
	baseURLFactory = f
}

// Get returns a provider by name. Returns an error if not found.
func Get(name string) (Provider, error) {
	mu.RLock()
	defer mu.RUnlock()
	p, ok := registry[name]
	if !ok {
		return nil, fmt.Errorf("provider %q not registered", name)
	}
	return p, nil
}

// BaseURLConfigurable is an optional interface that providers can implement
// to support being cloned with a custom base URL. GetWithBaseURL will use this
// interface when available, preserving provider-specific logic (e.g. Jina's
// encoding_format normalization) while overriding the upstream endpoint.
type BaseURLConfigurable interface {
	WithBaseURL(baseURL string) Provider
}

// GetWithBaseURL returns a provider by name. When apiBase is set:
//  1. If the registered provider implements BaseURLConfigurable, clone it
//     with the new base URL — this preserves provider-specific logic.
//  2. Otherwise fall back to the OpenAI-compatible baseURLFactory.
//
// This matches Python LiteLLM's behavior where providers like jina_ai keep
// their validation logic regardless of a custom api_base.
func GetWithBaseURL(name, apiBase string) (Provider, error) {
	mu.RLock()
	registered, registeredOK := registry[name]
	factory := baseURLFactory
	mu.RUnlock()

	if apiBase != "" {
		// Prefer cloning the registered provider so its custom logic is retained.
		if registeredOK {
			if configurable, ok := registered.(BaseURLConfigurable); ok {
				return configurable.WithBaseURL(apiBase), nil
			}
		}
		// Fall back to the generic OpenAI-compatible factory for unknown providers.
		if factory != nil {
			return factory(apiBase), nil
		}
	}
	if registeredOK {
		return registered, nil
	}
	return nil, fmt.Errorf("provider %q not registered", name)
}

// List returns all registered provider names in sorted order.
func List() []string {
	mu.RLock()
	defer mu.RUnlock()
	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// ParseModelName splits a "provider/model" string into provider and model parts.
// Examples:
//
//	"openai/gpt-4o" → ("openai", "gpt-4o")
//	"anthropic/claude-sonnet-4-5-20250929" → ("anthropic", "claude-sonnet-4-5-20250929")
//	"gpt-4o" → ("openai", "gpt-4o")  // default to openai
func ParseModelName(fullModel string) (providerName, modelName string) {
	parts := strings.SplitN(fullModel, "/", 2)
	if len(parts) == 1 {
		return "openai", parts[0]
	}
	return parts[0], parts[1]
}
