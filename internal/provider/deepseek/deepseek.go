package deepseek

import (
	"github.com/praxisllmlab/tianjiLLM/internal/provider"
	"github.com/praxisllmlab/tianjiLLM/internal/provider/openai"
)

const defaultBaseURL = "https://api.deepseek.com/beta"

type Provider struct{ *openai.Provider }

func init() {
	provider.Register("deepseek", &Provider{openai.NewWithBaseURL(defaultBaseURL)})
}

func (p *Provider) GetSupportedParams() []string {
	return []string{
		"model", "messages", "temperature", "max_tokens", "max_completion_tokens",
		"top_p", "frequency_penalty", "presence_penalty", "n", "stop",
		"stream", "stream_options", "seed", "tools", "tool_choice",
		"response_format", "logprobs", "top_logprobs", "user",
	}
}
