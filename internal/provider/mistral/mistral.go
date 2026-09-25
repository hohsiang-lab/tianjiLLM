package mistral

import (
	"context"
	"net/http"

	"github.com/praxisllmlab/tianjiLLM/internal/model"
	"github.com/praxisllmlab/tianjiLLM/internal/provider"
	"github.com/praxisllmlab/tianjiLLM/internal/provider/openai"
)

const defaultBaseURL = "https://api.mistral.ai/v1"

type Provider struct{ *openai.Provider }

func (p *Provider) SupportsStandardOpenAIChatCapabilities() bool { return true }

func (p *Provider) TransformRequest(ctx context.Context, req *model.ChatCompletionRequest, apiKey string) (*http.Request, error) {
	return p.Provider.TransformRequest(ctx, provider.WithMaxTokensAlias(req), apiKey)
}

func init() {
	provider.Register("mistral", &Provider{openai.NewWithBaseURL(defaultBaseURL)})
}

func (p *Provider) GetSupportedParams() []string {
	return []string{
		"model", "messages", "temperature", "max_tokens", "max_completion_tokens",
		"top_p", "stop", "stream", "seed", "tools", "tool_choice",
		"response_format", "parallel_tool_calls",
	}
}

func (p *Provider) MapParams(params map[string]any) map[string]any {
	result := make(map[string]any, len(params))
	for k, v := range params {
		switch k {
		case "max_completion_tokens":
			result["max_tokens"] = v
		default:
			result[k] = v
		}
	}
	return result
}
