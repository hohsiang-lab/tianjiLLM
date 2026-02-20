package model

import "encoding/json"

// ChatCompletionRequest represents an OpenAI-compatible chat completion request.
type ChatCompletionRequest struct {
	Model            string         `json:"model"`
	Messages         []Message      `json:"messages"`
	Temperature      *float64       `json:"temperature,omitempty"`
	MaxTokens        *int           `json:"max_tokens,omitempty"`
	TopP             *float64       `json:"top_p,omitempty"`
	FrequencyPenalty *float64       `json:"frequency_penalty,omitempty"`
	PresencePenalty  *float64       `json:"presence_penalty,omitempty"`
	Tools            []Tool         `json:"tools,omitempty"`
	ToolChoice       any            `json:"tool_choice,omitempty"`
	ResponseFormat   any            `json:"response_format,omitempty"`
	Stream           *bool          `json:"stream,omitempty"`
	StreamOptions    *StreamOptions `json:"stream_options,omitempty"`
	N                *int           `json:"n,omitempty"`
	Stop             any            `json:"stop,omitempty"`
	User             *string        `json:"user,omitempty"`
	Seed             *int           `json:"seed,omitempty"`
	LogProbs         *bool          `json:"logprobs,omitempty"`
	TopLogProbs      *int           `json:"top_logprobs,omitempty"`
	ExtraParams      map[string]any `json:"-"`
	Metadata         map[string]any `json:"metadata,omitempty"`

	// Prompt template resolution fields.
	PromptName      string            `json:"prompt_name,omitempty"`
	PromptVariables map[string]string `json:"prompt_variables,omitempty"`
	PromptVersion   *int              `json:"prompt_version,omitempty"`
}

type Message struct {
	Role       string     `json:"role"`
	Content    any        `json:"content"`
	Name       *string    `json:"name,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID *string    `json:"tool_call_id,omitempty"`
}

type Tool struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

type ToolFunction struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Parameters  any    `json:"parameters,omitempty"`
}

type ToolCall struct {
	ID       string           `json:"id"`
	Type     string           `json:"type"`
	Function ToolCallFunction `json:"function"`
	Index    *int             `json:"index,omitempty"`
}

type ToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type StreamOptions struct {
	IncludeUsage bool `json:"include_usage,omitempty"`
}

// IsStreaming returns true if the request has streaming enabled.
func (r *ChatCompletionRequest) IsStreaming() bool {
	return r.Stream != nil && *r.Stream
}

// knownFields lists all known JSON fields on ChatCompletionRequest.
var knownFields = map[string]bool{
	"model": true, "messages": true, "temperature": true,
	"max_tokens": true, "top_p": true, "frequency_penalty": true,
	"presence_penalty": true, "tools": true, "tool_choice": true,
	"response_format": true, "stream": true, "stream_options": true,
	"n": true, "stop": true, "user": true, "seed": true,
	"logprobs": true, "top_logprobs": true,
	// Also accept max_completion_tokens as a known alias
	"max_completion_tokens": true,
	"metadata":              true,
	// Prompt template fields
	"prompt_name":      true,
	"prompt_variables": true,
	"prompt_version":   true,
}

// UnmarshalJSON implements custom JSON unmarshaling that captures unknown
// fields into ExtraParams for pass-through to upstream providers.
func (r *ChatCompletionRequest) UnmarshalJSON(data []byte) error {
	// Use alias to avoid infinite recursion
	type Alias ChatCompletionRequest
	var alias Alias
	if err := json.Unmarshal(data, &alias); err != nil {
		return err
	}
	*r = ChatCompletionRequest(alias)

	// Extract unknown fields
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil // don't fail on this
	}

	for key := range raw {
		if !knownFields[key] {
			if r.ExtraParams == nil {
				r.ExtraParams = make(map[string]any)
			}
			var val any
			_ = json.Unmarshal(raw[key], &val)
			r.ExtraParams[key] = val
		}
	}

	return nil
}

// ContentPart represents a multimodal content part (text or image).
type ContentPart struct {
	Type     string    `json:"type"`
	Text     string    `json:"text,omitempty"`
	ImageURL *ImageURL `json:"image_url,omitempty"`
}

type ImageURL struct {
	URL    string  `json:"url"`
	Detail *string `json:"detail,omitempty"`
}
