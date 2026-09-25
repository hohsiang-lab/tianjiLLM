package moonshot

import (
	"github.com/praxisllmlab/tianjiLLM/internal/provider"
	"github.com/praxisllmlab/tianjiLLM/internal/provider/openai"
)

const defaultBaseURL = "https://api.moonshot.cn/v1"

type Provider struct{ *openai.Provider }

func init() {
	provider.Register("moonshot", &Provider{openai.NewWithBaseURL(defaultBaseURL)})
}

func (p *Provider) GetSupportedParams() []string {
	return []string{
		"model", "messages", "temperature", "max_tokens",
		"top_p", "frequency_penalty", "presence_penalty", "n", "stop",
		"stream", "stream_options", "tools", "tool_choice",
		"response_format",
	}
}
