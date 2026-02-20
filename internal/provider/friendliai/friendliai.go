package friendliai

import (
	"github.com/praxisllmlab/tianjiLLM/internal/provider"
	"github.com/praxisllmlab/tianjiLLM/internal/provider/openai"
)

const defaultBaseURL = "https://inference.friendli.ai/v1"

type Provider struct{ *openai.Provider }

func init() {
	provider.Register("friendliai", &Provider{openai.NewWithBaseURL(defaultBaseURL)})
}

func (p *Provider) GetSupportedParams() []string {
	return []string{
		"model", "messages", "temperature", "max_tokens",
		"top_p", "stream", "stream_options", "stop", "n",
		"frequency_penalty", "presence_penalty",
		"tools", "tool_choice", "response_format",
	}
}
