package search

import (
	"fmt"
	"sort"
	"sync"
)

var (
	mu       sync.RWMutex
	registry = make(map[string]SearchProvider)
)

// Register adds a search provider to the global registry.
// Called from init() in each provider implementation file.
func Register(name string, p SearchProvider) {
	mu.Lock()
	defer mu.Unlock()
	registry[name] = p
}

// Get returns the search provider for the given name, or an error if not found.
func Get(name string) (SearchProvider, error) {
	mu.RLock()
	defer mu.RUnlock()
	p, ok := registry[name]
	if !ok {
		return nil, fmt.Errorf("search provider not found: %s", name)
	}
	return p, nil
}

// List returns sorted names of all registered search providers.
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
