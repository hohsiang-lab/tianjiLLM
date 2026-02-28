package router

import (
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/config"
)

func strPtr(s string) *string { return &s }

func TestNewPolicyEngine(t *testing.T) {
	pe := NewPolicyEngine(nil)
	if pe == nil {
		t.Fatal("nil")
	}
}

func TestEvaluateNoMatch(t *testing.T) {
	pe := NewPolicyEngine([]Policy{
		{Name: "p1", Conditions: PolicyCondition{Model: strPtr("gpt-4o")}, Guardrails: []string{"pii"}},
	})
	result := pe.Evaluate(PolicyRequest{Model: "claude-3"})
	if len(result.Guardrails) != 0 {
		t.Fatalf("expected no guardrails, got %v", result.Guardrails)
	}
}

func TestEvaluateMatch(t *testing.T) {
	pe := NewPolicyEngine([]Policy{
		{Name: "p1", Conditions: PolicyCondition{Model: strPtr("gpt-4o")}, Guardrails: []string{"pii", "toxic"}},
	})
	result := pe.Evaluate(PolicyRequest{Model: "gpt-4o"})
	if len(result.Guardrails) != 2 {
		t.Fatalf("expected 2 guardrails, got %v", result.Guardrails)
	}
}

func TestEvaluateRoutingStrategy(t *testing.T) {
	pe := NewPolicyEngine([]Policy{
		{Name: "low", Conditions: PolicyCondition{}, RoutingStrategy: strPtr("round-robin"), Priority: 1},
		{Name: "high", Conditions: PolicyCondition{}, RoutingStrategy: strPtr("lowest-latency"), Priority: 10},
	})
	result := pe.Evaluate(PolicyRequest{})
	if result.RoutingStrategy != "lowest-latency" {
		t.Fatalf("strategy = %q, want lowest-latency", result.RoutingStrategy)
	}
}

func TestEvaluateWildcard(t *testing.T) {
	pe := NewPolicyEngine([]Policy{
		{Name: "p1", Conditions: PolicyCondition{Model: strPtr("gpt-*")}, Guardrails: []string{"g1"}},
	})
	result := pe.Evaluate(PolicyRequest{Model: "gpt-4o"})
	if len(result.Guardrails) != 1 {
		t.Fatal("wildcard should match")
	}
}

func TestEvaluateTeamAndKey(t *testing.T) {
	pe := NewPolicyEngine([]Policy{
		{Name: "p1", Conditions: PolicyCondition{TeamAlias: strPtr("eng"), KeyAlias: strPtr("prod-*")}, Guardrails: []string{"g1"}},
	})
	result := pe.Evaluate(PolicyRequest{TeamAlias: "eng", KeyAlias: "prod-key1"})
	if len(result.Guardrails) != 1 {
		t.Fatal("team+key should match")
	}
	result = pe.Evaluate(PolicyRequest{TeamAlias: "eng", KeyAlias: "dev-key"})
	if len(result.Guardrails) != 0 {
		t.Fatal("should not match wrong key")
	}
}

func TestEvaluateTags(t *testing.T) {
	pe := NewPolicyEngine([]Policy{
		{Name: "p1", Conditions: PolicyCondition{Tags: []string{"finance"}}, Guardrails: []string{"pii"}},
	})
	result := pe.Evaluate(PolicyRequest{Tags: []string{"finance", "internal"}})
	if len(result.Guardrails) != 1 {
		t.Fatal("tag should match")
	}
	result = pe.Evaluate(PolicyRequest{Tags: []string{"marketing"}})
	if len(result.Guardrails) != 0 {
		t.Fatal("wrong tag should not match")
	}
}

func TestEvaluateDedup(t *testing.T) {
	pe := NewPolicyEngine([]Policy{
		{Name: "p1", Conditions: PolicyCondition{}, Guardrails: []string{"pii", "toxic"}},
		{Name: "p2", Conditions: PolicyCondition{}, Guardrails: []string{"toxic", "bias"}},
	})
	result := pe.Evaluate(PolicyRequest{})
	if len(result.Guardrails) != 3 {
		t.Fatalf("expected 3 deduped guardrails, got %v", result.Guardrails)
	}
}

func TestMatchWildcardStar(t *testing.T) {
	if !matchWildcard("*", "anything") {
		t.Fatal("* should match anything")
	}
}

func TestMatchWildcardExact(t *testing.T) {
	if !matchWildcard("gpt-4o", "gpt-4o") {
		t.Fatal("exact match")
	}
	if matchWildcard("gpt-4o", "gpt-3.5") {
		t.Fatal("should not match")
	}
}

func TestHasAnyTag(t *testing.T) {
	if !hasAnyTag([]string{"a", "b"}, []string{"b", "c"}) {
		t.Fatal("should overlap")
	}
	if hasAnyTag([]string{"a"}, []string{"b"}) {
		t.Fatal("should not overlap")
	}
}

func TestDedup(t *testing.T) {
	result := dedup([]string{"a", "b", "a", "c", "b"})
	if len(result) != 3 {
		t.Fatalf("dedup got %v", result)
	}
	if r := dedup(nil); r != nil {
		t.Fatal("nil input should return nil")
	}
}

func TestLatencyEMA(t *testing.T) {
	d := &Deployment{}
	if d.LatencyEMA() != 0 {
		t.Fatal("initial latency should be 0")
	}
}

func TestAPIKeyNil(t *testing.T) {
	d := &Deployment{Config: &config.ModelConfig{}}
	if d.APIKey() != "" {
		t.Fatal("nil key should return empty")
	}
}

func TestAPIKeySet(t *testing.T) {
	k := "sk-123"
	d := &Deployment{Config: &config.ModelConfig{TianjiParams: config.TianjiParams{APIKey: &k}}}
	if d.APIKey() != "sk-123" {
		t.Fatalf("APIKey = %q", d.APIKey())
	}
}
