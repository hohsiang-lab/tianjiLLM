package router

import (
	"context"
	"fmt"
	"time"

	"github.com/praxisllmlab/tianjiLLM/internal/provider"
)

// fallbackDelay is the pause between fallback attempts to avoid rapid-fire requests to providers.
const fallbackDelay = 500 * time.Millisecond

// GeneralFallback returns the first available fallback deployment for the given model.
// Checks model-specific fallbacks first, then default fallbacks.
func (r *Router) GeneralFallback(ctx context.Context, modelName string) (*Deployment, provider.Provider, error) {
	// Try model-specific fallbacks first
	if fallbacks, ok := r.settings.Fallbacks[modelName]; ok {
		for i, fb := range fallbacks {
			if i > 0 {
				select {
				case <-time.After(fallbackDelay):
				case <-ctx.Done():
					return nil, nil, ctx.Err()
				}
			}
			d, p, err := r.Route(ctx, fb, nil)
			if err == nil {
				return d, p, nil
			}
		}
	}

	// Try default fallbacks
	first := true
	for _, fb := range r.settings.DefaultFallbacks {
		if fb == modelName {
			continue // skip self
		}
		if !first {
			select {
			case <-time.After(fallbackDelay):
			case <-ctx.Done():
				return nil, nil, ctx.Err()
			}
		}
		first = false
		d, p, err := r.Route(ctx, fb, nil)
		if err == nil {
			return d, p, nil
		}
	}

	return nil, nil, fmt.Errorf("all fallbacks exhausted for model %q", modelName)
}

// ContentPolicyFallback returns the first available fallback for content policy errors (HTTP 400).
func (r *Router) ContentPolicyFallback(ctx context.Context, modelName string) (*Deployment, provider.Provider, error) {
	fallbacks := r.settings.ContentPolicyFallbacks
	if len(fallbacks) == 0 {
		return nil, nil, fmt.Errorf("no content policy fallbacks configured for %q", modelName)
	}

	models, ok := fallbacks[modelName]
	if !ok || len(models) == 0 {
		return nil, nil, fmt.Errorf("no content policy fallbacks configured for %q", modelName)
	}

	for i, fb := range models {
		if i > 0 {
			select {
			case <-time.After(fallbackDelay):
			case <-ctx.Done():
				return nil, nil, ctx.Err()
			}
		}
		d, p, err := r.Route(ctx, fb, nil)
		if err == nil {
			return d, p, nil
		}
	}

	return nil, nil, fmt.Errorf("all content policy fallbacks exhausted for %q", modelName)
}
