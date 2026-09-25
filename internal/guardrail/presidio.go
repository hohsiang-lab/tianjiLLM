package guardrail

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"time"

	"github.com/praxisllmlab/tianjiLLM/internal/model"
)

// PresidioGuardrail detects and redacts PII entities using a Presidio analyzer service.
type PresidioGuardrail struct {
	analyzerURL string
	entities    []string
	client      *http.Client
	redactWith  string
}

// NewPresidioGuardrail creates a PII detection guardrail.
func NewPresidioGuardrail(analyzerURL string, entities []string) *PresidioGuardrail {
	if len(entities) == 0 {
		entities = []string{"PHONE_NUMBER", "EMAIL_ADDRESS", "CREDIT_CARD", "US_SSN", "PERSON", "LOCATION"}
	}
	return &PresidioGuardrail{
		analyzerURL: analyzerURL,
		entities:    entities,
		client:      &http.Client{Timeout: 5 * time.Second},
		redactWith:  "<REDACTED>",
	}
}

func (p *PresidioGuardrail) Name() string           { return "presidio" }
func (p *PresidioGuardrail) SupportedHooks() []Hook { return []Hook{HookPreCall} }

type presidioRequest struct {
	Text     string   `json:"text"`
	Language string   `json:"language"`
	Entities []string `json:"entities,omitempty"`
}

type presidioResult struct {
	EntityType string  `json:"entity_type"`
	Start      int     `json:"start"`
	End        int     `json:"end"`
	Score      float64 `json:"score"`
}

func (p *PresidioGuardrail) Run(ctx context.Context, hook Hook, req *model.ChatCompletionRequest, _ *model.ModelResponse) (Result, error) {
	if hook != HookPreCall || req == nil {
		return Result{Passed: true}, nil
	}

	modified := *req
	messages := make([]model.Message, len(req.Messages))
	copy(messages, req.Messages)
	modified.Messages = messages

	anyRedacted := false
	for i, msg := range modified.Messages {
		content, ok := msg.Content.(string)
		if !ok || content == "" {
			continue
		}
		redacted, found, err := p.analyze(ctx, content)
		if err != nil {
			return Result{}, err
		}
		if found {
			modified.Messages[i].Content = redacted
			anyRedacted = true
		}
	}

	if anyRedacted {
		return Result{Passed: true, Message: "PII detected and redacted", ModifiedRequest: &modified}, nil
	}
	return Result{Passed: true}, nil
}

func (p *PresidioGuardrail) analyze(ctx context.Context, text string) (string, bool, error) {
	body, _ := json.Marshal(presidioRequest{Text: text, Language: "en", Entities: p.entities})

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.analyzerURL+"/analyze", bytes.NewReader(body))
	if err != nil {
		return "", false, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return "", false, fmt.Errorf("presidio analyzer unavailable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", false, fmt.Errorf("presidio returned %d", resp.StatusCode)
	}

	var results []presidioResult
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return "", false, err
	}

	if len(results) == 0 {
		return text, false, nil
	}

	// Sort by start descending — replace from end to start so indices stay valid
	sort.Slice(results, func(i, j int) bool {
		return results[i].Start > results[j].Start
	})

	out := []byte(text)
	replacement := []byte(p.redactWith)
	for _, r := range results {
		if r.Start >= 0 && r.End <= len(out) && r.Start < r.End {
			out = append(out[:r.Start], append(replacement, out[r.End:]...)...)
		}
	}
	return string(out), true, nil
}
