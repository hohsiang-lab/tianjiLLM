package guardrail

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/praxisllmlab/tianjiLLM/internal/model"
)

// LassoGuardrail calls Lasso Security's API for content safety checks.
type LassoGuardrail struct {
	apiKey  string
	baseURL string
}

func NewLassoGuardrail(apiKey, baseURL string) *LassoGuardrail {
	if baseURL == "" {
		baseURL = "https://api.lasso.security/v1"
	}
	return &LassoGuardrail{apiKey: apiKey, baseURL: baseURL}
}

func (g *LassoGuardrail) Name() string           { return "lasso" }
func (g *LassoGuardrail) SupportedHooks() []Hook { return []Hook{HookPreCall, HookPostCall} }

func (g *LassoGuardrail) Run(ctx context.Context, hook Hook, req *model.ChatCompletionRequest, resp *model.ModelResponse) (Result, error) {
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
		return Result{}, fmt.Errorf("lasso guardrail: %w", err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		return Result{}, fmt.Errorf("lasso guardrail: status %d", httpResp.StatusCode)
	}

	var result struct {
		Action  string `json:"action"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(httpResp.Body).Decode(&result); err != nil {
		return Result{}, fmt.Errorf("lasso guardrail: decode: %w", err)
	}

	if result.Action == "block" {
		return Result{Passed: false, Message: result.Message}, nil
	}
	return Result{Passed: true}, nil
}
