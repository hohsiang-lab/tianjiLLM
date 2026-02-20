package cloudflare

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/praxisllmlab/tianjiLLM/internal/model"
	"github.com/praxisllmlab/tianjiLLM/internal/provider"
	"github.com/praxisllmlab/tianjiLLM/internal/provider/openai"
)

const baseURLTemplate = "https://api.cloudflare.com/client/v4/accounts"

// Provider implements the Cloudflare Workers AI translation layer.
// URL format: {baseURL}/{account_id}/ai/run/{model}
type Provider struct{}

func init() {
	provider.Register("cloudflare", &Provider{})
}

func (p *Provider) TransformRequest(ctx context.Context, req *model.ChatCompletionRequest, apiKey string) (*http.Request, error) {
	body := map[string]any{
		"messages": req.Messages,
	}

	if req.MaxTokens != nil {
		body["max_tokens"] = *req.MaxTokens
	}
	if req.Stream != nil {
		body["stream"] = *req.Stream
	}

	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal cloudflare request: %w", err)
	}

	url := p.GetRequestURL(req.Model)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("create cloudflare request: %w", err)
	}

	p.SetupHeaders(httpReq, apiKey)
	return httpReq, nil
}

func (p *Provider) TransformResponse(_ context.Context, resp *http.Response) (*model.ModelResponse, error) {
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, parseErrorResponse(resp)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read cloudflare response: %w", err)
	}

	var result model.ModelResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse cloudflare response: %w", err)
	}

	return &result, nil
}

func (p *Provider) TransformStreamChunk(_ context.Context, data []byte) (*model.StreamChunk, bool, error) {
	return openai.ParseStreamChunk(data)
}

func (p *Provider) GetSupportedParams() []string {
	return []string{
		"model", "messages", "stream", "max_tokens",
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

// GetRequestURL returns the Cloudflare Workers AI endpoint.
// Model format expected: "{account_id}/{model_name}" or just the model with
// account_id provided via api_base in config.
func (p *Provider) GetRequestURL(modelName string) string {
	return fmt.Sprintf("%s/%s/ai/run", baseURLTemplate, modelName)
}

func (p *Provider) SetupHeaders(req *http.Request, apiKey string) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
}

func parseErrorResponse(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)

	msg := string(body)
	var errResp struct {
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if json.Unmarshal(body, &errResp) == nil && len(errResp.Errors) > 0 {
		msg = errResp.Errors[0].Message
	}

	return &model.TianjiError{
		StatusCode: resp.StatusCode,
		Message:    msg,
		Type:       "api_error",
		Provider:   "cloudflare",
		Err:        model.MapHTTPStatusToError(resp.StatusCode),
	}
}
