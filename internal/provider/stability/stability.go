package stability

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

const defaultBaseURL = "https://api.stability.ai"

type Provider struct {
	baseURL string
}

func init() {
	provider.Register("stability", &Provider{baseURL: defaultBaseURL})
}

type stabilityRequest struct {
	TextPrompts []textPrompt `json:"text_prompts"`
	CfgScale    float64      `json:"cfg_scale,omitempty"`
	Height      int          `json:"height,omitempty"`
	Width       int          `json:"width,omitempty"`
	Samples     int          `json:"samples,omitempty"`
	Steps       int          `json:"steps,omitempty"`
}

type textPrompt struct {
	Text   string  `json:"text"`
	Weight float64 `json:"weight,omitempty"`
}

type stabilityResponse struct {
	Artifacts []struct {
		Base64       string `json:"base64"`
		FinishReason string `json:"finishReason"`
		Seed         int64  `json:"seed"`
	} `json:"artifacts"`
}

func (p *Provider) TransformRequest(ctx context.Context, req *model.ChatCompletionRequest, apiKey string) (*http.Request, error) {
	var prompt string
	for _, msg := range req.Messages {
		if s, ok := msg.Content.(string); ok {
			prompt = s
			break
		}
	}

	engine := req.Model
	body := stabilityRequest{
		TextPrompts: []textPrompt{{Text: prompt, Weight: 1.0}},
		CfgScale:    7.0,
		Samples:     1,
		Steps:       30,
	}

	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal stability request: %w", err)
	}

	url := fmt.Sprintf("%s/v1/generation/%s/text-to-image", p.baseURL, engine)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("create stability request: %w", err)
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
			Provider:   "stability",
			Err:        model.MapHTTPStatusToError(resp.StatusCode),
		}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read stability response: %w", err)
	}

	var stabResp stabilityResponse
	if err := json.Unmarshal(body, &stabResp); err != nil {
		return nil, fmt.Errorf("parse stability response: %w", err)
	}

	var content string
	if len(stabResp.Artifacts) > 0 {
		content = fmt.Sprintf("[image:base64:%d chars]", len(stabResp.Artifacts[0].Base64))
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
	return nil, true, fmt.Errorf("stability image generation does not support streaming")
}

func (p *Provider) GetSupportedParams() []string {
	return []string{"model", "messages", "n", "size"}
}

func (p *Provider) MapParams(params map[string]any) map[string]any {
	return params
}

func (p *Provider) GetRequestURL(modelName string) string {
	return fmt.Sprintf("%s/v1/generation/%s/text-to-image", p.baseURL, modelName)
}

func (p *Provider) SetupHeaders(req *http.Request, apiKey string) {
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
}
