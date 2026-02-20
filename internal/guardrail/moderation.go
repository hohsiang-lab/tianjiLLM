package guardrail

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/praxisllmlab/tianjiLLM/internal/model"
)

// ModerationGuardrail uses OpenAI's moderation endpoint to block harmful content.
type ModerationGuardrail struct {
	apiKey string
	apiURL string
	client *http.Client
}

// NewModerationGuardrail creates a content moderation guardrail.
func NewModerationGuardrail(apiKey, apiURL string) *ModerationGuardrail {
	if apiURL == "" {
		apiURL = "https://api.openai.com/v1/moderations"
	}
	return &ModerationGuardrail{
		apiKey: apiKey,
		apiURL: apiURL,
		client: &http.Client{Timeout: 5 * time.Second},
	}
}

func (m *ModerationGuardrail) Name() string           { return "openai_moderation" }
func (m *ModerationGuardrail) SupportedHooks() []Hook { return []Hook{HookPreCall, HookPostCall} }

type moderationRequest struct {
	Input string `json:"input"`
}

type moderationResponse struct {
	Results []moderationResult `json:"results"`
}

type moderationResult struct {
	Flagged    bool            `json:"flagged"`
	Categories map[string]bool `json:"categories"`
}

func (m *ModerationGuardrail) Run(ctx context.Context, hook Hook, req *model.ChatCompletionRequest, resp *model.ModelResponse) (Result, error) {
	var text string

	switch hook {
	case HookPreCall:
		if req == nil {
			return Result{Passed: true}, nil
		}
		var parts []string
		for _, msg := range req.Messages {
			if s, ok := msg.Content.(string); ok && s != "" {
				parts = append(parts, s)
			}
		}
		text = strings.Join(parts, "\n")

	case HookPostCall:
		if resp == nil {
			return Result{Passed: true}, nil
		}
		var parts []string
		for _, choice := range resp.Choices {
			if choice.Message != nil {
				if s, ok := choice.Message.Content.(string); ok {
					parts = append(parts, s)
				}
			}
		}
		text = strings.Join(parts, "\n")
	}

	if text == "" {
		return Result{Passed: true}, nil
	}

	flagged, categories, err := m.moderate(ctx, text)
	if err != nil {
		return Result{}, err
	}

	if flagged {
		return Result{
			Passed:  false,
			Message: fmt.Sprintf("content flagged by moderation: %s", strings.Join(categories, ", ")),
		}, nil
	}

	return Result{Passed: true}, nil
}

func (m *ModerationGuardrail) moderate(ctx context.Context, text string) (bool, []string, error) {
	body, _ := json.Marshal(moderationRequest{Input: text})

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, m.apiURL, bytes.NewReader(body))
	if err != nil {
		return false, nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+m.apiKey)

	resp, err := m.client.Do(httpReq)
	if err != nil {
		return false, nil, fmt.Errorf("moderation API unavailable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, nil, fmt.Errorf("moderation API returned %d", resp.StatusCode)
	}

	var result moderationResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false, nil, err
	}

	if len(result.Results) == 0 {
		return false, nil, nil
	}

	r := result.Results[0]
	if !r.Flagged {
		return false, nil, nil
	}

	var flaggedCats []string
	for cat, flagged := range r.Categories {
		if flagged {
			flaggedCats = append(flaggedCats, cat)
		}
	}
	return true, flaggedCats, nil
}
