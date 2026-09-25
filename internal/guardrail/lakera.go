package guardrail

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/praxisllmlab/tianjiLLM/internal/model"
)

// LakeraGuardrail uses Lakera AI v2 Guard API.
type LakeraGuardrail struct {
	apiKey  string
	baseURL string
}

func (l *LakeraGuardrail) Name() string { return "lakera_guard" }

func (l *LakeraGuardrail) SupportedHooks() []Hook { return []Hook{HookPreCall, HookPostCall} }

func (l *LakeraGuardrail) Run(ctx context.Context, hook Hook, req *model.ChatCompletionRequest, resp *model.ModelResponse) (Result, error) {
	content := extractContent(hook, req, resp)

	if content == "" {
		return Result{Passed: true}, nil
	}

	baseURL := l.baseURL
	if baseURL == "" {
		baseURL = "https://api.lakera.ai"
	}

	body, _ := json.Marshal(map[string]any{
		"input": content,
	})

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/v2/guard", bytes.NewReader(body))
	if err != nil {
		return Result{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+l.apiKey)

	httpResp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return Result{}, fmt.Errorf("lakera guard: %w", err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		return Result{}, fmt.Errorf("lakera guard: status %d", httpResp.StatusCode)
	}

	var result struct {
		Flagged    bool `json:"flagged"`
		Categories []struct {
			Name    string `json:"name"`
			Flagged bool   `json:"flagged"`
		} `json:"categories"`
	}
	if err := json.NewDecoder(httpResp.Body).Decode(&result); err != nil {
		return Result{}, fmt.Errorf("lakera guard: decode: %w", err)
	}

	if result.Flagged {
		var flaggedCats []string
		for _, c := range result.Categories {
			if c.Flagged {
				flaggedCats = append(flaggedCats, c.Name)
			}
		}
		return Result{
			Passed:  false,
			Message: fmt.Sprintf("Content flagged by Lakera: %v", flaggedCats),
		}, nil
	}

	return Result{Passed: true}, nil
}
