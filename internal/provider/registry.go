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

// GetWithBaseURL returns a provider by name. When apiBase is set,
// it creates a fresh OpenAI-compatible provider pointing at that URL
// instead of using the registered singleton — matching Python LiteLLM's
// behavior where lm_studio, ollama, vllm, etc. are all OpenAI-compatible
// providers distinguished only by their api_base.
func GetWithBaseURL(name, apiBase string) (Provider, error) {
	mu.RLock()
	factory := baseURLFactory
	mu.RUnlock()

	if apiBase != "" && factory != nil {
		return factory(apiBase), nil
	}
	return Get(name)
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
