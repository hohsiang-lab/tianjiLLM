package voyage

import (
	"context"
	"net/http"

	"github.com/praxisllmlab/tianjiLLM/internal/model"
	"github.com/praxisllmlab/tianjiLLM/internal/provider"
	"github.com/praxisllmlab/tianjiLLM/internal/provider/openai"
)

const defaultBaseURL = "https://api.voyageai.com/v1"

type Provider struct{ *openai.Provider }

func init() {
	provider.Register("voyage", &Provider{openai.NewWithBaseURL(defaultBaseURL)})
}

// WithBaseURL preserves Voyage-specific embedding response normalization when
// a deployment supplies a custom api_base.
func (p *Provider) WithBaseURL(baseURL string) provider.Provider {
	return &Provider{openai.NewWithBaseURL(baseURL)}
}

// TransformEmbeddingResponse normalizes Voyage's total-only usage into the
// standard embedding usage fields Tianji exposes to clients.
func (p *Provider) TransformEmbeddingResponse(ctx context.Context, resp *http.Response) (*model.EmbeddingResponse, error) {
	return p.TransformEmbeddingResponseWithTotalOnlyUsage(ctx, resp)
}

func (p *Provider) GetSupportedParams() []string {
	return []string{
		"model", "input", "encoding_format", "input_type",
	}
}
