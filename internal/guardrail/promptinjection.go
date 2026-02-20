package guardrail

import (
	"context"
	"strings"

	"github.com/praxisllmlab/tianjiLLM/internal/model"
)

// PromptInjectionGuardrail detects prompt injection attempts using pattern matching.
// For production use, consider integrating with a dedicated ML-based detector.
type PromptInjectionGuardrail struct {
	patterns []string
}

// NewPromptInjectionGuardrail creates a prompt injection detection guardrail.
func NewPromptInjectionGuardrail(customPatterns []string) *PromptInjectionGuardrail {
	patterns := []string{
		"ignore previous instructions",
		"ignore all previous",
		"disregard previous",
		"forget previous instructions",
		"forget your instructions",
		"override your instructions",
		"you are now",
		"new instructions:",
		"system prompt:",
		"act as",
		"pretend you are",
		"roleplay as",
		"jailbreak",
		"do anything now",
		"developer mode",
	}
	if len(customPatterns) > 0 {
		patterns = append(patterns, customPatterns...)
	}
	return &PromptInjectionGuardrail{patterns: patterns}
}

func (p *PromptInjectionGuardrail) Name() string           { return "prompt_injection" }
func (p *PromptInjectionGuardrail) SupportedHooks() []Hook { return []Hook{HookPreCall} }

func (p *PromptInjectionGuardrail) Run(_ context.Context, hook Hook, req *model.ChatCompletionRequest, _ *model.ModelResponse) (Result, error) {
	if hook != HookPreCall || req == nil {
		return Result{Passed: true}, nil
	}

	for _, msg := range req.Messages {
		content, ok := msg.Content.(string)
		if !ok || content == "" {
			continue
		}
		lower := strings.ToLower(content)
		for _, pattern := range p.patterns {
			if strings.Contains(lower, pattern) {
				return Result{
					Passed:  false,
					Message: "potential prompt injection detected",
				}, nil
			}
		}
	}

	return Result{Passed: true}, nil
}
