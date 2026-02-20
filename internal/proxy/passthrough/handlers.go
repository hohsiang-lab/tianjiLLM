package passthrough

import "encoding/json"

// BaseLoggingHandler is a no-op logging handler for providers without
// specific usage parsing.
type BaseLoggingHandler struct {
	Name string
}

func (h *BaseLoggingHandler) ParseUsage(_ []byte) (int, int) { return 0, 0 }
func (h *BaseLoggingHandler) ProviderName() string           { return h.Name }

// OpenAILoggingHandler extracts usage from OpenAI-format responses.
type OpenAILoggingHandler struct{}

func (h *OpenAILoggingHandler) ProviderName() string { return "openai" }
func (h *OpenAILoggingHandler) ParseUsage(body []byte) (int, int) {
	var resp struct {
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
	}
	if json.Unmarshal(body, &resp) == nil {
		return resp.Usage.PromptTokens, resp.Usage.CompletionTokens
	}
	return 0, 0
}

// AnthropicLoggingHandler extracts usage from Anthropic-format responses.
type AnthropicLoggingHandler struct{}

func (h *AnthropicLoggingHandler) ProviderName() string { return "anthropic" }
func (h *AnthropicLoggingHandler) ParseUsage(body []byte) (int, int) {
	var resp struct {
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}
	if json.Unmarshal(body, &resp) == nil {
		return resp.Usage.InputTokens, resp.Usage.OutputTokens
	}
	return 0, 0
}

// VertexAILoggingHandler extracts usage from Vertex AI responses.
// Vertex AI uses the same format as Gemini.
type VertexAILoggingHandler struct{}

func (h *VertexAILoggingHandler) ProviderName() string { return "vertex_ai" }
func (h *VertexAILoggingHandler) ParseUsage(body []byte) (int, int) {
	var resp struct {
		UsageMetadata struct {
			PromptTokenCount     int `json:"promptTokenCount"`
			CandidatesTokenCount int `json:"candidatesTokenCount"`
		} `json:"usageMetadata"`
	}
	if json.Unmarshal(body, &resp) == nil {
		return resp.UsageMetadata.PromptTokenCount, resp.UsageMetadata.CandidatesTokenCount
	}
	return 0, 0
}

// CohereLoggingHandler extracts usage from Cohere v2 responses.
type CohereLoggingHandler struct{}

func (h *CohereLoggingHandler) ProviderName() string { return "cohere" }
func (h *CohereLoggingHandler) ParseUsage(body []byte) (int, int) {
	// Cohere v2 uses OpenAI-compatible format
	var resp struct {
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
	}
	if json.Unmarshal(body, &resp) == nil {
		return resp.Usage.PromptTokens, resp.Usage.CompletionTokens
	}
	return 0, 0
}

// GeminiLoggingHandler extracts usage from Gemini API responses.
type GeminiLoggingHandler struct{}

func (h *GeminiLoggingHandler) ProviderName() string { return "gemini" }
func (h *GeminiLoggingHandler) ParseUsage(body []byte) (int, int) {
	var resp struct {
		UsageMetadata struct {
			PromptTokenCount     int `json:"promptTokenCount"`
			CandidatesTokenCount int `json:"candidatesTokenCount"`
		} `json:"usageMetadata"`
	}
	if json.Unmarshal(body, &resp) == nil {
		return resp.UsageMetadata.PromptTokenCount, resp.UsageMetadata.CandidatesTokenCount
	}
	return 0, 0
}
