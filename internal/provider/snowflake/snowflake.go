package snowflake

import (
	"net/http"

	"github.com/praxisllmlab/tianjiLLM/internal/provider"
	"github.com/praxisllmlab/tianjiLLM/internal/provider/openai"
)

// Snowflake Cortex uses OpenAI-compatible API at account-specific URLs.
// Base URL format: https://<account>.snowflakecomputing.com/api/v2/cortex/inference:chat
type Provider struct{ *openai.Provider }

func init() {
	// Users must set api_base in config to their Snowflake account URL.
	provider.Register("snowflake", &Provider{openai.NewWithBaseURL("")})
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
		"top_p", "stop", "stream", "tools", "tool_choice",
		"response_format",
	}
}
