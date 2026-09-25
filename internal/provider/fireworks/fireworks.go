package fireworks

import (
	"github.com/praxisllmlab/tianjiLLM/internal/provider"
	"github.com/praxisllmlab/tianjiLLM/internal/provider/openai"
)

const defaultBaseURL = "https://api.fireworks.ai/inference/v1"

type Provider struct{ *openai.Provider }

func init() {
	provider.Register("fireworks_ai", &Provider{openai.NewWithBaseURL(defaultBaseURL)})
}

func (p *Provider) GetSupportedParams() []string {
	return []string{
		"model", "messages", "temperature", "max_tokens", "max_completion_tokens",
		"top_p", "frequency_penalty", "presence_penalty", "n", "stop",
		"stream", "tools", "tool_choice", "response_format", "user",
		"logprobs", "top_k",
	}
}
