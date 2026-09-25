package elevenlabs

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

const defaultBaseURL = "https://api.elevenlabs.io"

type Provider struct {
	baseURL string
}

func init() {
	provider.Register("elevenlabs", &Provider{baseURL: defaultBaseURL})
}

// elevenlabsRequest maps OpenAI audio/speech format to ElevenLabs TTS.
type elevenlabsRequest struct {
	Text          string         `json:"text"`
	ModelID       string         `json:"model_id,omitempty"`
	VoiceSettings *voiceSettings `json:"voice_settings,omitempty"`
}

type voiceSettings struct {
	Stability       float64 `json:"stability"`
	SimilarityBoost float64 `json:"similarity_boost"`
}

func (p *Provider) TransformRequest(ctx context.Context, req *model.ChatCompletionRequest, apiKey string) (*http.Request, error) {
	// Extract voice from model name: elevenlabs/{voice_id} or just use model as voice_id
	voiceID := req.Model
	if idx := strings.LastIndex(voiceID, "/"); idx >= 0 {
		voiceID = voiceID[idx+1:]
	}

	// Build text from messages
	var text string
	for _, msg := range req.Messages {
		if s, ok := msg.Content.(string); ok {
			text += s + " "
		}
	}
	text = strings.TrimSpace(text)

	body := elevenlabsRequest{
		Text:    text,
		ModelID: "eleven_monolingual_v1",
	}

	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal elevenlabs request: %w", err)
	}

	url := fmt.Sprintf("%s/v1/text-to-speech/%s", p.baseURL, voiceID)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("create elevenlabs request: %w", err)
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
			Provider:   "elevenlabs",
			Err:        model.MapHTTPStatusToError(resp.StatusCode),
		}
	}

	// ElevenLabs returns raw audio bytes — pass through as base64 or raw
	audioData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read elevenlabs response: %w", err)
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

func (p *Provider) TransformStreamChunk(_ context.Context, data []byte) (*model.StreamChunk, bool, error) {
	return nil, true, fmt.Errorf("elevenlabs TTS does not support streaming chunks")
}

func (p *Provider) GetSupportedParams() []string {
	return []string{"model", "messages", "voice"}
}

func (p *Provider) MapParams(params map[string]any) map[string]any {
	return params
}

func (p *Provider) GetRequestURL(modelName string) string {
	voiceID := modelName
	if idx := strings.LastIndex(voiceID, "/"); idx >= 0 {
		voiceID = voiceID[idx+1:]
	}
	return fmt.Sprintf("%s/v1/text-to-speech/%s", p.baseURL, voiceID)
}

func (p *Provider) SetupHeaders(req *http.Request, apiKey string) {
	req.Header.Set("xi-api-key", apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "audio/mpeg")
}
