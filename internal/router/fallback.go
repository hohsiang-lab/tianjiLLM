package router

import (
	"context"
	"fmt"

	"github.com/praxisllmlab/tianjiLLM/internal/provider"
)

// GeneralFallback returns the first available fallback deployment for the given model.
// Checks model-specific fallbacks first, then default fallbacks.
func (r *Router) GeneralFallback(modelName string) (*Deployment, provider.Provider, error) {
	// Try model-specific fallbacks first
	if fallbacks, ok := r.settings.Fallbacks[modelName]; ok {
		for _, fb := range fallbacks {
			d, p, err := r.Route(context.Background(), fb, nil)
			if err == nil {
				return d, p, nil
			}
		}
	}

	// Try default fallbacks
	for _, fb := range r.settings.DefaultFallbacks {
		if fb == modelName {
			continue // skip self
		}
		d, p, err := r.Route(context.Background(), fb, nil)
		if err == nil {
			return d, p, nil
		}
	}

	return nil, nil, fmt.Errorf("all fallbacks exhausted for model %q", modelName)
}

// ContentPolicyFallback returns the first available fallback for content policy errors (HTTP 400).
func (r *Router) ContentPolicyFallback(modelName string) (*Deployment, provider.Provider, error) {
	fallbacks := r.settings.ContentPolicyFallbacks
	if len(fallbacks) == 0 {
		return nil, nil, fmt.Errorf("no content policy fallbacks configured for %q", modelName)
	}

	models, ok := fallbacks[modelName]
	if !ok || len(models) == 0 {
		return nil, nil, fmt.Errorf("no content policy fallbacks configured for %q", modelName)
	}

	for _, fb := range models {
		d, p, err := r.Route(context.Background(), fb, nil)
		if err == nil {
			return d, p, nil
		}
	}

	return nil, nil, fmt.Errorf("all content policy fallbacks exhausted for %q", modelName)
}
