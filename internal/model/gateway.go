package model

import "encoding/json"

// CompletionRequest represents an OpenAI-compatible legacy text completion request.
type CompletionRequest struct {
	Model            string   `json:"model"`
	Prompt           any      `json:"prompt"`
	MaxTokens        *int     `json:"max_tokens,omitempty"`
	Temperature      *float64 `json:"temperature,omitempty"`
	TopP             *float64 `json:"top_p,omitempty"`
	N                *int     `json:"n,omitempty"`
	Stream           *bool    `json:"stream,omitempty"`
	Stop             any      `json:"stop,omitempty"`
	PresencePenalty  *float64 `json:"presence_penalty,omitempty"`
	FrequencyPenalty *float64 `json:"frequency_penalty,omitempty"`
	User             *string  `json:"user,omitempty"`
	Suffix           *string  `json:"suffix,omitempty"`
	Echo             *bool    `json:"echo,omitempty"`
	BestOf           *int     `json:"best_of,omitempty"`
}

// ImageGenerationRequest represents an OpenAI-compatible image generation request.
type ImageGenerationRequest struct {
	Model             string         `json:"model"`
	Prompt            string         `json:"prompt"`
	N                 *int           `json:"n,omitempty"`
	Size              *string        `json:"size,omitempty"`
	Quality           *string        `json:"quality,omitempty"`
	OutputFormat      *string        `json:"output_format,omitempty"`
	Background        *string        `json:"background,omitempty"`
	Moderation        *string        `json:"moderation,omitempty"`
	OutputCompression *int           `json:"output_compression,omitempty"`
	ResponseFormat    *string        `json:"response_format,omitempty"`
	Style             *string        `json:"style,omitempty"`
	User              *string        `json:"user,omitempty"`
	ExtraParams       map[string]any `json:"-"`
}

var knownImageGenerationFields = map[string]bool{
	"model": true, "prompt": true, "n": true, "size": true,
	"quality": true, "output_format": true, "background": true, "moderation": true,
	"output_compression": true, "response_format": true, "style": true, "user": true,
}

func (r *ImageGenerationRequest) UnmarshalJSON(data []byte) error {
	type Alias ImageGenerationRequest
	var alias Alias
	if err := json.Unmarshal(data, &alias); err != nil {
		return err
	}
	*r = ImageGenerationRequest(alias)

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	for key := range raw {
		if knownImageGenerationFields[key] {
			continue
		}
		if r.ExtraParams == nil {
			r.ExtraParams = make(map[string]any)
		}
		var val any
		if err := json.Unmarshal(raw[key], &val); err != nil || val == nil {
			continue
		}
		r.ExtraParams[key] = val
	}
	return nil
}

// ImageGenerationResponse represents an OpenAI-compatible image generation response.
type ImageGenerationResponse struct {
	Created int64       `json:"created"`
	Data    []ImageData `json:"data"`
}

// ImageEditRequest represents an OpenAI-compatible image edit request decoded from multipart form data.
type ImageEditRequest struct {
	Model             string
	Prompt            string
	Images            []ImageEditFile
	Mask              *ImageEditFile
	N                 *int
	Size              *string
	Quality           *string
	OutputFormat      *string
	Background        *string
	OutputCompression *int
	ResponseFormat    *string
	User              *string
	ExtraParams       map[string][]string
}

type ImageEditFile struct {
	Filename    string
	ContentType string
	Data        []byte
}

type ImageData struct {
	URL           string `json:"url,omitempty"`
	B64JSON       string `json:"b64_json,omitempty"`
	RevisedPrompt string `json:"revised_prompt,omitempty"`
}

// AudioTranscriptionRequest is decoded from multipart form data.
type AudioTranscriptionRequest struct {
	Model          string   `json:"model"`
	Language       *string  `json:"language,omitempty"`
	Prompt         *string  `json:"prompt,omitempty"`
	ResponseFormat *string  `json:"response_format,omitempty"`
	Temperature    *float64 `json:"temperature,omitempty"`
}

// AudioTranscriptionResponse represents an OpenAI-compatible transcription response.
type AudioTranscriptionResponse struct {
	Text string `json:"text"`
}

// AudioSpeechRequest represents an OpenAI-compatible TTS request.
type AudioSpeechRequest struct {
	Model          string   `json:"model"`
	Input          string   `json:"input"`
	Voice          string   `json:"voice"`
	ResponseFormat *string  `json:"response_format,omitempty"`
	Speed          *float64 `json:"speed,omitempty"`
}

// ModerationRequest represents an OpenAI-compatible moderation request.
type ModerationRequest struct {
	Model string `json:"model,omitempty"`
	Input any    `json:"input"`
}

// ModerationResponse represents an OpenAI-compatible moderation response.
type ModerationResponse struct {
	ID      string             `json:"id"`
	Model   string             `json:"model"`
	Results []ModerationResult `json:"results"`
}

type ModerationResult struct {
	Flagged        bool               `json:"flagged"`
	Categories     map[string]bool    `json:"categories"`
	CategoryScores map[string]float64 `json:"category_scores"`
}
