package guardrail

import (
	"context"
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/model"
)

func BenchmarkPromptInjection(b *testing.B) {
	g := NewPromptInjectionGuardrail(nil)
	req := &model.ChatCompletionRequest{
		Messages: []model.Message{
			{Role: "user", Content: "Tell me about the weather today in New York"},
		},
	}

	b.ResetTimer()
	for range b.N {
		_, _ = g.Run(context.Background(), HookPreCall, req, nil)
	}
}

func BenchmarkContentFilter(b *testing.B) {
	g := NewContentFilter(2)
	req := &model.ChatCompletionRequest{
		Messages: []model.Message{
			{Role: "user", Content: "What is machine learning and how does it work?"},
		},
	}

	b.ResetTimer()
	for range b.N {
		_, _ = g.Run(context.Background(), HookPreCall, req, nil)
	}
}

func BenchmarkRegistryRunPreCall(b *testing.B) {
	r := NewRegistry()
	r.Register(NewPromptInjectionGuardrail(nil))
	r.Register(NewContentFilter(2))

	req := &model.ChatCompletionRequest{
		Messages: []model.Message{
			{Role: "user", Content: "Normal conversation about programming"},
		},
	}

	names := []string{"prompt_injection", "content_filter"}

	b.ResetTimer()
	for range b.N {
		_, _ = r.RunPreCall(context.Background(), names, req)
	}
}
