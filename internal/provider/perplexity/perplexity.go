package perplexity

import (
	"github.com/praxisllmlab/tianjiLLM/internal/provider"
	"github.com/praxisllmlab/tianjiLLM/internal/provider/openai"
)

const defaultBaseURL = "https://api.perplexity.ai"

type Provider struct{ *openai.Provider }

func init() {
	provider.Register("perplexity", &Provider{openai.NewWithBaseURL(defaultBaseURL)})
}

func (p *Provider) GetSupportedParams() []string {
	return []string{
		"model", "messages", "temperature", "max_tokens", "max_completion_tokens",
		"top_p", "frequency_penalty", "presence_penalty",
		"stream", "response_format",
	}
}
