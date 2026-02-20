package jina

import (
	"github.com/praxisllmlab/tianjiLLM/internal/provider"
	"github.com/praxisllmlab/tianjiLLM/internal/provider/openai"
)

const defaultBaseURL = "https://api.jina.ai/v1"

type Provider struct{ *openai.Provider }

func init() {
	provider.Register("jina_ai", &Provider{openai.NewWithBaseURL(defaultBaseURL)})
}

func (p *Provider) GetSupportedParams() []string {
	return []string{
		"model", "input", "encoding_format",
	}
}
