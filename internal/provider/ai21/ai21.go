package ai21

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/praxisllmlab/tianjiLLM/internal/model"
	"github.com/praxisllmlab/tianjiLLM/internal/provider"
)

const defaultBaseURL = "https://api.ai21.com/studio/v1"

// Provider implements the AI21 translation layer.
// AI21 is OpenAI-compatible, so minimal transformation is needed.
type Provider struct {
	baseURL string
}

func New() *Provider {
	return &Provider{baseURL: defaultBaseURL}
}

func (p *Provider) TransformRequest(ctx context.Context, req *model.ChatCompletionRequest, apiKey string) (*http.Request, error) {
	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal ai21 request: %w", err)
	}

	url := p.GetRequestURL(req.Model)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("create ai21 request: %w", err)
	}

	p.SetupHeaders(httpReq, apiKey)
	return httpReq, nil
}

func (p *Provider) TransformResponse(_ context.Context, resp *http.Response) (*model.ModelResponse, error) {
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, &model.TianjiError{
			StatusCode: resp.StatusCode,
			Message:    string(body),
			Type:       "api_error",
			Provider:   "ai21",
			Err:        model.MapHTTPStatusToError(resp.StatusCode),
		}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read ai21 response: %w", err)
	}

	var result model.ModelResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse ai21 response: %w", err)
	}

	return &result, nil
}

func (p *Provider) TransformStreamChunk(_ context.Context, data []byte) (*model.StreamChunk, bool, error) {
	var chunk model.StreamChunk
	if err := json.Unmarshal(data, &chunk); err != nil {
		return nil, false, err
	}

	isDone := false
	if len(chunk.Choices) > 0 && chunk.Choices[0].FinishReason != nil {
		isDone = *chunk.Choices[0].FinishReason == "stop"
	}
	return &chunk, isDone, nil
}

func (p *Provider) GetSupportedParams() []string {
	return []string{
		"model", "messages", "temperature", "max_tokens", "top_p",
		"stop", "stream", "n", "tools", "tool_choice",
		"response_format", "seed",
	}
}

func (p *Provider) MapParams(params map[string]any) map[string]any {
	return params // OpenAI-compatible, no mapping needed
}

func (p *Provider) GetRequestURL(modelName string) string {
	return fmt.Sprintf("%s/chat/completions", p.baseURL)
}

func (p *Provider) SetupHeaders(req *http.Request, apiKey string) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
}

func init() {
	provider.Register("ai21", New())
}
