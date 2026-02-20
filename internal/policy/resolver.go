package policy

import (
	"fmt"

	"github.com/praxisllmlab/tianjiLLM/internal/db"
)

// maxChainDepth limits the depth of policy inheritance to prevent
// runaway chains. Any chain deeper than this is treated as a cycle.
const maxChainDepth = 50

// ResolvedPolicy holds a policy's effective guardrails after walking
// the inheritance chain from child → root.
type ResolvedPolicy struct {
	Name             string
	GuardrailsAdd    []string
	GuardrailsRemove []string
	Pipeline         []byte
}

// ResolveChain walks the inheritance chain from the given policy
// rows (child first, root last) and merges guardrails.
// Parent guardrails apply first; child guardrails override (add/remove).
// Cycle detection: the chain rows come from a recursive CTE; we
// additionally check for duplicates by name.
func ResolveChain(chain []db.GetPolicyChainRow) (ResolvedPolicy, error) {
	if len(chain) == 0 {
		return ResolvedPolicy{}, nil
	}

	if len(chain) > maxChainDepth {
		return ResolvedPolicy{}, fmt.Errorf("policy chain exceeds max depth %d", maxChainDepth)
	}

	// Check for cycles: no duplicate names in the chain.
	seen := make(map[string]struct{}, len(chain))
	for _, row := range chain {
		if _, dup := seen[row.Name]; dup {
			return ResolvedPolicy{}, fmt.Errorf("cycle detected: policy %q appears twice in chain", row.Name)
		}
		seen[row.Name] = struct{}{}
	}

	// Walk from root (last element) to child (first element).
	// Root's guardrails form the base; each child adds/removes.
	addSet := make(map[string]struct{})
	for i := len(chain) - 1; i >= 0; i-- {
		row := chain[i]
		for _, g := range row.GuardrailsAdd {
			addSet[g] = struct{}{}
		}
		for _, g := range row.GuardrailsRemove {
			delete(addSet, g)
		}
	}

	guardrails := make([]string, 0, len(addSet))
	for g := range addSet {
		guardrails = append(guardrails, g)
	}

	return ResolvedPolicy{
		Name:             chain[0].Name,
		GuardrailsAdd:    guardrails,
		GuardrailsRemove: chain[0].GuardrailsRemove,
		Pipeline:         chain[0].Pipeline,
	}, nil
}
