package replicate

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

const defaultBaseURL = "https://api.replicate.com/v1"

// Provider implements the Replicate translation layer.
// Replicate uses "Token" auth (not Bearer) and wraps inputs in a predictions API.
type Provider struct {
	baseURL string
}

func init() {
	provider.Register("replicate", &Provider{baseURL: defaultBaseURL})
}

func (p *Provider) TransformRequest(ctx context.Context, req *model.ChatCompletionRequest, apiKey string) (*http.Request, error) {
	body := p.transformRequestBody(req)

	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal replicate request: %w", err)
	}

	url := p.GetRequestURL(req.Model)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("create replicate request: %w", err)
	}

	p.SetupHeaders(httpReq, apiKey)
	return httpReq, nil
}

func (p *Provider) TransformResponse(_ context.Context, resp *http.Response) (*model.ModelResponse, error) {
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, parseErrorResponse(resp)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read replicate response: %w", err)
	}

	var result model.ModelResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse replicate response: %w", err)
	}

	return &result, nil
}

func (p *Provider) TransformStreamChunk(_ context.Context, data []byte) (*model.StreamChunk, bool, error) {
	return openai.ParseStreamChunk(data)
}

func (p *Provider) GetSupportedParams() []string {
	return []string{
		"model", "messages", "temperature", "max_tokens",
		"top_p", "stop", "stream", "seed",
		"tools", "tool_choice",
	}
}

func (p *Provider) MapParams(params map[string]any) map[string]any {
	result := make(map[string]any, len(params))
	for k, v := range params {
		switch k {
		case "max_tokens":
			result["max_new_tokens"] = v
		case "stop":
			result["stop_sequences"] = v
		default:
			result[k] = v
		}
	}
	return result
}

func (p *Provider) GetRequestURL(modelName string) string {
	return fmt.Sprintf("%s/models/%s/predictions", p.baseURL, modelName)
}

func (p *Provider) SetupHeaders(req *http.Request, apiKey string) {
	req.Header.Set("Content-Type", "application/json")
	// Replicate uses "Token" auth, not "Bearer"
	req.Header.Set("Authorization", "Token "+apiKey)
}

func (p *Provider) transformRequestBody(req *model.ChatCompletionRequest) map[string]any {
	input := map[string]any{
		"prompt": req.Messages,
	}

	if req.Temperature != nil {
		input["temperature"] = *req.Temperature
	}
	if req.MaxTokens != nil {
		input["max_new_tokens"] = *req.MaxTokens
	}
	if req.TopP != nil {
		input["top_p"] = *req.TopP
	}
	if req.Stop != nil {
		input["stop_sequences"] = req.Stop
	}
	if req.Seed != nil {
		input["seed"] = *req.Seed
	}

	body := map[string]any{
		"input": input,
	}

	if req.Stream != nil && *req.Stream {
		body["stream"] = true
	}

	return body
}

func parseErrorResponse(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)

	msg := string(body)
	var errResp struct {
		Detail string `json:"detail"`
	}
	if json.Unmarshal(body, &errResp) == nil && errResp.Detail != "" {
		msg = errResp.Detail
	}

	return &model.TianjiError{
		StatusCode: resp.StatusCode,
		Message:    msg,
		Type:       "api_error",
		Provider:   "replicate",
		Err:        model.MapHTTPStatusToError(resp.StatusCode),
	}
}
