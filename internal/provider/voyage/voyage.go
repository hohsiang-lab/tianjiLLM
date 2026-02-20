package voyage

import (
	"github.com/praxisllmlab/tianjiLLM/internal/provider"
	"github.com/praxisllmlab/tianjiLLM/internal/provider/openai"
)

const defaultBaseURL = "https://api.voyageai.com/v1"

type Provider struct{ *openai.Provider }

func init() {
	provider.Register("voyage", &Provider{openai.NewWithBaseURL(defaultBaseURL)})
}

func (p *Provider) GetSupportedParams() []string {
	return []string{
		"model", "input", "encoding_format", "input_type",
	}
}
