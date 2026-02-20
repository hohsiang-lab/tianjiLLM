package githubcopilot

import (
	"net/http"

	"github.com/praxisllmlab/tianjiLLM/internal/provider"
	"github.com/praxisllmlab/tianjiLLM/internal/provider/openai"
)

const defaultBaseURL = "https://api.githubcopilot.com"

type Provider struct{ *openai.Provider }

func init() {
	provider.Register("github_copilot", &Provider{openai.NewWithBaseURL(defaultBaseURL)})
}

// SetupHeaders uses Bearer token auth (GitHub token or Copilot session token).
func (p *Provider) SetupHeaders(req *http.Request, apiKey string) {
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	req.Header.Set("Editor-Version", "tianjiLLM/1.0")
	req.Header.Set("Copilot-Integration-Id", "tianjiLLM")
}

func (p *Provider) GetSupportedParams() []string {
	return []string{
		"model", "messages", "temperature", "max_tokens",
		"top_p", "n", "stop", "stream", "stream_options",
	}
}
