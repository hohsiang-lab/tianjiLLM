package github

import (
	"net/http"

	"github.com/praxisllmlab/tianjiLLM/internal/provider"
	"github.com/praxisllmlab/tianjiLLM/internal/provider/openai"
)

const defaultBaseURL = "https://models.inference.ai.azure.com"

// Provider wraps the OpenAI provider for GitHub Models API.
// GitHub Models uses an OpenAI-compatible API at models.inference.ai.azure.com.
type Provider struct{ *openai.Provider }

func init() {
	provider.Register("github", &Provider{openai.NewWithBaseURL(defaultBaseURL)})
}

// SetupHeaders uses Bearer token auth with GitHub personal access token.
func (p *Provider) SetupHeaders(req *http.Request, apiKey string) {
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	req.Header.Set("Content-Type", "application/json")
}

func (p *Provider) GetSupportedParams() []string {
	return []string{
		"model", "messages", "temperature", "max_tokens", "max_completion_tokens",
		"top_p", "frequency_penalty", "presence_penalty", "n", "stop",
		"stream", "stream_options", "seed", "tools", "tool_choice",
		"response_format", "logprobs", "top_logprobs", "user",
	}
}
