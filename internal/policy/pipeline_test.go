package policy

import (
	"encoding/json"
	"testing"
)

type pipelineMockChecker struct {
	results map[string]bool
}

func (m *pipelineMockChecker) Check(name string, _ map[string]any) (bool, error) {
	return m.results[name], nil
}

func makePipeline(t *testing.T, steps []map[string]any) []byte {
	t.Helper()
	cfg := map[string]any{"mode": "pre_call", "steps": steps}
	b, _ := json.Marshal(cfg)
	return b
}

func TestExecutePipelineAllow(t *testing.T) {
	pipeline := makePipeline(t, []map[string]any{
		{"guardrail": "pii", "on_pass": "allow", "on_fail": "block"},
	})
	checker := &pipelineMockChecker{results: map[string]bool{"pii": true}}
	r := ExecutePipeline(pipeline, checker, map[string]any{"text": "hello"})
	if r.Action != "allow" {
		t.Fatalf("got %q, want allow", r.Action)
	}
}

func TestExecutePipelineBlock(t *testing.T) {
	pipeline := makePipeline(t, []map[string]any{
		{"guardrail": "pii", "on_pass": "allow", "on_fail": "block"},
	})
	checker := &pipelineMockChecker{results: map[string]bool{"pii": false}}
	r := ExecutePipeline(pipeline, checker, map[string]any{})
	if r.Action != "block" {
		t.Fatalf("got %q, want block", r.Action)
	}
}

func TestExecutePipelineModifyResponse(t *testing.T) {
	pipeline := makePipeline(t, []map[string]any{
		{"guardrail": "toxic", "on_pass": "next", "on_fail": "modify_response", "modify_response_message": "content filtered"},
	})
	checker := &pipelineMockChecker{results: map[string]bool{"toxic": false}}
	r := ExecutePipeline(pipeline, checker, map[string]any{})
	if r.Action != "modify_response" {
		t.Fatalf("got %q, want modify_response", r.Action)
	}
	if r.Message != "content filtered" {
		t.Fatalf("message = %q", r.Message)
	}
}

func TestExecutePipelineMultiStepNext(t *testing.T) {
	pipeline := makePipeline(t, []map[string]any{
		{"guardrail": "pii", "on_pass": "next", "on_fail": "block"},
		{"guardrail": "toxic", "on_pass": "allow", "on_fail": "block"},
	})
	checker := &pipelineMockChecker{results: map[string]bool{"pii": true, "toxic": true}}
	r := ExecutePipeline(pipeline, checker, map[string]any{})
	if r.Action != "allow" {
		t.Fatalf("got %q, want allow", r.Action)
	}
	if len(r.Steps) != 2 {
		t.Fatalf("steps = %d, want 2", len(r.Steps))
	}
}

func TestExecutePipelineAllNext(t *testing.T) {
	// All steps "next" → default allow
	pipeline := makePipeline(t, []map[string]any{
		{"guardrail": "a", "on_pass": "next", "on_fail": "next"},
		{"guardrail": "b", "on_pass": "next", "on_fail": "next"},
	})
	checker := &pipelineMockChecker{results: map[string]bool{"a": true, "b": false}}
	r := ExecutePipeline(pipeline, checker, map[string]any{})
	if r.Action != "allow" {
		t.Fatalf("got %q, want allow (default)", r.Action)
	}
}

func TestExecutePipelineEmptySteps(t *testing.T) {
	pipeline := makePipeline(t, []map[string]any{})
	r := ExecutePipeline(pipeline, &pipelineMockChecker{}, map[string]any{})
	if r.Action != "allow" {
		t.Fatalf("got %q, want allow", r.Action)
	}
}

func TestExecutePipelineInvalidJSON(t *testing.T) {
	r := ExecutePipeline([]byte("not json"), &pipelineMockChecker{}, map[string]any{})
	if r.Action != "block" {
		t.Fatalf("got %q, want block", r.Action)
	}
}

func TestExecutePipelineUnknownAction(t *testing.T) {
	pipeline := makePipeline(t, []map[string]any{
		{"guardrail": "x", "on_pass": "unknown_action", "on_fail": "block"},
	})
	checker := &pipelineMockChecker{results: map[string]bool{"x": true}}
	r := ExecutePipeline(pipeline, checker, map[string]any{})
	if r.Action != "block" {
		t.Fatalf("got %q, want block for unknown action", r.Action)
	}
}

func TestExecutePipelinePassData(t *testing.T) {
	pipeline := makePipeline(t, []map[string]any{
		{"guardrail": "a", "on_pass": "next", "on_fail": "next", "pass_data": true},
		{"guardrail": "b", "on_pass": "allow", "on_fail": "block"},
	})
	checker := &pipelineMockChecker{results: map[string]bool{"a": true, "b": true}}
	r := ExecutePipeline(pipeline, checker, map[string]any{"key": "val"})
	if r.Action != "allow" {
		t.Fatalf("got %q, want allow", r.Action)
	}
}
