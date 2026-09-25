package awspolly

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
)

const defaultBaseURL = "https://polly.us-east-1.amazonaws.com"

type Provider struct {
	baseURL string
}

func init() {
	provider.Register("aws_polly", &Provider{baseURL: defaultBaseURL})
}

type pollyRequest struct {
	OutputFormat string `json:"OutputFormat"`
	Text         string `json:"Text"`
	VoiceId      string `json:"VoiceId"`
	Engine       string `json:"Engine,omitempty"`
}

func (p *Provider) TransformRequest(ctx context.Context, req *model.ChatCompletionRequest, apiKey string) (*http.Request, error) {
	voiceID := req.Model
	if idx := strings.LastIndex(voiceID, "/"); idx >= 0 {
		voiceID = voiceID[idx+1:]
	}

	var text string
	for _, msg := range req.Messages {
		if s, ok := msg.Content.(string); ok {
			text += s + " "
		}
	}
	text = strings.TrimSpace(text)

	body := pollyRequest{
		OutputFormat: "mp3",
		Text:         text,
		VoiceId:      voiceID,
		Engine:       "neural",
	}

	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal aws_polly request: %w", err)
	}

	url := p.baseURL + "/v1/speech"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("create aws_polly request: %w", err)
	}

	p.SetupHeaders(httpReq, apiKey)
	// Note: Real AWS Polly requires SigV4 signing, which should be done by the
	// AWS SDK or a signing middleware. This sets basic headers for the proxy flow.
	return httpReq, nil
}

func (p *Provider) TransformResponse(_ context.Context, resp *http.Response) (*model.ModelResponse, error) {
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, &model.TianjiError{
			StatusCode: resp.StatusCode,
			Message:    string(body),
			Provider:   "aws_polly",
			Err:        model.MapHTTPStatusToError(resp.StatusCode),
		}
	}

	audioData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read aws_polly response: %w", err)
	}

	return &model.ModelResponse{
		Choices: []model.Choice{
			{
				Message: &model.Message{
					Role:    "assistant",
					Content: fmt.Sprintf("[audio:%d bytes]", len(audioData)),
				},
			},
		},
	}, nil
}

func (p *Provider) TransformStreamChunk(_ context.Context, _ []byte) (*model.StreamChunk, bool, error) {
	return nil, true, fmt.Errorf("aws_polly TTS does not support streaming chunks")
}

func (p *Provider) GetSupportedParams() []string {
	return []string{"model", "messages", "voice"}
}

func (p *Provider) MapParams(params map[string]any) map[string]any {
	return params
}

func (p *Provider) GetRequestURL(_ string) string {
	return p.baseURL + "/v1/speech"
}

func (p *Provider) SetupHeaders(req *http.Request, apiKey string) {
	req.Header.Set("Content-Type", "application/json")
	// AWS SigV4 signing would be applied separately; apiKey here is a placeholder
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
}
