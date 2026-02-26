package openai

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

const (
	defaultBaseURL = "https://api.openai.com/v1"
)

// Provider implements the OpenAI provider.
type Provider struct {
	baseURL string
}

// New creates a new OpenAI provider with the default base URL.
func New() *Provider {
	return &Provider{baseURL: defaultBaseURL}
}

// NewWithBaseURL creates a new OpenAI provider with a custom base URL.
func NewWithBaseURL(baseURL string) *Provider {
	return &Provider{baseURL: baseURL}
}

func (p *Provider) TransformRequest(ctx context.Context, req *model.ChatCompletionRequest, apiKey string) (*http.Request, error) {
	body := transformRequestBody(req)

	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	url := p.GetRequestURL(req.Model)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
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
		return nil, fmt.Errorf("read response: %w", err)
	}

	var result model.ModelResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	return &result, nil
}

func (p *Provider) TransformStreamChunk(_ context.Context, data []byte) (*model.StreamChunk, bool, error) {
	return ParseStreamChunk(data)
}

func (p *Provider) GetSupportedParams() []string {
	return SupportedParams
}

func (p *Provider) MapParams(params map[string]any) map[string]any {
	return MapOpenAIParams(params)
}

func (p *Provider) GetRequestURL(modelName string) string {
	return p.baseURL + "/chat/completions"
}

func (p *Provider) SetupHeaders(req *http.Request, apiKey string) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
}

func transformRequestBody(req *model.ChatCompletionRequest) map[string]any {
	body := map[string]any{
		"model":    req.Model,
		"messages": req.Messages,
	}

	if req.Temperature != nil {
		body["temperature"] = *req.Temperature
	}
	if req.MaxTokens != nil {
		body["max_tokens"] = *req.MaxTokens
	}
	if req.TopP != nil {
		body["top_p"] = *req.TopP
	}
	if req.FrequencyPenalty != nil {
		body["frequency_penalty"] = *req.FrequencyPenalty
	}
	if req.PresencePenalty != nil {
		body["presence_penalty"] = *req.PresencePenalty
	}
	if req.N != nil {
		body["n"] = *req.N
	}
	if req.Stop != nil {
		body["stop"] = req.Stop
	}
	if req.User != nil {
		body["user"] = *req.User
	}
	if req.Seed != nil {
		body["seed"] = *req.Seed
	}
	if req.LogProbs != nil {
		body["logprobs"] = *req.LogProbs
	}
	if req.TopLogProbs != nil {
		body["top_logprobs"] = *req.TopLogProbs
	}
	if req.Tools != nil {
		body["tools"] = req.Tools
	}
	if req.ToolChoice != nil {
		body["tool_choice"] = req.ToolChoice
	}
	if req.ResponseFormat != nil {
		body["response_format"] = req.ResponseFormat
	}
	if req.Stream != nil {
		body["stream"] = *req.Stream
	}
	if req.StreamOptions != nil {
		body["stream_options"] = req.StreamOptions
	}
	if len(req.Modalities) > 0 {
		body["modalities"] = req.Modalities
	}

	return body
}

func parseErrorResponse(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)

	var errResp struct {
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
			Code    string `json:"code"`
		} `json:"error"`
	}

	msg := string(body)
	errType := "api_error"
	if json.Unmarshal(body, &errResp) == nil && errResp.Error.Message != "" {
		msg = errResp.Error.Message
		errType = errResp.Error.Type
	}

	return &model.TianjiError{
		StatusCode: resp.StatusCode,
		Message:    msg,
		Type:       errType,
		Provider:   "openai",
		Err:        model.MapHTTPStatusToError(resp.StatusCode),
	}
}

func init() {
	provider.Register("openai", New())
	provider.RegisterBaseURLFactory(func(baseURL string) provider.Provider {
		return NewWithBaseURL(baseURL)
	})
}
