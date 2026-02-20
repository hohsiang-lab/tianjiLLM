package oci

import (
	"net/http"

	"github.com/praxisllmlab/tianjiLLM/internal/provider"
	"github.com/praxisllmlab/tianjiLLM/internal/provider/openai"
)

// OCI Generative AI uses OpenAI-compatible API.
// Base URL format: https://inference.generativeai.<region>.oci.oraclecloud.com/20231130/actions/chat
type Provider struct{ *openai.Provider }

func init() {
	provider.Register("oci", &Provider{openai.NewWithBaseURL("")})
}

func (p *Provider) SetupHeaders(req *http.Request, apiKey string) {
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	req.Header.Set("Content-Type", "application/json")
}

func (p *Provider) GetSupportedParams() []string {
	return []string{
		"model", "messages", "temperature", "max_tokens",
		"top_p", "frequency_penalty", "presence_penalty", "stop",
		"stream", "tools", "tool_choice",
	}
}
