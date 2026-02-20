package hook

import "fmt"

// hookConstructors maps hook names to their constructors.
var hookConstructors = map[string]func(config map[string]any) (Hook, error){}

// RegisterConstructor registers a hook constructor by name.
func RegisterConstructor(name string, fn func(config map[string]any) (Hook, error)) {
	hookConstructors[name] = fn
}

// Create creates a hook by name with the given config.
func Create(name string, config map[string]any) (Hook, error) {
	fn, ok := hookConstructors[name]
	if !ok {
		return nil, fmt.Errorf("unknown hook: %q", name)
	}
	return fn(config)
}

// RegistryFromConfig builds a hook registry from a config map.
// Each key is a hook name, each value is the hook's config.
func RegistryFromConfig(configs map[string]map[string]any) (*Registry, error) {
	r := NewRegistry()
	for name, cfg := range configs {
		h, err := Create(name, cfg)
		if err != nil {
			return nil, fmt.Errorf("hook %q: %w", name, err)
		}
		r.Register(h)
	}
	return r, nil
}
