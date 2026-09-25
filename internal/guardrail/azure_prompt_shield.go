package guardrail

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/praxisllmlab/tianjiLLM/internal/model"
)

// AzurePromptShield uses Azure Content Safety text:shieldPrompt API.
type AzurePromptShield struct {
	endpoint string
	apiKey   string
}

func (a *AzurePromptShield) Name() string { return "azure_prompt_shield" }

func (a *AzurePromptShield) SupportedHooks() []Hook { return []Hook{HookPreCall} }

func (a *AzurePromptShield) Run(ctx context.Context, hook Hook, req *model.ChatCompletionRequest, resp *model.ModelResponse) (Result, error) {
	content := extractContent(hook, req, resp)
	if content == "" {
		return Result{Passed: true}, nil
	}

	body, _ := json.Marshal(map[string]any{
		"userPrompt": content,
		"documents":  []string{},
	})

	url := fmt.Sprintf("%s/contentsafety/text:shieldPrompt?api-version=2024-09-01", a.endpoint)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return Result{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Ocp-Apim-Subscription-Key", a.apiKey)

	httpResp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return Result{}, fmt.Errorf("azure prompt shield: %w", err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		return Result{}, fmt.Errorf("azure prompt shield: status %d", httpResp.StatusCode)
	}

	var result struct {
		UserPromptAnalysis struct {
			AttackDetected bool `json:"attackDetected"`
		} `json:"userPromptAnalysis"`
	}
	if err := json.NewDecoder(httpResp.Body).Decode(&result); err != nil {
		return Result{}, fmt.Errorf("azure prompt shield: decode: %w", err)
	}

	if result.UserPromptAnalysis.AttackDetected {
		return Result{
			Passed:  false,
			Message: "Prompt injection attack detected by Azure Prompt Shield",
		}, nil
	}

	return Result{Passed: true}, nil
}
