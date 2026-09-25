package volcengine

import (
	"github.com/praxisllmlab/tianjiLLM/internal/provider"
	"github.com/praxisllmlab/tianjiLLM/internal/provider/openai"
)

const defaultBaseURL = "https://ark.cn-beijing.volces.com/api/v3"

type Provider struct{ *openai.Provider }

func init() {
	provider.Register("volcengine", &Provider{openai.NewWithBaseURL(defaultBaseURL)})
}

func (p *Provider) GetSupportedParams() []string {
	return []string{
		"model", "messages", "temperature", "max_tokens",
		"top_p", "frequency_penalty", "presence_penalty", "stop",
		"stream", "stream_options", "seed", "tools", "tool_choice",
		"response_format",
	}
}
