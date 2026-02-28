package guardrail

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/praxisllmlab/tianjiLLM/internal/model"
)

func TestNewFromConfigAllModes(t *testing.T) {
	modes := []struct {
		mode   string
		params map[string]any
	}{
		{"openai_moderation", nil},
		{"presidio", nil},
		{"prompt_injection", nil},
		{"lakera_guard", nil},
		{"azure_prompt_shield", map[string]any{"endpoint": "https://e"}},
		{"azure_text_moderation", map[string]any{"endpoint": "https://e", "threshold": 3}},
		{"content_filter", map[string]any{"threshold": 2}},
		{"tool_permission", map[string]any{"allowed_tools": map[string]any{"*": []any{"web_search"}}}},
		{"generic", map[string]any{"endpoint": "http://e"}},
		{"aim", nil},
		{"aporia", nil},
		{"custom_code", nil},
		{"dynamoai", nil},
		{"enkryptai", nil},
		{"grayswan", nil},
		{"guardrails_ai", nil},
		{"hiddenlayer", nil},
		{"ibm_guardrails", nil},
		{"javelin", nil},
		{"lakera_v2", nil},
		{"lasso", nil},
		{"model_armor", nil},
		{"noma", nil},
		{"onyx", nil},
		{"pangea", nil},
		{"panw_prisma_airs", nil},
		{"pillar", nil},
		{"prompt_security", nil},
		{"qualifire", nil},
		{"unified_guardrail", nil},
		{"zscaler_ai_guard", nil},
	}

	for _, tc := range modes {
		t.Run(tc.mode, func(t *testing.T) {
			params := map[string]any{"mode": tc.mode, "api_key": "k", "api_base": "http://localhost"}
			for k, v := range tc.params {
				params[k] = v
			}
			gc := config.GuardrailConfig{
				GuardrailName: tc.mode,
				TianjiParams:  params,
			}
			g, err := NewFromConfig(gc)
			if err != nil {
				t.Fatalf("mode %s: %v", tc.mode, err)
			}
			if g.Name() == "" {
				t.Fatal("empty name")
			}
			hooks := g.SupportedHooks()
			if len(hooks) == 0 {
				t.Fatal("no hooks")
			}
		})
	}
}

func TestNewFromConfigUnknown(t *testing.T) {
	_, err := NewFromConfig(config.GuardrailConfig{
		GuardrailName: "x",
		TianjiParams:  map[string]any{"mode": "nonexistent"},
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

// mockGuardrailServer creates a server that returns pass/block based on action field
func mockGuardrailServer(t *testing.T, action string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		resp := map[string]any{"action": action, "message": "test", "flagged": action == "block", "is_safe": action != "block", "safe": action != "block"}
		_ = json.NewEncoder(w).Encode(resp)
	}))
}

// Test several HTTP-based guardrails with mock servers
func TestAIMGuardrailRun(t *testing.T) {
	srv := mockGuardrailServer(t, "allow")
	defer srv.Close()

	g := NewAIMGuardrail("key", srv.URL)
	r, err := g.Run(context.Background(), HookPreCall, &model.ChatCompletionRequest{
		Messages: []model.Message{{Role: "user", Content: "hello"}},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !r.Passed {
		t.Fatal("should pass")
	}
}

func TestAIMGuardrailBlock(t *testing.T) {
	srv := mockGuardrailServer(t, "block")
	defer srv.Close()

	g := NewAIMGuardrail("key", srv.URL)
	r, err := g.Run(context.Background(), HookPreCall, &model.ChatCompletionRequest{
		Messages: []model.Message{{Role: "user", Content: "bad"}},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if r.Passed {
		t.Fatal("should block")
	}
}

func TestAIMGuardrailEmpty(t *testing.T) {
	g := NewAIMGuardrail("key", "http://localhost")
	r, err := g.Run(context.Background(), HookPreCall, &model.ChatCompletionRequest{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !r.Passed {
		t.Fatal("empty should pass")
	}
}

func TestAIMGuardrailServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv.Close()

	g := NewAIMGuardrail("key", srv.URL)
	_, err := g.Run(context.Background(), HookPreCall, &model.ChatCompletionRequest{
		Messages: []model.Message{{Role: "user", Content: "test"}},
	}, nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestAporiaGuardrailRun(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"action": "passthrough"})
	}))
	defer srv.Close()

	g := NewAporiaGuardrail("key", srv.URL)
	r, err := g.Run(context.Background(), HookPreCall, &model.ChatCompletionRequest{
		Messages: []model.Message{{Role: "user", Content: "hello"}},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !r.Passed {
		t.Fatal("should pass")
	}
}

func TestDynamoAIGuardrailRun(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"safe": true})
	}))
	defer srv.Close()

	g := NewDynamoAIGuardrail("key", srv.URL)
	r, err := g.Run(context.Background(), HookPreCall, &model.ChatCompletionRequest{
		Messages: []model.Message{{Role: "user", Content: "test"}},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !r.Passed {
		t.Fatal("should pass")
	}
}

func TestEnkryptAIGuardrailRun(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"is_safe": true})
	}))
	defer srv.Close()

	g := NewEnkryptAIGuardrail("key", srv.URL)
	r, err := g.Run(context.Background(), HookPreCall, &model.ChatCompletionRequest{
		Messages: []model.Message{{Role: "user", Content: "test"}},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !r.Passed {
		t.Fatal("should pass")
	}
}

func TestGraySwanGuardrailRun(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"safe": true})
	}))
	defer srv.Close()

	g := NewGraySwanGuardrail("key", srv.URL)
	r, err := g.Run(context.Background(), HookPreCall, &model.ChatCompletionRequest{
		Messages: []model.Message{{Role: "user", Content: "test"}},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !r.Passed {
		t.Fatal("should pass")
	}
}

func TestHiddenLayerGuardrailRun(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"is_safe": true})
	}))
	defer srv.Close()

	g := NewHiddenLayerGuardrail("key", srv.URL)
	r, err := g.Run(context.Background(), HookPreCall, &model.ChatCompletionRequest{
		Messages: []model.Message{{Role: "user", Content: "test"}},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !r.Passed {
		t.Fatal("should pass")
	}
}

func TestIBMGuardrailRun(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"is_safe": true})
	}))
	defer srv.Close()

	g := NewIBMGuardrail("key", srv.URL)
	r, err := g.Run(context.Background(), HookPreCall, &model.ChatCompletionRequest{
		Messages: []model.Message{{Role: "user", Content: "test"}},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !r.Passed {
		t.Fatal("should pass")
	}
}

func TestJavelinGuardrailRun(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"safe": true})
	}))
	defer srv.Close()

	g := NewJavelinGuardrail("key", srv.URL)
	r, err := g.Run(context.Background(), HookPreCall, &model.ChatCompletionRequest{
		Messages: []model.Message{{Role: "user", Content: "test"}},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !r.Passed {
		t.Fatal("should pass")
	}
}

func TestCustomCodeGuardrailRun(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"action": "allow"})
	}))
	defer srv.Close()

	g := NewCustomCodeGuardrail("key", srv.URL)
	r, err := g.Run(context.Background(), HookPreCall, &model.ChatCompletionRequest{
		Messages: []model.Message{{Role: "user", Content: "test"}},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !r.Passed {
		t.Fatal("should pass")
	}
}

func TestLakeraV2GuardrailRun(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"flagged": false})
	}))
	defer srv.Close()

	g := NewLakeraV2Guardrail("key", srv.URL)
	r, err := g.Run(context.Background(), HookPreCall, &model.ChatCompletionRequest{
		Messages: []model.Message{{Role: "user", Content: "test"}},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !r.Passed {
		t.Fatal("should pass")
	}
}

func TestLassoGuardrailRun(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"safe": true})
	}))
	defer srv.Close()

	g := NewLassoGuardrail("key", srv.URL)
	r, err := g.Run(context.Background(), HookPreCall, &model.ChatCompletionRequest{
		Messages: []model.Message{{Role: "user", Content: "test"}},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !r.Passed {
		t.Fatal("should pass")
	}
}

func TestModelArmorGuardrailRun(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"safe": true})
	}))
	defer srv.Close()

	g := NewModelArmorGuardrail("key", srv.URL)
	r, err := g.Run(context.Background(), HookPreCall, &model.ChatCompletionRequest{
		Messages: []model.Message{{Role: "user", Content: "test"}},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !r.Passed {
		t.Fatal("should pass")
	}
}

func TestNomaGuardrailRun(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"safe": true})
	}))
	defer srv.Close()

	g := NewNomaGuardrail("key", srv.URL)
	r, err := g.Run(context.Background(), HookPreCall, &model.ChatCompletionRequest{
		Messages: []model.Message{{Role: "user", Content: "test"}},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !r.Passed {
		t.Fatal("should pass")
	}
}

func TestOnyxGuardrailRun(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"is_safe": true})
	}))
	defer srv.Close()

	g := NewOnyxGuardrail("key", srv.URL)
	r, err := g.Run(context.Background(), HookPreCall, &model.ChatCompletionRequest{
		Messages: []model.Message{{Role: "user", Content: "test"}},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !r.Passed {
		t.Fatal("should pass")
	}
}

func TestPangeaGuardrailRun(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"safe": true})
	}))
	defer srv.Close()

	g := NewPangeaGuardrail("key", srv.URL)
	r, err := g.Run(context.Background(), HookPreCall, &model.ChatCompletionRequest{
		Messages: []model.Message{{Role: "user", Content: "test"}},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !r.Passed {
		t.Fatal("should pass")
	}
}

func TestPANWPrismaGuardrailRun(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"action": "allow"})
	}))
	defer srv.Close()

	g := NewPANWPrismaGuardrail("key", srv.URL)
	r, err := g.Run(context.Background(), HookPreCall, &model.ChatCompletionRequest{
		Messages: []model.Message{{Role: "user", Content: "test"}},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !r.Passed {
		t.Fatal("should pass")
	}
}

func TestPillarGuardrailRun(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"safe": true})
	}))
	defer srv.Close()

	g := NewPillarGuardrail("key", srv.URL)
	r, err := g.Run(context.Background(), HookPreCall, &model.ChatCompletionRequest{
		Messages: []model.Message{{Role: "user", Content: "test"}},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !r.Passed {
		t.Fatal("should pass")
	}
}

func TestPromptSecurityGuardrailRun(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"safe": true})
	}))
	defer srv.Close()

	g := NewPromptSecurityGuardrail("key", srv.URL)
	r, err := g.Run(context.Background(), HookPreCall, &model.ChatCompletionRequest{
		Messages: []model.Message{{Role: "user", Content: "test"}},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !r.Passed {
		t.Fatal("should pass")
	}
}

func TestQualifireGuardrailRun(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"safe": true})
	}))
	defer srv.Close()

	g := NewQualifireGuardrail("key", srv.URL)
	r, err := g.Run(context.Background(), HookPreCall, &model.ChatCompletionRequest{
		Messages: []model.Message{{Role: "user", Content: "test"}},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !r.Passed {
		t.Fatal("should pass")
	}
}

func TestUnifiedGuardrailRun(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"safe": true})
	}))
	defer srv.Close()

	g := NewUnifiedGuardrail("key", srv.URL)
	r, err := g.Run(context.Background(), HookPreCall, &model.ChatCompletionRequest{
		Messages: []model.Message{{Role: "user", Content: "test"}},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !r.Passed {
		t.Fatal("should pass")
	}
}

func TestZscalerGuardrailRun(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"safe": true})
	}))
	defer srv.Close()

	g := NewZscalerGuardrail("key", srv.URL)
	r, err := g.Run(context.Background(), HookPreCall, &model.ChatCompletionRequest{
		Messages: []model.Message{{Role: "user", Content: "test"}},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !r.Passed {
		t.Fatal("should pass")
	}
}

func TestGuardrailsAIGuardrailRun(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"safe": true})
	}))
	defer srv.Close()

	g := NewGuardrailsAIGuardrail("key", srv.URL)
	r, err := g.Run(context.Background(), HookPreCall, &model.ChatCompletionRequest{
		Messages: []model.Message{{Role: "user", Content: "test"}},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !r.Passed {
		t.Fatal("should pass")
	}
}
