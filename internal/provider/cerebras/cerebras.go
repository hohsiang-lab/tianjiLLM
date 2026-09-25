package cerebras

import (
	"github.com/praxisllmlab/tianjiLLM/internal/provider"
	"github.com/praxisllmlab/tianjiLLM/internal/provider/openai"
)

const defaultBaseURL = "https://api.cerebras.ai/v1"

type Provider struct{ *openai.Provider }

func init() {
	provider.Register("cerebras", &Provider{openai.NewWithBaseURL(defaultBaseURL)})
}

func (p *Provider) GetSupportedParams() []string {
	return []string{
		"model", "messages", "temperature", "max_tokens", "max_completion_tokens",
		"top_p", "stop", "stream", "seed", "tools", "tool_choice",
		"response_format", "user",
	}
}
