package guardrail

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/praxisllmlab/tianjiLLM/internal/model"
)

// PANWPrismaGuardrail calls Palo Alto Networks Prisma AIRS API for content safety checks.
type PANWPrismaGuardrail struct {
	apiKey  string
	baseURL string
}

func NewPANWPrismaGuardrail(apiKey, baseURL string) *PANWPrismaGuardrail {
	if baseURL == "" {
		baseURL = "https://api.prisma.paloaltonetworks.com/v1"
	}
	return &PANWPrismaGuardrail{apiKey: apiKey, baseURL: baseURL}
}

func (g *PANWPrismaGuardrail) Name() string           { return "panw_prisma_airs" }
func (g *PANWPrismaGuardrail) SupportedHooks() []Hook { return []Hook{HookPreCall, HookPostCall} }

func (g *PANWPrismaGuardrail) Run(ctx context.Context, hook Hook, req *model.ChatCompletionRequest, resp *model.ModelResponse) (Result, error) {
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
		return Result{}, fmt.Errorf("panw_prisma guardrail: %w", err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		return Result{}, fmt.Errorf("panw_prisma guardrail: status %d", httpResp.StatusCode)
	}

	var result struct {
		Action  string `json:"action"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(httpResp.Body).Decode(&result); err != nil {
		return Result{}, fmt.Errorf("panw_prisma guardrail: decode: %w", err)
	}

	if result.Action == "block" {
		return Result{Passed: false, Message: result.Message}, nil
	}
	return Result{Passed: true}, nil
}
