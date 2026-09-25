package model

import "encoding/json"

// ModelResponse represents an OpenAI-compatible chat completion response.
type ModelResponse struct {
	ID                string   `json:"id"`
	Object            string   `json:"object"`
	Created           int64    `json:"created"`
	Model             string   `json:"model"`
	Choices           []Choice `json:"choices"`
	Usage             Usage    `json:"usage"`
	SystemFingerprint *string  `json:"system_fingerprint,omitempty"`
}

type Choice struct {
	Index        int       `json:"index"`
	Message      *Message  `json:"message,omitempty"`
	Delta        *Delta    `json:"delta,omitempty"`
	FinishReason *string   `json:"finish_reason"`
	Logprobs     *Logprobs `json:"logprobs,omitempty"`
}

type Usage struct {
	PromptTokens             int `json:"prompt_tokens"`
	CompletionTokens         int `json:"completion_tokens"`
	TotalTokens              int `json:"total_tokens"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
}

type Logprobs struct {
	Content []LogprobContent `json:"content,omitempty"`
}

type LogprobContent struct {
	Token   string  `json:"token"`
	Logprob float64 `json:"logprob"`
	Bytes   []int   `json:"bytes,omitempty"`
}

// StreamChunk represents a streaming chunk in SSE format.
type StreamChunk struct {
	ID                string         `json:"id"`
	Object            string         `json:"object"`
	Created           int64          `json:"created"`
	Model             string         `json:"model"`
	Choices           []StreamChoice `json:"choices"`
	Usage             *Usage         `json:"usage,omitempty"`
	SystemFingerprint *string        `json:"system_fingerprint,omitempty"`
}

type StreamChoice struct {
	Index        int     `json:"index"`
	Delta        Delta   `json:"delta"`
	FinishReason *string `json:"finish_reason"`
}

type Delta struct {
	Role         *string       `json:"role,omitempty"`
	Content      *string       `json:"content,omitempty"`
	ContentParts []ContentPart `json:"content_parts,omitempty"`
	ToolCalls    []ToolCall    `json:"tool_calls,omitempty"`
	Refusal      *string       `json:"refusal,omitempty"`
}

// EmbeddingRequest represents an OpenAI-compatible embedding request.
// ExtraParams captures unknown fields for pass-through to upstream providers
// (e.g. Jina-specific "task" and "normalized" fields).
type EmbeddingRequest struct {
	Model          string         `json:"model"`
	Input          any            `json:"input"`
	EncodingFormat string         `json:"encoding_format,omitempty"`
	Dimensions     *int           `json:"dimensions,omitempty"`
	User           string         `json:"user,omitempty"`
	ExtraParams    map[string]any `json:"-"` // captured unknown fields, not serialized directly
}

// knownEmbeddingFields lists all known JSON fields on EmbeddingRequest.
var knownEmbeddingFields = map[string]bool{
	"model": true, "input": true, "encoding_format": true,
	"dimensions": true, "user": true,
}

// UnmarshalJSON implements custom JSON unmarshaling that captures unknown
// fields into ExtraParams for pass-through to upstream providers.
func (r *EmbeddingRequest) UnmarshalJSON(data []byte) error {
	type Alias EmbeddingRequest
	var alias Alias
	if err := json.Unmarshal(data, &alias); err != nil {
		return err
	}
	*r = EmbeddingRequest(alias)

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	for key := range raw {
		if !knownEmbeddingFields[key] {
			if r.ExtraParams == nil {
				r.ExtraParams = make(map[string]any)
			}
			var val any
			if err := json.Unmarshal(raw[key], &val); err != nil {
				continue // skip malformed extra field
			}
			if val == nil {
				continue // skip JSON null values
			}
			r.ExtraParams[key] = val
		}
	}

	return nil
}

// EmbeddingResponse represents an OpenAI-compatible embedding response.
type EmbeddingResponse struct {
	Object string          `json:"object"`
	Data   []EmbeddingData `json:"data"`
	Model  string          `json:"model"`
	Usage  EmbeddingUsage  `json:"usage"`
}

type EmbeddingData struct {
	Object    string    `json:"object"`
	Index     int       `json:"index"`
	Embedding []float64 `json:"embedding"`
}

type EmbeddingUsage struct {
	PromptTokens int `json:"prompt_tokens"`
	TotalTokens  int `json:"total_tokens"`
}
