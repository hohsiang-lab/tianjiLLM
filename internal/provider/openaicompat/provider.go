package openaicompat

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"

	"github.com/praxisllmlab/tianjiLLM/internal/model"
	"github.com/praxisllmlab/tianjiLLM/internal/provider/openai"
)

// Provider is a generic OpenAI-compatible provider driven by JSON config.
type Provider struct {
	config SimpleProviderConfig
}

// NewFromConfig creates a Provider from a SimpleProviderConfig.
func NewFromConfig(cfg SimpleProviderConfig) *Provider {
	cfg = normalizeConfig(cfg)
	return &Provider{config: cfg}
}

func normalizeConfig(cfg SimpleProviderConfig) SimpleProviderConfig {
	if cfg.AuthHeader == "" {
		cfg.AuthHeader = "Authorization"
	}
	if cfg.AuthPrefix == "" {
		cfg.AuthPrefix = "Bearer "
	}
	return cfg
}

func (p *Provider) TransformRequest(ctx context.Context, req *model.ChatCompletionRequest, apiKey string) (*http.Request, error) {
	body := buildRequestBody(req, p.config.ParamMappings, p.config.Constraints)

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

func (p *Provider) TransformResponse(ctx context.Context, resp *http.Response) (*model.ModelResponse, error) {
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, parseErrorResponse(resp, p.config.Name)
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

func (p *Provider) TransformStreamChunk(ctx context.Context, data []byte) (*model.StreamChunk, bool, error) {
	return openai.ParseStreamChunk(data)
}

func (p *Provider) GetSupportedParams() []string {
	if len(p.config.SupportedParams) > 0 {
		return p.config.SupportedParams
	}
	return openai.CompatibleSupportedParams()
}

func (p *Provider) MapParams(params map[string]any) map[string]any {
	return applyParamMappings(params, p.config.ParamMappings)
}

func (p *Provider) GetRequestURL(modelName string) string {
	return p.config.BaseURL + "/chat/completions"
}

func (p *Provider) SetupHeaders(req *http.Request, apiKey string) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(p.config.AuthHeader, p.config.AuthPrefix+apiKey)

	for k, v := range p.config.Headers {
		req.Header.Set(k, v)
	}
}

func buildRequestBody(req *model.ChatCompletionRequest, paramMappings map[string]string, constraints []ParamConstraint) map[string]any {
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
	if req.MaxCompletionTokens != nil {
		body["max_completion_tokens"] = *req.MaxCompletionTokens
	}
	if req.TopP != nil {
		body["top_p"] = *req.TopP
	}
	if req.Stream != nil {
		body["stream"] = *req.Stream
	}
	if req.StreamOptions != nil && req.StreamOptions.IncludeUsage {
		body["stream_options"] = req.StreamOptions
	}
	if req.ResponseFormat != nil {
		body["response_format"] = req.ResponseFormat
	}
	if req.Stop != nil {
		body["stop"] = req.Stop
	}
	if len(req.Tools) > 0 {
		body["tools"] = req.Tools
	}
	if req.ToolChoice != nil {
		body["tool_choice"] = req.ToolChoice
	}

	return ApplyConstraints(applyParamMappings(body, paramMappings), constraints)
}

func applyParamMappings(params map[string]any, mappings map[string]string) map[string]any {
	keys := make([]string, 0, len(params))
	for key := range params {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	result := make(map[string]any, len(params))
	for _, source := range keys {
		target := source
		seen := map[string]struct{}{target: {}}
		for {
			next, ok := mappings[target]
			if !ok || next == "" {
				break
			}
			if _, cycle := seen[next]; cycle {
				break
			}
			seen[next] = struct{}{}
			target = next
		}
		result[target] = params[source]
	}
	return result
}

func parseErrorResponse(resp *http.Response, providerName string) error {
	body, _ := io.ReadAll(resp.Body)

	msg := string(body)
	var errResp struct {
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
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
		Provider:   providerName,
		Err:        model.MapHTTPStatusToError(resp.StatusCode),
	}
}
