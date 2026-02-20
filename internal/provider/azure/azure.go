package azure

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/praxisllmlab/tianjiLLM/internal/model"
	"github.com/praxisllmlab/tianjiLLM/internal/provider"
	"github.com/praxisllmlab/tianjiLLM/internal/provider/openai"
)

const defaultAPIVersion = "2024-10-21"

// Provider implements the Azure OpenAI translation layer.
// Azure OpenAI uses the same request/response format as OpenAI
// but with different URL structure and auth headers.
type Provider struct {
	resourceName string
	apiVersion   string
	apiBase      string // optional custom endpoint
}

func New() *Provider {
	return &Provider{apiVersion: defaultAPIVersion}
}

func NewWithConfig(apiBase, apiVersion string) *Provider {
	return &Provider{
		apiBase:    apiBase,
		apiVersion: apiVersion,
	}
}

func (p *Provider) TransformRequest(ctx context.Context, req *model.ChatCompletionRequest, apiKey string) (*http.Request, error) {
	// Azure uses the same request format as OpenAI
	body := map[string]any{
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
	if req.Stop != nil {
		body["stop"] = req.Stop
	}
	if req.Stream != nil {
		body["stream"] = *req.Stream
	}
	if len(req.Tools) > 0 {
		body["tools"] = req.Tools
	}
	if req.ToolChoice != nil {
		body["tool_choice"] = req.ToolChoice
	}
	if req.ResponseFormat != nil {
		body["response_format"] = req.ResponseFormat
	}
	if req.N != nil {
		body["n"] = *req.N
	}
	if req.Seed != nil {
		body["seed"] = *req.Seed
	}

	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal azure request: %w", err)
	}

	url := p.GetRequestURL(req.Model)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("create azure request: %w", err)
	}

	p.SetupHeaders(httpReq, apiKey)
	return httpReq, nil
}

func (p *Provider) TransformResponse(ctx context.Context, resp *http.Response) (*model.ModelResponse, error) {
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, parseErrorResponse(resp)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read azure response: %w", err)
	}

	var result model.ModelResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse azure response: %w", err)
	}

	return &result, nil
}

func (p *Provider) TransformStreamChunk(ctx context.Context, data []byte) (*model.StreamChunk, bool, error) {
	return openai.ParseStreamChunk(data)
}

func (p *Provider) GetSupportedParams() []string {
	return openai.SupportedParams
}

func (p *Provider) MapParams(params map[string]any) map[string]any {
	return openai.MapOpenAIParams(params)
}

func (p *Provider) GetRequestURL(modelName string) string {
	if p.apiBase != "" {
		// Custom api_base: append /chat/completions?api-version=...
		base := strings.TrimSuffix(p.apiBase, "/")
		if !strings.Contains(base, "chat/completions") {
			base += "/chat/completions"
		}
		if !strings.Contains(base, "api-version") {
			base += "?api-version=" + p.apiVersion
		}
		return base
	}

	// Standard Azure format:
	// https://{resource}.openai.azure.com/openai/deployments/{deployment}/chat/completions?api-version=...
	return fmt.Sprintf("https://%s.openai.azure.com/openai/deployments/%s/chat/completions?api-version=%s",
		p.resourceName, modelName, p.apiVersion)
}

func (p *Provider) SetupHeaders(req *http.Request, apiKey string) {
	req.Header.Set("Content-Type", "application/json")
	// Azure supports both api-key header and Bearer token
	if strings.HasPrefix(apiKey, "Bearer ") {
		req.Header.Set("Authorization", apiKey)
	} else {
		req.Header.Set("api-key", apiKey)
	}
}

func parseErrorResponse(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)

	msg := string(body)
	var errResp struct {
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
			Code    string `json:"code"`
		} `json:"error"`
	}
	errType := "api_error"
	if json.Unmarshal(body, &errResp) == nil && errResp.Error.Message != "" {
		msg = errResp.Error.Message
		errType = errResp.Error.Type
	}

	return &model.TianjiError{
		StatusCode: resp.StatusCode,
		Message:    msg,
		Type:       errType,
		Provider:   "azure",
		Err:        model.MapHTTPStatusToError(resp.StatusCode),
	}
}

func init() {
	provider.Register("azure", New())
}
