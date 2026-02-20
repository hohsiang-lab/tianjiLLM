package lambdaai

import (
	"github.com/praxisllmlab/tianjiLLM/internal/provider"
	"github.com/praxisllmlab/tianjiLLM/internal/provider/openai"
)

const defaultBaseURL = "https://api.lambdalabs.com/v1"

type Provider struct{ *openai.Provider }

func init() {
	provider.Register("lambda_ai", &Provider{openai.NewWithBaseURL(defaultBaseURL)})
}

func (p *Provider) GetSupportedParams() []string {
	return []string{
		"model", "messages", "temperature", "max_tokens",
		"top_p", "stream", "stream_options", "stop", "n",
		"tools", "tool_choice",
	}
}
