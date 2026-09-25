package guardrail

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/praxisllmlab/tianjiLLM/internal/model"
)

// AporiaGuardrail calls Aporia's API for content safety checks.
type AporiaGuardrail struct {
	apiKey  string
	baseURL string
}

func NewAporiaGuardrail(apiKey, baseURL string) *AporiaGuardrail {
	if baseURL == "" {
		baseURL = "https://guardrails.aporia.com/v1"
	}
	return &AporiaGuardrail{apiKey: apiKey, baseURL: baseURL}
}

func (g *AporiaGuardrail) Name() string           { return "aporia" }
func (g *AporiaGuardrail) SupportedHooks() []Hook { return []Hook{HookPreCall, HookPostCall} }

func (g *AporiaGuardrail) Run(ctx context.Context, hook Hook, req *model.ChatCompletionRequest, resp *model.ModelResponse) (Result, error) {
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
	httpReq.Header.Set("Authorization", "Bearer "+g.apiKey)

	httpResp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return Result{}, fmt.Errorf("aporia guardrail: %w", err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		return Result{}, fmt.Errorf("aporia guardrail: status %d", httpResp.StatusCode)
	}

	var result struct {
		Action  string `json:"action"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(httpResp.Body).Decode(&result); err != nil {
		return Result{}, fmt.Errorf("aporia guardrail: decode: %w", err)
	}

	if result.Action == "block" {
		return Result{Passed: false, Message: result.Message}, nil
	}
	return Result{Passed: true}, nil
}
