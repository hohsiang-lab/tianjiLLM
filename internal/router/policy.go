package router

import (
	"path"
	"strings"
)

// Policy defines a routing policy that conditionally overrides routing strategy
// and guardrail bindings based on request metadata.
type Policy struct {
	Name       string          `yaml:"name" json:"name"`
	Conditions PolicyCondition `yaml:"conditions" json:"conditions"`

	// Overrides applied when conditions match
	RoutingStrategy *string  `yaml:"routing_strategy,omitempty" json:"routing_strategy,omitempty"`
	Guardrails      []string `yaml:"guardrails,omitempty" json:"guardrails,omitempty"`
	Priority        int      `yaml:"priority,omitempty" json:"priority,omitempty"`
}

// PolicyCondition specifies match conditions. All non-empty fields must match
// (AND logic). Each field supports wildcard patterns.
type PolicyCondition struct {
	TeamAlias *string  `yaml:"team_alias,omitempty" json:"team_alias,omitempty"`
	KeyAlias  *string  `yaml:"key_alias,omitempty" json:"key_alias,omitempty"`
	Model     *string  `yaml:"model,omitempty" json:"model,omitempty"`
	Tags      []string `yaml:"tags,omitempty" json:"tags,omitempty"`
}

// PolicyRequest holds metadata extracted from the incoming request for policy matching.
type PolicyRequest struct {
	TeamAlias string
	KeyAlias  string
	Model     string
	Tags      []string
}

// PolicyResult holds the merged result of all matching policies.
type PolicyResult struct {
	RoutingStrategy string
	Guardrails      []string
}

// PolicyEngine evaluates request metadata against a set of policies.
type PolicyEngine struct {
	policies []Policy
}

// NewPolicyEngine creates a policy engine with the given policies.
func NewPolicyEngine(policies []Policy) *PolicyEngine {
	return &PolicyEngine{policies: policies}
}

// Evaluate matches the request against all policies and returns the merged result.
// Higher priority policies override lower priority ones for routing strategy.
// Guardrails are accumulated from all matching policies.
func (pe *PolicyEngine) Evaluate(req PolicyRequest) PolicyResult {
	var result PolicyResult
	bestPriority := -1

	for _, p := range pe.policies {
		if !matchConditions(p.Conditions, req) {
			continue
		}

		// Accumulate guardrails
		result.Guardrails = append(result.Guardrails, p.Guardrails...)

		// Higher priority wins for routing strategy
		if p.RoutingStrategy != nil && p.Priority > bestPriority {
			result.RoutingStrategy = *p.RoutingStrategy
			bestPriority = p.Priority
		}
	}

	// Deduplicate guardrails
	result.Guardrails = dedup(result.Guardrails)
	return result
}

func matchConditions(cond PolicyCondition, req PolicyRequest) bool {
	if cond.TeamAlias != nil && !matchWildcard(*cond.TeamAlias, req.TeamAlias) {
		return false
	}
	if cond.KeyAlias != nil && !matchWildcard(*cond.KeyAlias, req.KeyAlias) {
		return false
	}
	if cond.Model != nil && !matchWildcard(*cond.Model, req.Model) {
		return false
	}
	if len(cond.Tags) > 0 && !hasAnyTag(req.Tags, cond.Tags) {
		return false
	}
	return true
}

// matchWildcard matches a pattern against a value using Go's path.Match
// which supports *, ?, and [char-range] patterns.
func matchWildcard(pattern, value string) bool {
	if pattern == "*" {
		return true
	}
	matched, err := path.Match(pattern, value)
	if err != nil {
		// Fall back to exact match on invalid pattern
		return strings.EqualFold(pattern, value)
	}
	return matched
}

func hasAnyTag(have, want []string) bool {
	set := make(map[string]struct{}, len(have))
	for _, t := range have {
		set[t] = struct{}{}
	}
	for _, t := range want {
		if _, ok := set[t]; ok {
			return true
		}
	}
	return false
}

func dedup(ss []string) []string {
	if len(ss) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(ss))
	result := make([]string, 0, len(ss))
	for _, s := range ss {
		if _, ok := seen[s]; !ok {
			seen[s] = struct{}{}
			result = append(result, s)
		}
	}
	return result
}
