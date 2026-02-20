package guardrail

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/praxisllmlab/tianjiLLM/internal/model"
)

// CustomCodeGuardrail calls a user-configured endpoint for content safety checks.
type CustomCodeGuardrail struct {
	apiKey  string
	baseURL string
}

func NewCustomCodeGuardrail(apiKey, baseURL string) *CustomCodeGuardrail {
	return &CustomCodeGuardrail{apiKey: apiKey, baseURL: baseURL}
}

func (g *CustomCodeGuardrail) Name() string           { return "custom_code" }
func (g *CustomCodeGuardrail) SupportedHooks() []Hook { return []Hook{HookPreCall, HookPostCall} }

func (g *CustomCodeGuardrail) Run(ctx context.Context, hook Hook, req *model.ChatCompletionRequest, resp *model.ModelResponse) (Result, error) {
	if g.baseURL == "" {
		return Result{}, fmt.Errorf("custom_code guardrail: base URL not configured")
	}

	content := extractContent(hook, req, resp)
	if content == "" {
		return Result{Passed: true}, nil
	}

	body, _ := json.Marshal(map[string]any{"text": content, "hook": string(hook)})
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, g.baseURL+"/check", bytes.NewReader(body))
	if err != nil {
		return Result{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if g.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+g.apiKey)
	}

	httpResp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return Result{}, fmt.Errorf("custom_code guardrail: %w", err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		return Result{}, fmt.Errorf("custom_code guardrail: status %d", httpResp.StatusCode)
	}

	var result struct {
		Action  string `json:"action"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(httpResp.Body).Decode(&result); err != nil {
		return Result{}, fmt.Errorf("custom_code guardrail: decode: %w", err)
	}

	if result.Action == "block" {
		return Result{Passed: false, Message: result.Message}, nil
	}
	return Result{Passed: true}, nil
}
