package guardrail

import (
	"context"
	"fmt"
	"log"
	"slices"
	"sync"

	"github.com/praxisllmlab/tianjiLLM/internal/model"
)

// Hook specifies when the guardrail runs.
type Hook string

const (
	HookPreCall  Hook = "pre_call"
	HookPostCall Hook = "post_call"
)

// Result is returned by a guardrail check.
type Result struct {
	Passed  bool
	Message string
	// ModifiedRequest is set when the guardrail rewrites the request (e.g., PII redaction).
	ModifiedRequest *model.ChatCompletionRequest
}

// Guardrail is the interface for content safety checks.
type Guardrail interface {
	Name() string
	SupportedHooks() []Hook
	Run(ctx context.Context, hook Hook, req *model.ChatCompletionRequest, resp *model.ModelResponse) (Result, error)
}

// GuardrailWithPolicy pairs a Guardrail with its failure policy from config.
// The Guardrail interface is NOT modified — fail-open/fail-closed is a config concern.
type GuardrailWithPolicy struct {
	Guardrail Guardrail
	FailOpen  bool // true = continue on guardrail error; false = block on error
}

// Registry holds named guardrails and runs them for requests.
type Registry struct {
	mu         sync.RWMutex
	guardrails map[string]Guardrail
	policies   map[string]bool // name → failOpen
}

// NewRegistry creates a guardrail registry.
func NewRegistry() *Registry {
	return &Registry{
		guardrails: make(map[string]Guardrail),
		policies:   make(map[string]bool),
	}
}

// Register adds a guardrail to the registry with default fail-closed policy.
func (r *Registry) Register(g Guardrail) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.guardrails[g.Name()] = g
}

// RegisterWithPolicy adds a guardrail with an explicit failure policy.
func (r *Registry) RegisterWithPolicy(g Guardrail, failOpen bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.guardrails[g.Name()] = g
	r.policies[g.Name()] = failOpen
}

// Get returns a guardrail by name.
func (r *Registry) Get(name string) (Guardrail, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	g, ok := r.guardrails[name]
	return g, ok
}

// Names returns all registered guardrail names.
func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.guardrails))
	for name := range r.guardrails {
		names = append(names, name)
	}
	return names
}

// RunPreCall runs pre-call guardrails for the given names.
// Returns modified request if any guardrail rewrites it.
// Returns error if any guardrail blocks the request.
// Respects fail-open/fail-closed policy: on Run() error (not BlockedError),
// fail-open logs warning and continues, fail-closed returns the error.
func (r *Registry) RunPreCall(ctx context.Context, guardrailNames []string, req *model.ChatCompletionRequest) (*model.ChatCompletionRequest, error) {
	r.mu.RLock()
	type entry struct {
		g        Guardrail
		failOpen bool
	}
	guards := make([]entry, 0, len(guardrailNames))
	for _, name := range guardrailNames {
		if g, ok := r.guardrails[name]; ok {
			guards = append(guards, entry{g: g, failOpen: r.policies[name]})
		}
	}
	r.mu.RUnlock()

	current := req
	for _, e := range guards {
		if !slices.Contains(e.g.SupportedHooks(), HookPreCall) {
			continue
		}
		result, err := e.g.Run(ctx, HookPreCall, current, nil)
		if err != nil {
			if e.failOpen {
				log.Printf("guardrail %s failed (fail-open, continuing): %v", e.g.Name(), err)
				continue
			}
			return nil, fmt.Errorf("guardrail %s: %w", e.g.Name(), err)
		}
		if !result.Passed {
			return nil, &BlockedError{GuardrailName: e.g.Name(), Message: result.Message}
		}
		if result.ModifiedRequest != nil {
			current = result.ModifiedRequest
		}
	}
	return current, nil
}

// RunPostCall runs post-call guardrails on the response.
// Respects fail-open/fail-closed policy on Run() errors.
func (r *Registry) RunPostCall(ctx context.Context, guardrailNames []string, req *model.ChatCompletionRequest, resp *model.ModelResponse) error {
	r.mu.RLock()
	type entry struct {
		g        Guardrail
		failOpen bool
	}
	guards := make([]entry, 0, len(guardrailNames))
	for _, name := range guardrailNames {
		if g, ok := r.guardrails[name]; ok {
			guards = append(guards, entry{g: g, failOpen: r.policies[name]})
		}
	}
	r.mu.RUnlock()

	for _, e := range guards {
		if !slices.Contains(e.g.SupportedHooks(), HookPostCall) {
			continue
		}
		result, err := e.g.Run(ctx, HookPostCall, req, resp)
		if err != nil {
			if e.failOpen {
				log.Printf("guardrail %s failed (fail-open, continuing): %v", e.g.Name(), err)
				continue
			}
			return fmt.Errorf("guardrail %s: %w", e.g.Name(), err)
		}
		if !result.Passed {
			return &BlockedError{GuardrailName: e.g.Name(), Message: result.Message}
		}
	}
	return nil
}

// BlockedError indicates a guardrail blocked the request.
type BlockedError struct {
	GuardrailName string
	Message       string
}

func (e *BlockedError) Error() string {
	return fmt.Sprintf("request blocked by guardrail %q: %s", e.GuardrailName, e.Message)
}

// extractContent extracts the text content from a request or response
// based on the hook type. Handles Content being string or any.
func extractContent(hook Hook, req *model.ChatCompletionRequest, resp *model.ModelResponse) string {
	if hook == HookPreCall && req != nil && len(req.Messages) > 0 {
		last := req.Messages[len(req.Messages)-1]
		if s, ok := last.Content.(string); ok {
			return s
		}
	}
	if hook == HookPostCall && resp != nil && len(resp.Choices) > 0 && resp.Choices[0].Message != nil {
		if s, ok := resp.Choices[0].Message.Content.(string); ok {
			return s
		}
	}
	return ""
}
