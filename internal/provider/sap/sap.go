package sap

import (
	"net/http"

	"github.com/praxisllmlab/tianjiLLM/internal/provider"
	"github.com/praxisllmlab/tianjiLLM/internal/provider/openai"
)

// SAP AI Core uses OpenAI-compatible API with OAuth Bearer tokens.
// Base URL format: https://<host>/v2/inference/deployments/<deployment_id>/chat/completions
type Provider struct{ *openai.Provider }

func init() {
	// Users must set api_base in config to their SAP deployment URL.
	provider.Register("sap", &Provider{openai.NewWithBaseURL("")})
}

func (p *Provider) SetupHeaders(req *http.Request, apiKey string) {
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	req.Header.Set("AI-Resource-Group", "default")
	req.Header.Set("Content-Type", "application/json")
}

func (p *Provider) GetSupportedParams() []string {
	return []string{
		"model", "messages", "temperature", "max_tokens",
		"top_p", "frequency_penalty", "presence_penalty", "n", "stop",
		"stream", "tools", "tool_choice", "response_format",
	}
}
