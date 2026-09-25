package gigachat

import (
	"net/http"

	"github.com/praxisllmlab/tianjiLLM/internal/provider"
	"github.com/praxisllmlab/tianjiLLM/internal/provider/openai"
)

const defaultBaseURL = "https://gigachat.devices.sberbank.ru/api/v1"

type Provider struct{ *openai.Provider }

func init() {
	provider.Register("gigachat", &Provider{openai.NewWithBaseURL(defaultBaseURL)})
}

func (p *Provider) SetupHeaders(req *http.Request, apiKey string) {
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
}

func (p *Provider) GetSupportedParams() []string {
	return []string{
		"model", "messages", "temperature", "max_tokens",
		"top_p", "stream", "n", "repetition_penalty",
	}
}
