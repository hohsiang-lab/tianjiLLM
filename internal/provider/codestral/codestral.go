package codestral

import (
	"github.com/praxisllmlab/tianjiLLM/internal/provider"
	"github.com/praxisllmlab/tianjiLLM/internal/provider/openai"
)

const defaultBaseURL = "https://codestral.mistral.ai/v1"

type Provider struct{ *openai.Provider }

func init() {
	provider.Register("codestral", &Provider{openai.NewWithBaseURL(defaultBaseURL)})
}

func (p *Provider) GetSupportedParams() []string {
	return []string{
		"model", "messages", "temperature", "max_tokens", "max_completion_tokens",
		"top_p", "stream", "stream_options", "stop", "n",
		"tools", "tool_choice", "response_format", "seed",
	}
}
