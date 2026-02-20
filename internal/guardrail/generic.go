package guardrail

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/praxisllmlab/tianjiLLM/internal/model"
)

// GenericGuardrail calls a configurable HTTP endpoint for content safety checks.
type GenericGuardrail struct {
	name     string
	endpoint string
	headers  map[string]string
}

func (g *GenericGuardrail) Name() string {
	if g.name != "" {
		return g.name
	}
	return "generic_guardrail"
}

func (g *GenericGuardrail) SupportedHooks() []Hook { return []Hook{HookPreCall, HookPostCall} }

func (g *GenericGuardrail) Run(ctx context.Context, hook Hook, req *model.ChatCompletionRequest, resp *model.ModelResponse) (Result, error) {
	prompt := extractContent(HookPreCall, req, nil)
	response := extractContent(HookPostCall, nil, resp)

	body, _ := json.Marshal(map[string]any{
		"prompt":   prompt,
		"response": response,
		"metadata": map[string]string{
			"hook": string(hook),
		},
	})

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, g.endpoint, bytes.NewReader(body))
	if err != nil {
		return Result{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	for k, v := range g.headers {
		httpReq.Header.Set(k, v)
	}

	httpResp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return Result{}, fmt.Errorf("generic guardrail: %w", err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		return Result{}, fmt.Errorf("generic guardrail: status %d", httpResp.StatusCode)
	}

	var result struct {
		Action          string `json:"action"`
		Message         string `json:"message"`
		ModifiedContent string `json:"modified_content"`
	}
	if err := json.NewDecoder(httpResp.Body).Decode(&result); err != nil {
		return Result{}, fmt.Errorf("generic guardrail: decode: %w", err)
	}

	if result.Action == "block" {
		return Result{Passed: false, Message: result.Message}, nil
	}

	if result.ModifiedContent != "" && req != nil {
		modified := *req
		msgs := make([]model.Message, len(modified.Messages))
		copy(msgs, modified.Messages)
		msgs[len(msgs)-1].Content = result.ModifiedContent
		modified.Messages = msgs
		return Result{Passed: true, ModifiedRequest: &modified}, nil
	}

	return Result{Passed: true}, nil
}
