package hook

import (
	"context"

	"github.com/praxisllmlab/tianjiLLM/internal/model"
)

// Hook defines the interface for pre-call and post-call hooks.
// Hooks run in the request pipeline before/after the LLM provider call.
type Hook interface {
	Name() string
	PreCall(ctx context.Context, req *model.ChatCompletionRequest) error
	PostCall(ctx context.Context, req *model.ChatCompletionRequest, resp *model.ModelResponse) error
}

// Registry manages registered hooks and runs them in order.
type Registry struct {
	hooks []Hook
}

// NewRegistry creates an empty hook registry.
func NewRegistry() *Registry {
	return &Registry{}
}

// Register adds a hook to the registry.
func (r *Registry) Register(h Hook) {
	r.hooks = append(r.hooks, h)
}

// RunPreCall runs all registered pre-call hooks in order.
// Returns the first error encountered (short-circuits).
func (r *Registry) RunPreCall(ctx context.Context, req *model.ChatCompletionRequest) error {
	for _, h := range r.hooks {
		if err := h.PreCall(ctx, req); err != nil {
			return err
		}
	}
	return nil
}

// RunPostCall runs all registered post-call hooks in order.
// Returns the first error encountered (short-circuits).
func (r *Registry) RunPostCall(ctx context.Context, req *model.ChatCompletionRequest, resp *model.ModelResponse) error {
	for _, h := range r.hooks {
		if err := h.PostCall(ctx, req, resp); err != nil {
			return err
		}
	}
	return nil
}

// Len returns the number of registered hooks.
func (r *Registry) Len() int {
	return len(r.hooks)
}
