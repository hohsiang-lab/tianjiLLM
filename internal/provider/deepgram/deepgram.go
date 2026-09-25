package deepgram

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

const defaultBaseURL = "https://api.deepgram.com"

type Provider struct {
	baseURL string
}

func init() {
	provider.Register("deepgram", &Provider{baseURL: defaultBaseURL})
}

type deepgramResponse struct {
	Results struct {
		Channels []struct {
			Alternatives []struct {
				Transcript string  `json:"transcript"`
				Confidence float64 `json:"confidence"`
			} `json:"alternatives"`
		} `json:"channels"`
	} `json:"results"`
}

func (p *Provider) TransformRequest(ctx context.Context, req *model.ChatCompletionRequest, apiKey string) (*http.Request, error) {
	// For STT, the request body is audio data.
	// In the OpenAI-compat flow, audio is passed as raw bytes in the first message content.
	var audioData []byte
	if len(req.Messages) > 0 {
		if s, ok := req.Messages[0].Content.(string); ok {
			audioData = []byte(s)
		}
	}

	url := fmt.Sprintf("%s/v1/listen?model=general&smart_format=true", p.baseURL)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(audioData))
	if err != nil {
		return nil, fmt.Errorf("create deepgram request: %w", err)
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
			Provider:   "deepgram",
			Err:        model.MapHTTPStatusToError(resp.StatusCode),
		}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read deepgram response: %w", err)
	}

	var dgResp deepgramResponse
	if err := json.Unmarshal(body, &dgResp); err != nil {
		return nil, fmt.Errorf("parse deepgram response: %w", err)
	}

	var transcript string
	if len(dgResp.Results.Channels) > 0 && len(dgResp.Results.Channels[0].Alternatives) > 0 {
		transcript = dgResp.Results.Channels[0].Alternatives[0].Transcript
	}

	return &model.ModelResponse{
		Choices: []model.Choice{
			{
				Message: &model.Message{
					Role:    "assistant",
					Content: transcript,
				},
			},
		},
	}, nil
}

func (p *Provider) TransformStreamChunk(_ context.Context, data []byte) (*model.StreamChunk, bool, error) {
	return nil, true, fmt.Errorf("deepgram STT streaming not implemented via this path")
}

func (p *Provider) GetSupportedParams() []string {
	return []string{"model", "messages", "language"}
}

func (p *Provider) MapParams(params map[string]any) map[string]any {
	return params
}

func (p *Provider) GetRequestURL(_ string) string {
	return fmt.Sprintf("%s/v1/listen", p.baseURL)
}

func (p *Provider) SetupHeaders(req *http.Request, apiKey string) {
	req.Header.Set("Authorization", "Token "+apiKey)
	req.Header.Set("Content-Type", "audio/wav")
}
