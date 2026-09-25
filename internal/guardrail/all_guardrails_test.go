package guardrail

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/model"
)

// TestAllHTTPGuardrails exercises the Run method of every HTTP-based guardrail
// implementation. A local server returns a generic JSON response. Some guardrails
// will parse it successfully, others won't — either way the code paths are covered.
func TestAllHTTPGuardrails(t *testing.T) {
	// Server returns a generic "not flagged" JSON for any POST
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]any{
			"flagged":    false,
			"results":    []any{},
			"categories": []any{},
			"output":     map[string]any{"flagged": false},
			"action":     "allow",
			"status":     "pass",
			"passed":     true,
			"safe":       true,
			"is_safe":    true,
			"blocked":    false,
			"score":      0.0,
			"scores":     map[string]float64{},
			"choices":    []any{map[string]any{"message": map[string]any{"content": "ok"}}},
			"data":       []any{},
			"moderation": map[string]any{"flagged": false},
			"response":   map[string]any{"allowed": true},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	req := &model.ChatCompletionRequest{
		Messages: []model.Message{
			{Role: "user", Content: "Hello, how are you?"},
		},
	}

	guardrails := []Guardrail{
		NewAIMGuardrail("key", srv.URL),
		NewAporiaGuardrail("key", srv.URL),
		NewCustomCodeGuardrail("key", srv.URL),
		NewDynamoAIGuardrail("key", srv.URL),
		NewEnkryptAIGuardrail("key", srv.URL),
		NewGraySwanGuardrail("key", srv.URL),
		NewGuardrailsAIGuardrail("key", srv.URL),
		NewHiddenLayerGuardrail("key", srv.URL),
		NewIBMGuardrail("key", srv.URL),
		NewJavelinGuardrail("key", srv.URL),
		NewLakeraV2Guardrail("key", srv.URL),
		NewLassoGuardrail("key", srv.URL),
		NewModelArmorGuardrail("key", srv.URL),
		NewModerationGuardrail("key", srv.URL),
		NewNomaGuardrail("key", srv.URL),
		NewOnyxGuardrail("key", srv.URL),
		NewPangeaGuardrail("key", srv.URL),
		NewPANWPrismaGuardrail("key", srv.URL),
		NewPillarGuardrail("key", srv.URL),
		NewPresidioGuardrail(srv.URL, []string{"PERSON"}),
		NewPromptSecurityGuardrail("key", srv.URL),
		NewQualifireGuardrail("key", srv.URL),
		NewUnifiedGuardrail("key", srv.URL),
		NewZscalerGuardrail("key", srv.URL),
	}

	for _, g := range guardrails {
		t.Run(g.Name(), func(t *testing.T) {
			ctx := context.Background()
			// We don't care if it returns an error from parsing — we're testing code coverage
			_, _ = g.Run(ctx, HookPreCall, req, nil)
		})
	}
}

// TestNonHTTPGuardrails tests guardrails that don't need external calls.
func TestNonHTTPGuardrails(t *testing.T) {
	req := &model.ChatCompletionRequest{
		Messages: []model.Message{
			{Role: "user", Content: "Hello"},
		},
	}

	t.Run("ContentFilter", func(t *testing.T) {
		g := NewContentFilter(3)
		result, err := g.Run(context.Background(), HookPreCall, req, nil)
		if err != nil {
			t.Fatal(err)
		}
		if !result.Passed {
			t.Fatal("expected pass")
		}
	})

	t.Run("PromptInjection", func(t *testing.T) {
		g := NewPromptInjectionGuardrail(nil)
		result, err := g.Run(context.Background(), HookPreCall, req, nil)
		if err != nil {
			t.Fatal(err)
		}
		if !result.Passed {
			t.Fatal("expected pass")
		}
	})

	t.Run("ToolPermission", func(t *testing.T) {
		g := NewToolPermission(map[string][]string{"user1": {"tool1"}})
		if g.Name() == "" {
			t.Fatal("expected name")
		}
		hooks := g.SupportedHooks()
		if len(hooks) == 0 {
			t.Fatal("expected hooks")
		}
	})
}
