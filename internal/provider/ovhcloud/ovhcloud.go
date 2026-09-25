package ovhcloud

import (
	"github.com/praxisllmlab/tianjiLLM/internal/provider"
	"github.com/praxisllmlab/tianjiLLM/internal/provider/openai"
)

const defaultBaseURL = "https://llm.api.cloud.ovh.net/v1"

type Provider struct{ *openai.Provider }

func init() {
	provider.Register("ovhcloud", &Provider{openai.NewWithBaseURL(defaultBaseURL)})
}

func (p *Provider) GetSupportedParams() []string {
	return []string{
		"model", "messages", "temperature", "max_tokens",
		"top_p", "stream", "stop",
	}
}
