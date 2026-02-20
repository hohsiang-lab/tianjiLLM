package recraft

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

const defaultBaseURL = "https://external.api.recraft.ai/v1"

type Provider struct {
	baseURL string
}

func init() {
	provider.Register("recraft", &Provider{baseURL: defaultBaseURL})
}

type recraftRequest struct {
	Prompt string `json:"prompt"`
	Model  string `json:"model,omitempty"`
	Size   string `json:"size,omitempty"`
	N      int    `json:"n,omitempty"`
	Style  string `json:"style,omitempty"`
}

type recraftResponse struct {
	Data []struct {
		URL     string `json:"url"`
		B64JSON string `json:"b64_json"`
	} `json:"data"`
	Created int64 `json:"created"`
}

func (p *Provider) TransformRequest(ctx context.Context, req *model.ChatCompletionRequest, apiKey string) (*http.Request, error) {
	var prompt string
	for _, msg := range req.Messages {
		if s, ok := msg.Content.(string); ok {
			prompt = s
			break
		}
	}

	body := recraftRequest{
		Prompt: prompt,
		N:      1,
	}

	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal recraft request: %w", err)
	}

	url := p.baseURL + "/images/generations"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("create recraft request: %w", err)
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
			Provider:   "recraft",
			Err:        model.MapHTTPStatusToError(resp.StatusCode),
		}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read recraft response: %w", err)
	}

	var rcResp recraftResponse
	if err := json.Unmarshal(body, &rcResp); err != nil {
		return nil, fmt.Errorf("parse recraft response: %w", err)
	}

	var content string
	if len(rcResp.Data) > 0 {
		if rcResp.Data[0].URL != "" {
			content = rcResp.Data[0].URL
		} else {
			content = fmt.Sprintf("[image:base64:%d chars]", len(rcResp.Data[0].B64JSON))
		}
	}

	return &model.ModelResponse{
		Choices: []model.Choice{
			{
				Message: &model.Message{
					Role:    "assistant",
					Content: content,
				},
			},
		},
	}, nil
}

func (p *Provider) TransformStreamChunk(_ context.Context, _ []byte) (*model.StreamChunk, bool, error) {
	return nil, true, fmt.Errorf("recraft image generation does not support streaming")
}

func (p *Provider) GetSupportedParams() []string {
	return []string{"model", "messages", "n", "size", "style"}
}

func (p *Provider) MapParams(params map[string]any) map[string]any {
	return params
}

func (p *Provider) GetRequestURL(_ string) string {
	return p.baseURL + "/images/generations"
}

func (p *Provider) SetupHeaders(req *http.Request, apiKey string) {
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
}
