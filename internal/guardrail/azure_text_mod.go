package guardrail

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/praxisllmlab/tianjiLLM/internal/model"
)

// AzureTextModeration uses Azure Content Safety text:analyze API.
type AzureTextModeration struct {
	endpoint  string
	apiKey    string
	threshold int // severity threshold 0-7, block if >= threshold
}

func (a *AzureTextModeration) Name() string { return "azure_text_moderation" }

func (a *AzureTextModeration) SupportedHooks() []Hook { return []Hook{HookPreCall, HookPostCall} }

func (a *AzureTextModeration) Run(ctx context.Context, hook Hook, req *model.ChatCompletionRequest, resp *model.ModelResponse) (Result, error) {
	content := extractContent(hook, req, resp)

	if content == "" {
		return Result{Passed: true}, nil
	}

	body, _ := json.Marshal(map[string]any{
		"text":       content,
		"categories": []string{"Hate", "SelfHarm", "Sexual", "Violence"},
	})

	url := fmt.Sprintf("%s/contentsafety/text:analyze?api-version=2024-09-01", a.endpoint)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return Result{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Ocp-Apim-Subscription-Key", a.apiKey)

	httpResp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return Result{}, fmt.Errorf("azure text moderation: %w", err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		return Result{}, fmt.Errorf("azure text moderation: status %d", httpResp.StatusCode)
	}

	var result struct {
		CategoriesAnalysis []struct {
			Category string `json:"category"`
			Severity int    `json:"severity"`
		} `json:"categoriesAnalysis"`
	}
	if err := json.NewDecoder(httpResp.Body).Decode(&result); err != nil {
		return Result{}, fmt.Errorf("azure text moderation: decode: %w", err)
	}

	threshold := a.threshold
	if threshold == 0 {
		threshold = 2
	}

	for _, cat := range result.CategoriesAnalysis {
		if cat.Severity >= threshold {
			return Result{
				Passed:  false,
				Message: fmt.Sprintf("Content flagged: %s (severity %d)", cat.Category, cat.Severity),
			}, nil
		}
	}

	return Result{Passed: true}, nil
}
