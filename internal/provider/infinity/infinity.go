package infinity

import (
	"github.com/praxisllmlab/tianjiLLM/internal/provider"
	"github.com/praxisllmlab/tianjiLLM/internal/provider/openai"
)

// infinity has no fixed base URL — it relies on api_base from config.
// GetWithBaseURL in the registry uses the baseURLFactory when api_base is set.
// This registration provides a fallback for discovery/listing purposes.
type Provider struct{ *openai.Provider }

func init() {
	provider.Register("infinity", &Provider{openai.New()})
}

func (p *Provider) GetSupportedParams() []string {
	return []string{
		"model", "messages", "temperature", "max_tokens",
		"top_p", "stream", "stop",
	}
}
