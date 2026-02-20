package policy

import (
	"context"
	"fmt"
	"sync"

	"github.com/praxisllmlab/tianjiLLM/internal/db"
)

// Engine holds all policies and attachments in memory, providing
// fast evaluation against incoming requests. Data is loaded from DB
// and can be refreshed atomically via Update().
type Engine struct {
	mu          sync.RWMutex
	policies    map[string]db.PolicyTable
	attachments []db.PolicyAttachmentTable
	db          *db.Queries
}

// NewEngine creates a policy engine backed by the given DB queries.
func NewEngine(database *db.Queries) *Engine {
	return &Engine{
		db:       database,
		policies: make(map[string]db.PolicyTable),
	}
}

// Load reads all policies and attachments from DB into memory.
// Call this at startup and from the scheduler hot-reload job.
func (e *Engine) Load(ctx context.Context) error {
	policies, err := e.db.ListPolicies(ctx)
	if err != nil {
		return err
	}

	attachments, err := e.db.ListPolicyAttachments(ctx)
	if err != nil {
		return err
	}

	policyMap := make(map[string]db.PolicyTable, len(policies))
	for _, p := range policies {
		policyMap[p.Name] = p
	}

	e.mu.Lock()
	e.policies = policyMap
	e.attachments = attachments
	e.mu.Unlock()

	return nil
}

// EvaluateResult holds the resolved guardrails for a matched request.
type EvaluateResult struct {
	Guardrails []string
	Policies   []string // names of matched policies
}

// Evaluate finds all matching policies for the request and resolves
// their guardrails through inheritance chains.
func (e *Engine) Evaluate(ctx context.Context, req MatchRequest) (EvaluateResult, error) {
	e.mu.RLock()
	attachments := e.attachments
	e.mu.RUnlock()

	matched := MatchAttachments(attachments, req)
	if len(matched) == 0 {
		return EvaluateResult{}, nil
	}

	// Collect unique policy names
	policyNames := make(map[string]struct{})
	for _, a := range matched {
		policyNames[a.PolicyName] = struct{}{}
	}

	// Resolve each policy's inheritance chain
	allGuardrails := make(map[string]struct{})
	var matchedPolicyNames []string

	for name := range policyNames {
		chain, err := e.db.GetPolicyChain(ctx, name)
		if err != nil {
			return EvaluateResult{}, fmt.Errorf("policy %q chain lookup: %w", name, err)
		}

		resolved, err := ResolveChain(chain)
		if err != nil {
			return EvaluateResult{}, fmt.Errorf("policy %q chain resolve: %w", name, err)
		}

		matchedPolicyNames = append(matchedPolicyNames, name)
		for _, g := range resolved.GuardrailsAdd {
			allGuardrails[g] = struct{}{}
		}
	}

	guardrails := make([]string, 0, len(allGuardrails))
	for g := range allGuardrails {
		guardrails = append(guardrails, g)
	}

	return EvaluateResult{
		Guardrails: guardrails,
		Policies:   matchedPolicyNames,
	}, nil
}
