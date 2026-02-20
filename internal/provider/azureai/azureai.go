package azureai

import (
	"net/http"

	"github.com/praxisllmlab/tianjiLLM/internal/provider"
	"github.com/praxisllmlab/tianjiLLM/internal/provider/openai"
)

const defaultBaseURL = "https://models.inference.ai.azure.com"

type Provider struct{ *openai.Provider }

func init() {
	provider.Register("azure_ai", &Provider{openai.NewWithBaseURL(defaultBaseURL)})
}

// SetupHeaders supports both Bearer token and api-key header auth.
func (p *Provider) SetupHeaders(req *http.Request, apiKey string) {
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
}

func (p *Provider) GetSupportedParams() []string {
	return []string{
		"model", "messages", "temperature", "max_tokens",
		"top_p", "frequency_penalty", "presence_penalty", "n", "stop",
		"stream", "stream_options", "seed", "tools", "tool_choice",
		"response_format",
	}
}
