package passthrough

import (
	"bytes"
	"encoding/json"
)

// LoggingHandler extracts usage/metadata from provider-specific responses.
type LoggingHandler interface {
	// ParseUsage extracts token usage from a non-streaming response body.
	// Returns prompt (total input including cache), completion, cacheRead,
	// cacheCreation, and model name.
	ParseUsage(body []byte) (prompt, completion, cacheRead, cacheCreation int, modelName string)
	// ParseSSEUsage extracts token usage from collected SSE bytes.
	// Same return signature as ParseUsage.
	ParseSSEUsage(raw []byte) (prompt, completion, cacheRead, cacheCreation int, modelName string)
	// ProviderName returns the name of this provider.
	ProviderName() string
}

// BaseLoggingHandler is a no-op logging handler for providers without
// specific usage parsing.
type BaseLoggingHandler struct {
	Name string
}

func (h *BaseLoggingHandler) ParseUsage(_ []byte) (int, int, int, int, string) { return 0, 0, 0, 0, "" }
func (h *BaseLoggingHandler) ParseSSEUsage(_ []byte) (int, int, int, int, string) {
	return 0, 0, 0, 0, ""
}
func (h *BaseLoggingHandler) ProviderName() string { return h.Name }

// OpenAILoggingHandler extracts usage from OpenAI-format responses.
type OpenAILoggingHandler struct{}

func (h *OpenAILoggingHandler) ProviderName() string { return "openai" }
func (h *OpenAILoggingHandler) ParseUsage(body []byte) (int, int, int, int, string) {
	var resp struct {
		Model string `json:"model"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
	}
	if json.Unmarshal(body, &resp) == nil {
		return resp.Usage.PromptTokens, resp.Usage.CompletionTokens, 0, 0, resp.Model
	}
	return 0, 0, 0, 0, ""
}
func (h *OpenAILoggingHandler) ParseSSEUsage(raw []byte) (int, int, int, int, string) {
	var prompt, completion int
	var modelName string
	for _, line := range bytes.Split(raw, []byte("\n")) {
		line = bytes.TrimSpace(line)
		if !bytes.HasPrefix(line, []byte("data: ")) {
			continue
		}
		var event struct {
			Model string `json:"model"`
			Usage struct {
				PromptTokens     int `json:"prompt_tokens"`
				CompletionTokens int `json:"completion_tokens"`
			} `json:"usage"`
		}
		if json.Unmarshal(line[6:], &event) != nil {
			continue
		}
		if event.Model != "" {
			modelName = event.Model
		}
		if event.Usage.PromptTokens > 0 || event.Usage.CompletionTokens > 0 {
			prompt = event.Usage.PromptTokens
			completion = event.Usage.CompletionTokens
		}
	}
	return prompt, completion, 0, 0, modelName
}

// AnthropicLoggingHandler extracts usage from Anthropic-format responses.
type AnthropicLoggingHandler struct{}

func (h *AnthropicLoggingHandler) ProviderName() string { return "anthropic" }
func (h *AnthropicLoggingHandler) ParseUsage(body []byte) (int, int, int, int, string) {
	var resp struct {
		Model string `json:"model"`
		Usage struct {
			InputTokens              int `json:"input_tokens"`
			OutputTokens             int `json:"output_tokens"`
			CacheReadInputTokens     int `json:"cache_read_input_tokens"`
			CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
		} `json:"usage"`
	}
	if json.Unmarshal(body, &resp) == nil {
		cr := resp.Usage.CacheReadInputTokens
		cc := resp.Usage.CacheCreationInputTokens
		// prompt = total input including cache tokens (mirrors native_format.go parseUsage)
		return resp.Usage.InputTokens + cr + cc, resp.Usage.OutputTokens, cr, cc, resp.Model
	}
	return 0, 0, 0, 0, ""
}
func (h *AnthropicLoggingHandler) ParseSSEUsage(raw []byte) (int, int, int, int, string) {
	var prompt, completion, cacheRead, cacheCreation int
	var modelName string
	for _, line := range bytes.Split(raw, []byte("\n")) {
		line = bytes.TrimSpace(line)
		if !bytes.HasPrefix(line, []byte("data: ")) {
			continue
		}
		var event struct {
			Type    string `json:"type"`
			Message struct {
				Model string `json:"model"`
				Usage struct {
					InputTokens              int `json:"input_tokens"`
					CacheReadInputTokens     int `json:"cache_read_input_tokens"`
					CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
				} `json:"usage"`
			} `json:"message"`
			Usage struct {
				OutputTokens int `json:"output_tokens"`
			} `json:"usage"`
		}
		if json.Unmarshal(line[6:], &event) != nil {
			continue
		}
		if event.Type == "message_start" {
			if event.Message.Model != "" {
				modelName = event.Message.Model
			}
			cacheRead = event.Message.Usage.CacheReadInputTokens
			cacheCreation = event.Message.Usage.CacheCreationInputTokens
			prompt = event.Message.Usage.InputTokens + cacheRead + cacheCreation
		}
		if event.Type == "message_delta" && event.Usage.OutputTokens > 0 {
			completion = event.Usage.OutputTokens
		}
	}
	return prompt, completion, cacheRead, cacheCreation, modelName
}

// VertexAILoggingHandler extracts usage from Vertex AI responses.
// Vertex AI uses the same format as Gemini.
type VertexAILoggingHandler struct{}

func (h *VertexAILoggingHandler) ProviderName() string { return "vertex_ai" }
func (h *VertexAILoggingHandler) ParseUsage(body []byte) (int, int, int, int, string) {
	var resp struct {
		UsageMetadata struct {
			PromptTokenCount     int `json:"promptTokenCount"`
			CandidatesTokenCount int `json:"candidatesTokenCount"`
		} `json:"usageMetadata"`
	}
	if json.Unmarshal(body, &resp) == nil {
		return resp.UsageMetadata.PromptTokenCount, resp.UsageMetadata.CandidatesTokenCount, 0, 0, ""
	}
	return 0, 0, 0, 0, ""
}
func (h *VertexAILoggingHandler) ParseSSEUsage(raw []byte) (int, int, int, int, string) {
	var prompt, completion int
	for _, line := range bytes.Split(raw, []byte("\n")) {
		line = bytes.TrimSpace(line)
		if !bytes.HasPrefix(line, []byte("data: ")) {
			continue
		}
		var event struct {
			UsageMetadata struct {
				PromptTokenCount     int `json:"promptTokenCount"`
				CandidatesTokenCount int `json:"candidatesTokenCount"`
			} `json:"usageMetadata"`
		}
		if json.Unmarshal(line[6:], &event) != nil {
			continue
		}
		if event.UsageMetadata.PromptTokenCount > 0 || event.UsageMetadata.CandidatesTokenCount > 0 {
			prompt = event.UsageMetadata.PromptTokenCount
			completion = event.UsageMetadata.CandidatesTokenCount
		}
	}
	return prompt, completion, 0, 0, ""
}

// CohereLoggingHandler extracts usage from Cohere v2 responses.
type CohereLoggingHandler struct{}

func (h *CohereLoggingHandler) ProviderName() string { return "cohere" }
func (h *CohereLoggingHandler) ParseUsage(body []byte) (int, int, int, int, string) {
	var resp struct {
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
	}
	if json.Unmarshal(body, &resp) == nil {
		return resp.Usage.PromptTokens, resp.Usage.CompletionTokens, 0, 0, ""
	}
	return 0, 0, 0, 0, ""
}
func (h *CohereLoggingHandler) ParseSSEUsage(_ []byte) (int, int, int, int, string) {
	return 0, 0, 0, 0, ""
}

// GeminiLoggingHandler extracts usage from Gemini API responses.
type GeminiLoggingHandler struct{}

func (h *GeminiLoggingHandler) ProviderName() string { return "gemini" }
func (h *GeminiLoggingHandler) ParseUsage(body []byte) (int, int, int, int, string) {
	var resp struct {
		ModelVersion  string `json:"modelVersion"`
		UsageMetadata struct {
			PromptTokenCount     int `json:"promptTokenCount"`
			CandidatesTokenCount int `json:"candidatesTokenCount"`
		} `json:"usageMetadata"`
	}
	if json.Unmarshal(body, &resp) == nil {
		return resp.UsageMetadata.PromptTokenCount, resp.UsageMetadata.CandidatesTokenCount, 0, 0, resp.ModelVersion
	}
	return 0, 0, 0, 0, ""
}
func (h *GeminiLoggingHandler) ParseSSEUsage(raw []byte) (int, int, int, int, string) {
	var prompt, completion int
	var modelName string
	for _, line := range bytes.Split(raw, []byte("\n")) {
		line = bytes.TrimSpace(line)
		if !bytes.HasPrefix(line, []byte("data: ")) {
			continue
		}
		var event struct {
			ModelVersion  string `json:"modelVersion"`
			UsageMetadata struct {
				PromptTokenCount     int `json:"promptTokenCount"`
				CandidatesTokenCount int `json:"candidatesTokenCount"`
			} `json:"usageMetadata"`
		}
		if json.Unmarshal(line[6:], &event) != nil {
			continue
		}
		if event.ModelVersion != "" {
			modelName = event.ModelVersion
		}
		if event.UsageMetadata.PromptTokenCount > 0 || event.UsageMetadata.CandidatesTokenCount > 0 {
			prompt = event.UsageMetadata.PromptTokenCount
			completion = event.UsageMetadata.CandidatesTokenCount
		}
	}
	return prompt, completion, 0, 0, modelName
}
