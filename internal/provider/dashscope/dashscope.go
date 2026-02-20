package dashscope

import (
	"github.com/praxisllmlab/tianjiLLM/internal/provider"
	"github.com/praxisllmlab/tianjiLLM/internal/provider/openai"
)

const defaultBaseURL = "https://dashscope.aliyuncs.com/compatible-mode/v1"

type Provider struct{ *openai.Provider }

func init() {
	provider.Register("dashscope", &Provider{openai.NewWithBaseURL(defaultBaseURL)})
}

func (p *Provider) GetSupportedParams() []string {
	return []string{
		"model", "messages", "temperature", "max_tokens",
		"top_p", "frequency_penalty", "presence_penalty", "stop",
		"stream", "stream_options", "seed", "tools", "tool_choice",
		"response_format",
	}
}
