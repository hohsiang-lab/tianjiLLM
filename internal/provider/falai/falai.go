package falai

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

const defaultBaseURL = "https://fal.run"

type Provider struct {
	baseURL string
}

func init() {
	provider.Register("fal_ai", &Provider{baseURL: defaultBaseURL})
}

type falRequest struct {
	Prompt              string `json:"prompt"`
	ImageSize           string `json:"image_size,omitempty"`
	NumImages           int    `json:"num_images,omitempty"`
	EnableSafetyChecker bool   `json:"enable_safety_checker,omitempty"`
}

type falResponse struct {
	Images []struct {
		URL         string `json:"url"`
		ContentType string `json:"content_type"`
		Width       int    `json:"width"`
		Height      int    `json:"height"`
	} `json:"images"`
	RequestID string `json:"request_id"`
}

func (p *Provider) TransformRequest(ctx context.Context, req *model.ChatCompletionRequest, apiKey string) (*http.Request, error) {
	var prompt string
	for _, msg := range req.Messages {
		if s, ok := msg.Content.(string); ok {
			prompt = s
			break
		}
	}

	body := falRequest{
		Prompt:    prompt,
		NumImages: 1,
	}

	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal fal_ai request: %w", err)
	}

	url := fmt.Sprintf("%s/%s", p.baseURL, req.Model)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("create fal_ai request: %w", err)
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
			Provider:   "fal_ai",
			Err:        model.MapHTTPStatusToError(resp.StatusCode),
		}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read fal_ai response: %w", err)
	}

	var falResp falResponse
	if err := json.Unmarshal(body, &falResp); err != nil {
		return nil, fmt.Errorf("parse fal_ai response: %w", err)
	}

	var content string
	if len(falResp.Images) > 0 {
		content = falResp.Images[0].URL
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
	return nil, true, fmt.Errorf("fal_ai does not support streaming chunks")
}

func (p *Provider) GetSupportedParams() []string {
	return []string{"model", "messages", "n", "size"}
}

func (p *Provider) MapParams(params map[string]any) map[string]any {
	return params
}

func (p *Provider) GetRequestURL(modelName string) string {
	return fmt.Sprintf("%s/%s", p.baseURL, modelName)
}

func (p *Provider) SetupHeaders(req *http.Request, apiKey string) {
	req.Header.Set("Authorization", "Key "+apiKey)
	req.Header.Set("Content-Type", "application/json")
}
