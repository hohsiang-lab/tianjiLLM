package policy

import (
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/praxisllmlab/tianjiLLM/internal/db"
	"github.com/praxisllmlab/tianjiLLM/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Resolver Tests ---

func TestResolveChain_Empty(t *testing.T) {
	result, err := ResolveChain(nil)
	require.NoError(t, err)
	assert.Empty(t, result.GuardrailsAdd)
}

func TestResolveChain_SinglePolicy(t *testing.T) {
	chain := []db.GetPolicyChainRow{
		{Name: "child", GuardrailsAdd: []string{"g1", "g2"}},
	}
	result, err := ResolveChain(chain)
	require.NoError(t, err)
	assert.Equal(t, "child", result.Name)
	assert.ElementsMatch(t, []string{"g1", "g2"}, result.GuardrailsAdd)
}

func TestResolveChain_InheritanceMerge(t *testing.T) {
	// Child first, root last (matches recursive CTE order)
	chain := []db.GetPolicyChainRow{
		{Name: "child", GuardrailsAdd: []string{"g3"}, GuardrailsRemove: []string{"g1"}},
		{Name: "parent", GuardrailsAdd: []string{"g1", "g2"}},
	}
	result, err := ResolveChain(chain)
	require.NoError(t, err)
	// parent adds g1, g2; child adds g3 and removes g1
	assert.ElementsMatch(t, []string{"g2", "g3"}, result.GuardrailsAdd)
}

func TestResolveChain_ThreeLevels(t *testing.T) {
	chain := []db.GetPolicyChainRow{
		{Name: "grandchild", GuardrailsAdd: []string{"g5"}, GuardrailsRemove: []string{"g2"}},
		{Name: "child", GuardrailsAdd: []string{"g3", "g4"}, GuardrailsRemove: []string{"g1"}},
		{Name: "root", GuardrailsAdd: []string{"g1", "g2"}},
	}
	result, err := ResolveChain(chain)
	require.NoError(t, err)
	// root: +g1,+g2 → child: -g1,+g3,+g4 → grandchild: -g2,+g5
	// result: g3, g4, g5
	assert.ElementsMatch(t, []string{"g3", "g4", "g5"}, result.GuardrailsAdd)
}

func TestResolveChain_CycleDetection(t *testing.T) {
	chain := []db.GetPolicyChainRow{
		{Name: "a"},
		{Name: "b"},
		{Name: "a"}, // cycle
	}
	_, err := ResolveChain(chain)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cycle detected")
}

// --- Matcher Tests ---

func TestMatchAttachments_GlobalScope(t *testing.T) {
	scope := "*"
	attachments := []db.PolicyAttachmentTable{
		{PolicyName: "p1", Scope: &scope},
	}
	matched := MatchAttachments(attachments, MatchRequest{Model: "anything"})
	assert.Len(t, matched, 1)
	assert.Equal(t, "p1", matched[0].PolicyName)
}

func TestMatchAttachments_TeamMatch(t *testing.T) {
	attachments := []db.PolicyAttachmentTable{
		{PolicyName: "p1", Teams: []string{"team-a", "team-b"}},
	}

	matched := MatchAttachments(attachments, MatchRequest{TeamID: "team-a"})
	assert.Len(t, matched, 1)

	matched = MatchAttachments(attachments, MatchRequest{TeamID: "team-c"})
	assert.Len(t, matched, 0)
}

func TestMatchAttachments_PrefixWildcard(t *testing.T) {
	attachments := []db.PolicyAttachmentTable{
		{PolicyName: "p1", Models: []string{"openai/*"}},
	}

	matched := MatchAttachments(attachments, MatchRequest{Model: "openai/gpt-4"})
	assert.Len(t, matched, 1)

	matched = MatchAttachments(attachments, MatchRequest{Model: "anthropic/claude"})
	assert.Len(t, matched, 0)
}

func TestMatchAttachments_MultiDimensionAND(t *testing.T) {
	attachments := []db.PolicyAttachmentTable{
		{
			PolicyName: "p1",
			Teams:      []string{"team-a"},
			Models:     []string{"openai/*"},
		},
	}

	// Both dimensions must match
	matched := MatchAttachments(attachments, MatchRequest{TeamID: "team-a", Model: "openai/gpt-4"})
	assert.Len(t, matched, 1)

	// Team matches but model doesn't
	matched = MatchAttachments(attachments, MatchRequest{TeamID: "team-a", Model: "anthropic/claude"})
	assert.Len(t, matched, 0)
}

func TestMatchAttachments_TagMatch(t *testing.T) {
	attachments := []db.PolicyAttachmentTable{
		{PolicyName: "p1", Tags: []string{"production"}},
	}

	matched := MatchAttachments(attachments, MatchRequest{Tags: []string{"production", "v2"}})
	assert.Len(t, matched, 1)

	matched = MatchAttachments(attachments, MatchRequest{Tags: []string{"staging"}})
	assert.Len(t, matched, 0)
}

func TestMatchAttachments_EmptyDimensionsMatchAll(t *testing.T) {
	attachments := []db.PolicyAttachmentTable{
		{PolicyName: "p1"}, // no dimensions — should match everything
	}
	matched := MatchAttachments(attachments, MatchRequest{TeamID: "any", Model: "any"})
	assert.Len(t, matched, 1)
}

// --- Pipeline Tests ---

func TestExecutePipeline_EmptyPipeline(t *testing.T) {
	result := ExecutePipeline([]byte(`{}`), &mockChecker{}, nil)
	assert.Equal(t, "allow", result.Action)
}

func TestExecutePipeline_AllPass(t *testing.T) {
	pipeline := `{"steps":[{"guardrail":"g1","on_pass":"next","on_fail":"block"},{"guardrail":"g2","on_pass":"allow","on_fail":"block"}]}`
	checker := &mockChecker{results: map[string]bool{"g1": true, "g2": true}}

	result := ExecutePipeline([]byte(pipeline), checker, map[string]any{})
	assert.Equal(t, "allow", result.Action)
	assert.Len(t, result.Steps, 2)
	assert.True(t, result.Steps[0].Passed)
	assert.True(t, result.Steps[1].Passed)
}

func TestExecutePipeline_FailBlock(t *testing.T) {
	pipeline := `{"steps":[{"guardrail":"g1","on_pass":"next","on_fail":"block"}]}`
	checker := &mockChecker{results: map[string]bool{"g1": false}}

	result := ExecutePipeline([]byte(pipeline), checker, map[string]any{})
	assert.Equal(t, "block", result.Action)
	assert.Len(t, result.Steps, 1)
	assert.False(t, result.Steps[0].Passed)
}

func TestExecutePipeline_ModifyResponse(t *testing.T) {
	pipeline := `{"steps":[{"guardrail":"g1","on_pass":"modify_response","on_fail":"block","modify_response_message":"custom msg"}]}`
	checker := &mockChecker{results: map[string]bool{"g1": true}}

	result := ExecutePipeline([]byte(pipeline), checker, map[string]any{})
	assert.Equal(t, "modify_response", result.Action)
	assert.Equal(t, "custom msg", result.Message)
}

func TestExecutePipeline_PassData(t *testing.T) {
	pipeline := `{"steps":[{"guardrail":"g1","on_pass":"next","on_fail":"block","pass_data":true},{"guardrail":"g2","on_pass":"allow","on_fail":"block"}]}`
	checker := &passDataChecker{}

	input := map[string]any{"key": "value"}
	result := ExecutePipeline([]byte(pipeline), checker, input)
	assert.Equal(t, "allow", result.Action)
}

// --- Test Helpers ---

type mockChecker struct {
	results map[string]bool
}

func (c *mockChecker) Check(name string, _ map[string]any) (bool, error) {
	if c.results == nil {
		return true, nil
	}
	return c.results[name], nil
}

type passDataChecker struct{}

func (c *passDataChecker) Check(name string, input map[string]any) (bool, error) {
	// g2 checks that _prev_result was forwarded
	if name == "g2" {
		_, hasPrev := input["_prev_result"]
		return hasPrev, nil
	}
	return true, nil
}

// Ensure model.PipelineConfig is deserializable (compile-time check)
var _ = model.PipelineConfig{}
var _ = pgtype.Timestamptz{}

// --- Engine Tests (no-match path, no DB needed) ---

func TestEngine_Evaluate_NoAttachments(t *testing.T) {
	e := &Engine{
		policies: make(map[string]db.PolicyTable),
	}
	result, err := e.Evaluate(nil, MatchRequest{KeyID: "k1"})
	require.NoError(t, err)
	assert.Empty(t, result.Guardrails)
	assert.Empty(t, result.Policies)
}
